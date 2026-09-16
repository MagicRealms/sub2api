package repository

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newOpenAIHTTP2CircuitTestService() *httpUpstreamService {
	return NewHTTPUpstream(&config.Config{Gateway: config.GatewayConfig{
		OpenAIHTTP2: config.GatewayOpenAIHTTP2Config{
			Enabled: true, AllowProxyFallbackToHTTP1: true,
			FallbackErrorThreshold: 2, FallbackWindowSeconds: 60, FallbackTTLSeconds: 600,
		},
	}}).(*httpUpstreamService)
}

func TestOpenAIHTTP2FallbackCoversSOCKSAndExpires(t *testing.T) {
	for _, scheme := range []string{"http", "https", "socks5", "socks5h"} {
		t.Run(scheme, func(t *testing.T) {
			svc := newOpenAIHTTP2CircuitTestService()
			proxy := scheme + "://user:password@proxy.example:1080"
			initial, err := svc.getClientEntry(proxy, 1, 2, service.HTTPUpstreamProfileOpenAI, false, false)
			require.NoError(t, err)
			// The transport normalizes socks5 to socks5h for remote DNS.
			proxyKey := initial.proxyKey
			for i := 0; i < 2; i++ {
				svc.recordOpenAIHTTP2Failure(service.HTTPUpstreamProfileOpenAI, upstreamProtocolModeOpenAIH2, proxyKey, errors.New("http2: client connection lost"))
			}
			entry, err := svc.getClientEntry(proxy, 1, 2, service.HTTPUpstreamProfileOpenAI, false, false)
			require.NoError(t, err)
			require.Equal(t, upstreamProtocolModeOpenAIH1Fallback, entry.protocolMode)
			transport := entry.client.Transport.(*http.Transport)
			require.False(t, transport.ForceAttemptHTTP2)
			require.NotNil(t, transport.TLSNextProto)
			// A different proxy and a direct route remain independent.
			other, err := svc.getClientEntry("", 2, 2, service.HTTPUpstreamProfileOpenAI, false, false)
			require.NoError(t, err)
			require.Equal(t, upstreamProtocolModeOpenAIH2, other.protocolMode)
			state := svc.getOrCreateOpenAIHTTP2FallbackState(proxyKey)
			state.fallbackUntil = time.Now().Add(-time.Second)
			restored, err := svc.getClientEntry(proxy, 1, 2, service.HTTPUpstreamProfileOpenAI, false, false)
			require.NoError(t, err)
			require.Equal(t, upstreamProtocolModeOpenAIH2, restored.protocolMode)
		})
	}
}

type partialHTTP2FailureBody struct{ closed bool }

func (b *partialHTTP2FailureBody) Read(p []byte) (int, error) {
	return copy(p, "data: first token\n\n"), errors.New("stream error: stream ID 29; PROTOCOL_ERROR; received from peer")
}
func (b *partialHTTP2FailureBody) Close() error { b.closed = true; return nil }

func TestOpenAIHTTP2BodyFailureFallsBackWithoutReplayingOrChangingRequest(t *testing.T) {
	svc := newOpenAIHTTP2CircuitTestService()
	proxy := "socks5://proxy.example:1080"
	entry, err := svc.getClientEntry(proxy, 1336, 20, service.HTTPUpstreamProfileOpenAI, false, false)
	require.NoError(t, err)
	svc.recordOpenAIHTTP2Failure(service.HTTPUpstreamProfileOpenAI, upstreamProtocolModeOpenAIH2, entry.proxyKey, errors.New("http2: protocol error"))
	// A healthy concurrent response must not erase failures inside the window.
	healthy := svc.observeOpenAIHTTP2Body(t.Context(), io.NopCloser(strings.NewReader("completed")), service.HTTPUpstreamProfileOpenAI, upstreamProtocolModeOpenAIH2, entry.proxyKey)
	_, err = io.ReadAll(healthy)
	require.NoError(t, err)
	require.NoError(t, healthy.Close())
	const payload = `{"model":"gpt-6-astra","reasoning":{"effort":"high"},"input":"Keep the entire context.","stream":true}`
	var calls int
	body := &partialHTTP2FailureBody{}
	entry.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		got, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, payload, string(got))
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body, Request: req}, nil
	})
	req, err := http.NewRequestWithContext(service.WithHTTPUpstreamProfile(t.Context(), service.HTTPUpstreamProfileOpenAI), "POST", "https://chatgpt.com/backend-api/codex/responses", strings.NewReader(payload))
	require.NoError(t, err)
	resp, err := svc.Do(req, proxy, 1336, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt64(&entry.inFlight))
	output, readErr := io.ReadAll(resp.Body)
	require.Equal(t, "data: first token\n\n", string(output))
	require.ErrorContains(t, readErr, "PROTOCOL_ERROR")
	require.True(t, svc.isOpenAIHTTP2FallbackActive(entry.proxyKey), "successful headers must not reset an incomplete stream's error window")
	require.Equal(t, 1, calls, "a started response must not be replayed")
	// A repeated terminal read is still just one failure observation.
	_, _ = resp.Body.Read(make([]byte, 64))
	require.Equal(t, 1, calls)
	require.NoError(t, resp.Body.Close())
	require.True(t, body.closed)
	require.Zero(t, atomic.LoadInt64(&entry.inFlight))
}

