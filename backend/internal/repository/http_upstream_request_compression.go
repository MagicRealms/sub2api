package repository

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

const (
	openAIRequestCompressionMinBytes = 256 << 10
	openAIRequestCompressionMaxBytes = 64 << 20
)

// Compression is optional: under CPU/memory pressure send the original request
// instead of adding a compression queue or allocating unbounded buffers.
var openAIRequestCompressionSlots = make(chan struct{}, 2)

type compressionBuffer struct {
	bytes.Buffer
	limit int
}

func (b *compressionBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}

func (s *httpUpstreamService) compressOpenAIRequest(req *http.Request, profile service.HTTPUpstreamProfile, accountID int64) *http.Request {
	if s.cfg == nil || !s.cfg.Gateway.OpenAIRequestGzipEnabled || profile != service.HTTPUpstreamProfileOpenAI || req == nil || req.URL == nil {
		return req
	}
	// This endpoint was verified with both production OAuth accounts. Do not
	// assume arbitrary OpenAI-compatible providers support compressed uploads.
	if req.Method != http.MethodPost || req.URL.Scheme != "https" || req.URL.Host != "chatgpt.com" || req.URL.Path != "/backend-api/codex/responses" || req.URL.RawQuery != "" ||
		req.Header.Get("Chatgpt-Account-Id") == "" || !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") ||
		req.Header.Get("Content-Encoding") != "" || req.Body == nil || req.GetBody == nil ||
		req.ContentLength < openAIRequestCompressionMinBytes || req.ContentLength > openAIRequestCompressionMaxBytes || req.Context().Err() != nil {
		return req
	}
	for _, h := range []string{"Content-MD5", "Digest", "Content-Digest", "Signature"} {
		if req.Header.Get(h) != "" {
			return req
		}
	}
	select {
	case openAIRequestCompressionSlots <- struct{}{}:
		defer func() { <-openAIRequestCompressionSlots }()
	default:
		return req
	}
	started := time.Now()
	source, err := req.GetBody()
	if err != nil {
		return req
	}
	defer source.Close()
	// Keep the original body untouched until compression has fully succeeded.
	// Require at least 10% savings; a bounded writer aborts incompressible input.
	compressed := &compressionBuffer{limit: int(req.ContentLength * 9 / 10)}
	writer, err := gzip.NewWriterLevel(compressed, gzip.BestSpeed)
	if err != nil {
		return req
	}
	n, copyErr := io.Copy(writer, io.LimitReader(source, req.ContentLength+1))
	closeErr := writer.Close()
	if copyErr != nil || closeErr != nil || n != req.ContentLength {
		return req
	}
	if req.Context().Err() != nil {
		return req
	}
	wire := compressed.Bytes()
	out := req.Clone(req.Context())
	out.Body = io.NopCloser(bytes.NewReader(wire))
	out.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(wire)), nil }
	out.ContentLength = int64(len(wire))
	out.Header.Set("Content-Encoding", "gzip")
	out.Header.Del("Content-Length")
	_ = req.Body.Close()
	logger.FromContext(req.Context()).Info("upstream request compressed",
		zap.String("component", "upstream.request_compression"),
		zap.Int64("account_id", accountID),
		zap.Int64("original_bytes", n), zap.Int("wire_bytes", len(wire)),
		zap.Int64("compression_ms", time.Since(started).Milliseconds()),
	)
	return out
}
