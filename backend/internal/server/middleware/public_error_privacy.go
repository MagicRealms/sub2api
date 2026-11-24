package middleware

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/util/errorprivacy"
	"github.com/gin-gonic/gin"
)

type publicErrorPrivacyKey struct{}
type publicErrorPrivacyState struct{ upstream atomic.Bool }

// PublicErrorPrivacy must precede ordered routing and Ops capture. Those layers
// see the original body for retry, billing and admin diagnostics; only the final
// socket-facing writer projects errors. Re-entered routes share the same writer.
func PublicErrorPrivacy(upstreamOnly ...bool) gin.HandlerFunc {
	return publicResponsePrivacy(false, upstreamOnly...)
}

// PublicIdentity applies the narrow identity projection to user panel responses.
// Admin routes retain their full provider catalog and original diagnostics.
func PublicIdentity() gin.HandlerFunc {
	return publicResponsePrivacy(true)
}

func publicResponsePrivacy(identityOnly bool, upstreamOnly ...bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		withPublicResponsePrivacy(c, identityOnly, upstreamOnly, func() { c.Next() })
	}
}

func withPublicResponsePrivacy(c *gin.Context, identityOnly bool, upstreamOnly []bool, next func()) {
	if identityOnly {
		normalizePublicPlatformFilters(c)
	}
	state, nested := c.Request.Context().Value(publicErrorPrivacyKey{}).(*publicErrorPrivacyState)
	if !nested {
		state = &publicErrorPrivacyState{}
		ctx := context.WithValue(c.Request.Context(), publicErrorPrivacyKey{}, state)
		c.Request = c.Request.WithContext(errorprivacy.WithUpstreamMarker(ctx, &state.upstream))
	}
	if len(upstreamOnly) > 0 && upstreamOnly[0] {
		state.upstream.Store(true)
	}
	observe := func() {
		if c.GetInt("ops_upstream_status_code") > 0 || c.GetString("ops_upstream_error_message") != "" || c.GetInt64("ops_account_id") > 0 {
			state.upstream.Store(true)
		}
	}
	if nested {
		defer observe()
		next()
		return
	}
	original := c.Writer
	writer := &publicErrorWriter{ResponseWriter: original, status: original.Status(), size: -1, state: state, identityOnly: identityOnly}
	c.Writer = writer
	defer func() {
		observe()
		writer.finish()
		c.Writer = original
	}()
	next()
}

// Each writer retains at most 64 KiB of an error body/event. Large known normal
// SSE events stream directly; oversized unknown/error events become a generic
// error and are discarded through the next event delimiter. No goroutines,
// pooled state, timers, or references to gin.Context outlive the request.
type publicErrorWriter struct {
	gin.ResponseWriter
	mu                            sync.Mutex
	state                         *publicErrorPrivacyState
	status, size                  int
	committed, finished, hijacked bool
	pending                       []byte
	overflow                      bool
	lineNonEmpty                  bool
	streamNormal, streamDiscard   bool
	writeErr                      error
	identityOnly                  bool
	normalScrubber                errorprivacy.StreamScrubber
	projector                     errorprivacy.Projector
}

func (w *publicErrorWriter) Status() int                 { w.mu.Lock(); defer w.mu.Unlock(); return w.status }
func (w *publicErrorWriter) Size() int                   { w.mu.Lock(); defer w.mu.Unlock(); return w.size }
func (w *publicErrorWriter) Written() bool               { w.mu.Lock(); defer w.mu.Unlock(); return w.size >= 0 }
func (w *publicErrorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *publicErrorWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size < 0 && !w.committed {
		w.status = status
	}
}

func (w *publicErrorWriter) streaming() bool {
	return strings.HasPrefix(strings.ToLower(w.Header().Get("Content-Type")), "text/event-stream")
}

func (w *publicErrorWriter) commit(errorResponse bool) {
	if w.committed || w.hijacked {
		return
	}
	errorprivacy.Headers(w.Header(), errorResponse)
	if errorResponse {
		w.Header().Del("Content-Length")
		w.Header().Del("Content-Encoding")
		w.Header().Del("ETag")
	}
	w.ResponseWriter.WriteHeader(w.status)
	w.committed = true
}

func (w *publicErrorWriter) WriteHeaderNow() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished || w.hijacked {
		return
	}
	if w.size < 0 {
		w.size = 0
	}
	if w.status < 400 || w.streaming() {
		w.commit(w.streaming())
		w.ResponseWriter.WriteHeaderNow()
	}
}

