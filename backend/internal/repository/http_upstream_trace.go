package repository

import (
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// Observe only transport metadata. Never log URLs, headers, prompts or credentials.
// Trace callbacks can run concurrently, including after Do has returned.
type upstreamHTTPTrace struct {
	mu                                 sync.Mutex
	started                            time.Time
	connReady, wroteRequest, firstByte time.Time
	tlsStart                           time.Time
	tlsDuration                        time.Duration
	gotConn, reused, resumed           bool
}

func traceOpenAIUpstream(req *http.Request, profile service.HTTPUpstreamProfile) (*http.Request, *upstreamHTTPTrace) {
	if profile != service.HTTPUpstreamProfileOpenAI {
		return req, nil
	}
	t := &upstreamHTTPTrace{started: time.Now()}
	hooks := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			t.connReady, t.gotConn, t.reused = time.Now(), true, info.Reused
			t.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			t.mu.Lock()
			t.tlsStart = time.Now()
			t.mu.Unlock()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, _ error) {
			t.mu.Lock()
			if !t.tlsStart.IsZero() {
				t.tlsDuration += time.Since(t.tlsStart)
			}
			t.resumed = state.DidResume
			t.mu.Unlock()
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				return
			}
			t.mu.Lock()
			t.wroteRequest = time.Now()
			t.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			t.firstByte = time.Now()
			t.mu.Unlock()
		},
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), hooks)), t
}

func (t *upstreamHTTPTrace) fields(now time.Time) []zap.Field {
	t.mu.Lock()
	defer t.mu.Unlock()
	fields := []zap.Field{
		zap.Int64("headers_ms", now.Sub(t.started).Milliseconds()),
		zap.Bool("conn_observed", t.gotConn),
		zap.Bool("conn_reused", t.reused),
		zap.Bool("tls_resumed", t.resumed),
		zap.Int64("tls_ms", t.tlsDuration.Milliseconds()),
	}
	for _, span := range []struct {
		name       string
		start, end time.Time
	}{
		{"conn_ready_ms", t.started, t.connReady},
		{"request_write_ms", t.connReady, t.wroteRequest},
		{"upstream_wait_ms", t.wroteRequest, t.firstByte},
	} {
		if !span.start.IsZero() && !span.end.IsZero() && !span.end.Before(span.start) {
			fields = append(fields, zap.Int64(span.name, span.end.Sub(span.start).Milliseconds()))
		}
	}
	return fields
}

func (t *upstreamHTTPTrace) log(req *http.Request, resp *http.Response, accountID int64, failed bool) {
	if t == nil {
		return
	}
	fields := append(t.fields(time.Now()),
		zap.String("component", "upstream.http_timing"),
		zap.Int64("account_id", accountID),
		zap.Int64("request_bytes", req.ContentLength),
		zap.Bool("failed", failed),
	)
	if resp != nil {
		fields = append(fields, zap.Int("status_code", resp.StatusCode), zap.String("upstream_protocol", resp.Proto), zap.Bool("auto_decompressed", resp.Uncompressed))
		encoding := resp.Header.Get("Content-Encoding")
		switch encoding {
		case "", "gzip", "br", "deflate", "zstd":
		default:
			encoding = "other"
		}
		fields = append(fields, zap.String("content_encoding", encoding))
	}
	logger.FromContext(req.Context()).Info("upstream HTTP response headers", fields...)
}