func TestOpenAIHTTP2FallbackWindowDoesNotAccumulateOldFailures(t *testing.T) {
	state := &openAIHTTP2FallbackState{}
	now := time.Now()
	tripped, _ := state.recordFailure(now, 2, time.Minute, 10*time.Minute)
	require.False(t, tripped)
	tripped, _ = state.recordFailure(now.Add(61*time.Second), 2, time.Minute, 10*time.Minute)
	require.False(t, tripped)
	tripped, until := state.recordFailure(now.Add(62*time.Second), 2, time.Minute, 10*time.Minute)
	require.True(t, tripped)
	require.Equal(t, now.Add(62*time.Second+10*time.Minute), until)
}

func TestOpenAIHTTP2BodyIgnoresClientCancellationAndNormalEOF(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		svc := newOpenAIHTTP2CircuitTestService()
		proxy := "socks5h://proxy.example:1080"
		ctx, cancel := context.WithCancel(t.Context())
		var body io.ReadCloser = io.NopCloser(strings.NewReader("data: completed\n\n"))
		if canceled {
			cancel()
			body = &partialHTTP2FailureBody{}
		}
		observed := svc.observeOpenAIHTTP2Body(ctx, body, service.HTTPUpstreamProfileOpenAI, upstreamProtocolModeOpenAIH2, proxy)
		_, _ = io.ReadAll(observed)
		_, _ = observed.Read(make([]byte, 64))
		require.NoError(t, observed.Close())
		_, recorded := svc.openAIHTTP2Fallbacks.Load(proxy)
		require.False(t, recorded)
		cancel()
	}
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, io.EOF, io.ErrUnexpectedEOF, errors.New("http2: timeout awaiting response headers")} {
		require.False(t, isOpenAIHTTP2CompatibilityError(err))
	}
}

func TestOpenAIHTTP2BodyCountsEachFailedStreamOnlyOnce(t *testing.T) {
	svc := newOpenAIHTTP2CircuitTestService()
	proxy := "socks5://proxy.example:1080"
	body := svc.observeOpenAIHTTP2Body(t.Context(), &partialHTTP2FailureBody{}, service.HTTPUpstreamProfileOpenAI, upstreamProtocolModeOpenAIH2, proxy)
	for i := 0; i < 3; i++ {
		_, err := body.Read(make([]byte, 64))
		require.Error(t, err)
	}
	require.False(t, svc.isOpenAIHTTP2FallbackActive(proxy), "re-reading one terminal error is not three failed requests")
	require.NoError(t, body.Close())
	next := svc.observeOpenAIHTTP2Body(t.Context(), &partialHTTP2FailureBody{}, service.HTTPUpstreamProfileOpenAI, upstreamProtocolModeOpenAIH2, proxy)
	_, err := next.Read(make([]byte, 64))
	require.Error(t, err)
	require.True(t, svc.isOpenAIHTTP2FallbackActive(proxy))
	require.NoError(t, next.Close())
}

type contextBoundTestBody struct {
	io.Reader
	ctx      context.Context
	closeErr error
}

func (b *contextBoundTestBody) Close() error {
	<-b.ctx.Done()
	return b.closeErr
}

func TestHTTPUpstreamCloseCancelsOnlyUpstreamRequest(t *testing.T) {
	svc := newOpenAIHTTP2CircuitTestService()
	entry, err := svc.getClientEntry("", 1, 2, service.HTTPUpstreamProfileOpenAI, false, false)
	require.NoError(t, err)
	parent, cancelParent := context.WithCancel(t.Context())
	defer cancelParent()
	const output = "data: {\"type\":\"response.completed\"}\n\n"
	closeErr := errors.New("underlying close result")
	var upstreamCtx context.Context
	entry.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		upstreamCtx = req.Context()
		return &http.Response{StatusCode: 200, Header: make(http.Header), Request: req,
			Body: &contextBoundTestBody{Reader: strings.NewReader(output), ctx: req.Context(), closeErr: closeErr}}, nil
	})
	req, err := http.NewRequestWithContext(service.WithHTTPUpstreamProfile(parent, service.HTTPUpstreamProfileOpenAI), "POST", "https://chatgpt.com/backend-api/codex/responses", strings.NewReader("{}"))
	require.NoError(t, err)
	resp, err := svc.Do(req, "", 1, 2)
	require.NoError(t, err)
	require.NoError(t, upstreamCtx.Err(), "an active stream must not be canceled")
	got := make([]byte, len(output))
	_, err = io.ReadFull(resp.Body, got)
	require.NoError(t, err)
	require.Equal(t, output, string(got))
	done := make(chan error, 1)
	go func() { done <- resp.Body.Close() }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, closeErr)
	case <-time.After(2 * time.Second):
		cancelParent()
		<-done
		t.Fatal("upstream cancellation waited for response cleanup")
	}
	require.ErrorIs(t, upstreamCtx.Err(), context.Canceled)
	require.NoError(t, parent.Err(), "billing and recovery still need the caller context")
	require.Zero(t, atomic.LoadInt64(&entry.inFlight))
}
