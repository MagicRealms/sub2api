package repository

import (
	"net/http"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestHTTPUpstreamTracePreservesExistingHooksAndContext(t *testing.T) {
	var previous atomic.Int64
	ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{GotConn: func(httptrace.GotConnInfo) { previous.Add(1) }})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://example.com", nil)
	require.NoError(t, err)
	unchanged, disabled := traceOpenAIUpstream(req, service.HTTPUpstreamProfileDefault)
	require.Same(t, req, unchanged)
	require.Nil(t, disabled)
	traced, observation := traceOpenAIUpstream(req, service.HTTPUpstreamProfileOpenAI)
	require.NotSame(t, req, traced)
	require.Same(t, req.URL, traced.URL)
	hooks := httptrace.ContextClientTrace(traced.Context())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hooks.GotConn(httptrace.GotConnInfo{Reused: true})
			hooks.WroteRequest(httptrace.WroteRequestInfo{})
			hooks.GotFirstResponseByte()
			_ = observation.fields(time.Now())
		}()
	}
	wg.Wait()
	require.EqualValues(t, 8, previous.Load())
	require.NoError(t, req.Context().Err())
	fields := observation.fields(time.Now())
	for _, f := range fields {
		if f.Key == "conn_reused" {
			require.EqualValues(t, 1, f.Integer)
		}
		if f.Key == "request_write_ms" || f.Key == "upstream_wait_ms" {
			require.GreaterOrEqual(t, f.Integer, int64(0))
		}
	}
}
