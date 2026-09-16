package repository

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func compressionTestRequest(t *testing.T, payload []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), "POST", "https://chatgpt.com/backend-api/codex/responses", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Chatgpt-Account-Id", "test-account")
	return req
}

func TestHTTPUpstreamRequestCompressionPreservesExactBytesAndReplay(t *testing.T) {
	svc := NewHTTPUpstream(&config.Config{Gateway: config.GatewayConfig{OpenAIRequestGzipEnabled: true}}).(*httpUpstreamService)
	payload := []byte(`{"model":"gpt-6-astra","reasoning":{"effort":"high"},"input":"` + strings.Repeat("Original image/context bytes. ", 20000) + `","stream":true}`)
	req := compressionTestRequest(t, payload)
	out := svc.compressOpenAIRequest(req, service.HTTPUpstreamProfileOpenAI, 1)
	require.NotSame(t, req, out)
	require.Empty(t, req.Header.Get("Content-Encoding"), "do not mutate the caller headers")
	require.Equal(t, "gzip", out.Header.Get("Content-Encoding"))
	require.Less(t, out.ContentLength, int64(len(payload)/2))
	for i := 0; i < 2; i++ {
		body := out.Body
		if i == 1 {
			var err error
			body, err = out.GetBody()
			require.NoError(t, err)
		}
		wire, err := io.ReadAll(body)
		require.NoError(t, err)
		require.NoError(t, body.Close())
		require.EqualValues(t, len(wire), out.ContentLength)
		decoder, err := gzip.NewReader(bytes.NewReader(wire))
		require.NoError(t, err)
		decoded, err := io.ReadAll(decoder)
		require.NoError(t, err)
		require.NoError(t, decoder.Close())
		require.Equal(t, payload, decoded, "compression must preserve every byte, including model, reasoning and context")
	}
}

func TestHTTPUpstreamRequestCompressionSkipsUnsupportedAndIncompressible(t *testing.T) {
	svc := NewHTTPUpstream(&config.Config{Gateway: config.GatewayConfig{OpenAIRequestGzipEnabled: true}}).(*httpUpstreamService)
	payload := bytes.Repeat([]byte("original "), 40000)
	for name, modify := range map[string]func(*http.Request){
		"other_host":      func(r *http.Request) { r.URL.Host = "api.openai.com" },
		"other_path":      func(r *http.Request) { r.URL.Path = "/v1/responses" },
		"small":           func(r *http.Request) { r.ContentLength = 100 },
		"too_large":       func(r *http.Request) { r.ContentLength = openAIRequestCompressionMaxBytes + 1 },
		"already_encoded": func(r *http.Request) { r.Header.Set("Content-Encoding", "zstd") },
		"signed":          func(r *http.Request) { r.Header.Set("Content-Digest", "example") },
		"not_replayable":  func(r *http.Request) { r.GetBody = nil },
		"length_mismatch": func(r *http.Request) { r.ContentLength++ },
		"reader_error":    func(r *http.Request) { r.GetBody = func() (io.ReadCloser, error) { return nil, io.ErrUnexpectedEOF } },
	} {
		t.Run(name, func(t *testing.T) {
			req := compressionTestRequest(t, payload)
			modify(req)
			require.Same(t, req, svc.compressOpenAIRequest(req, service.HTTPUpstreamProfileOpenAI, 1))
			got, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, payload, got)
		})
	}
	noise := make([]byte, len(payload))
	_, err := rand.Read(noise)
	require.NoError(t, err)
	req := compressionTestRequest(t, noise)
	require.Same(t, req, svc.compressOpenAIRequest(req, service.HTTPUpstreamProfileOpenAI, 1))
	got, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, noise, got)
	req = compressionTestRequest(t, payload)
	require.Same(t, req, svc.compressOpenAIRequest(req, service.HTTPUpstreamProfileDefault, 1))
	svc.cfg.Gateway.OpenAIRequestGzipEnabled = false
	require.Same(t, req, svc.compressOpenAIRequest(req, service.HTTPUpstreamProfileOpenAI, 1))
}

func TestHTTPUpstreamRequestCompressionHandlesOpaqueBase64AndBusyEncoders(t *testing.T) {
	svc := NewHTTPUpstream(&config.Config{Gateway: config.GatewayConfig{OpenAIRequestGzipEnabled: true}}).(*httpUpstreamService)
	noise := make([]byte, 256<<10)
	_, err := rand.Read(noise)
	require.NoError(t, err)
	payload := []byte(`{"input":"data:image/png;base64,` + base64.StdEncoding.EncodeToString(noise) + `"}`)
	req := compressionTestRequest(t, payload)
	out := svc.compressOpenAIRequest(req, service.HTTPUpstreamProfileOpenAI, 1)
	require.NotSame(t, req, out, "base64 must compress even when the image itself is opaque")
	require.Less(t, out.ContentLength, int64(len(payload)*8/10))
	decoder, err := gzip.NewReader(out.Body)
	require.NoError(t, err)
	decoded, err := io.ReadAll(decoder)
	require.NoError(t, err)
	require.Equal(t, payload, decoded)
	require.NoError(t, decoder.Close())
	require.NoError(t, out.Body.Close())
	for i := 0; i < cap(openAIRequestCompressionSlots); i++ {
		openAIRequestCompressionSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(openAIRequestCompressionSlots); i++ {
			<-openAIRequestCompressionSlots
		}
	}()
	req = compressionTestRequest(t, payload)
	require.Same(t, req, svc.compressOpenAIRequest(req, service.HTTPUpstreamProfileOpenAI, 1), "compression must not add a queue")
	got, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}