func (w *publicErrorWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished || w.hijacked {
		return 0, errors.New("public response is closed")
	}
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	inputSize := len(body)
	if w.size < 0 {
		w.size = 0
	}
	w.size += len(body)
	if w.streaming() {
		w.commit(true)
		w.writeSSE(body)
		return len(body), w.writeErr
	}
	if w.status >= 400 {
		if !w.overflow && len(w.pending)+len(body) <= errorprivacy.MaxErrorBytes {
			w.appendPending(body)
		} else {
			w.pending = nil
			w.overflow = true
		}
		return len(body), nil
	}
	// Some providers return JSON errors with HTTP 200. Gin and the gateway's
	// non-stream writers emit those envelopes in one write, so project them too.
	projected, changed := body, false
	contentType := strings.ToLower(w.Header().Get("Content-Type"))
	if strings.Contains(contentType, "json") || contentType == "" {
		if w.identityOnly {
			projected, changed = errorprivacy.ScrubJSON(body)
		} else {
			projected, changed = errorprivacy.PublicJSON(body, w.state.upstream.Load())
		}
	} else if strings.HasPrefix(contentType, "text/") {
		projected = errorprivacy.ScrubBytes(body)
		changed = !bytes.Equal(projected, body)
	}
	w.commit(changed)
	if changed {
		body = projected
	}
	w.rawWrite(body)
	return inputSize, w.writeErr
}

func (w *publicErrorWriter) WriteString(body string) (int, error) { return w.Write([]byte(body)) }

func (w *publicErrorWriter) rawWrite(body []byte) {
	if w.writeErr == nil && len(body) > 0 {
		_, w.writeErr = w.ResponseWriter.Write(body)
	}
}

func (w *publicErrorWriter) writeSSE(body []byte) {
	for len(body) > 0 && w.writeErr == nil {
		n := bytes.IndexByte(body, '\n')
		newline := n >= 0
		if newline {
			n++
		} else {
			n = len(body)
		}
		part := body[:n]
		body = body[n:]
		lineContent := bytes.TrimSuffix(bytes.TrimSuffix(part, []byte("\n")), []byte("\r"))
		if len(lineContent) > 0 {
			w.lineNonEmpty = true
		}
		end := newline && !w.lineNonEmpty
		if w.streamNormal {
			w.rawWrite(w.normalScrubber.Write(part))
		} else if !w.streamDiscard {
			remaining := errorprivacy.MaxErrorBytes - len(w.pending)
			if len(part) <= remaining {
				w.appendPending(part)
			} else {
				w.appendPending(part[:remaining])
				if errorprivacy.NormalSSEPrefix(w.pending) {
					w.streamNormal = true
					w.rawWrite(w.normalScrubber.Write(w.pending))
					w.rawWrite(w.normalScrubber.Write(part[remaining:]))
				} else {
					w.streamDiscard = true
					w.rawWrite(errorprivacy.SSEFallback())
				}
				w.pending = nil
			}
		}
		if end {
			if w.streamNormal {
				w.rawWrite(w.normalScrubber.Finish())
			}
			if len(w.pending) > 0 {
				w.rawWrite(errorprivacy.SSEWithProjector(w.pending, w.state.upstream.Load(), &w.projector))
			}
			w.pending = w.pending[:0]
			w.streamNormal, w.streamDiscard = false, false
		}
		if newline {
			w.lineNonEmpty = false
		}
	}
}

func (w *publicErrorWriter) appendPending(body []byte) {
	if len(w.pending)+len(body) > cap(w.pending) {
		capacity := max(1024, max(cap(w.pending)*2, len(w.pending)+len(body)))
		if capacity > errorprivacy.MaxErrorBytes {
			capacity = errorprivacy.MaxErrorBytes
		}
		next := make([]byte, len(w.pending), capacity)
		copy(next, w.pending)
		w.pending = next
	}
	w.pending = append(w.pending, body...)
}

func (w *publicErrorWriter) flushError() {
	if w.committed {
		return
	}
	body := errorprivacy.Fallback()
	if !w.overflow {
		if w.identityOnly {
			if json.Valid(w.pending) {
				body, _ = errorprivacy.ScrubJSON(w.pending)
			} else {
				body = errorprivacy.ScrubBytes(w.pending)
			}
		} else if projected, changed := errorprivacy.JSON(w.pending, w.state.upstream.Load()); changed {
			body, _ = errorprivacy.ScrubJSON(projected)
		}
	}
	if !w.identityOnly || w.overflow || json.Valid(body) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.commit(true)
	w.rawWrite(body)
	w.pending = nil
}

func (w *publicErrorWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished || w.hijacked {
		return
	}
	if w.size < 0 {
		w.size = 0
	}
	// Error bodies are deliberately held until the handler returns: an early
	// flush must not allow later fragments to escape redaction.
	if w.status >= 400 && !w.streaming() {
		return
	}
	w.commit(w.streaming())
	w.ResponseWriter.Flush()
}

func (w *publicErrorWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	errorprivacy.Headers(w.Header(), false)
	conn, buffer, err := w.ResponseWriter.Hijack()
	if err == nil {
		w.hijacked = true
	}
	return conn, buffer, err
}

func (w *publicErrorWriter) finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished {
		return
	}
	defer func() { w.finished = true; w.pending = nil }()
	if w.hijacked {
		return
	}
	if w.status >= 400 && !w.streaming() {
		w.flushError()
		return
	}
	if w.streaming() && len(w.pending) > 0 {
		w.commit(true)
		w.rawWrite(errorprivacy.SSEWithProjector(w.pending, w.state.upstream.Load(), &w.projector))
	}
	if w.streamNormal {
		w.rawWrite(w.normalScrubber.Finish())
	}
	if !w.committed {
		w.commit(false)
	}
}
