package repository

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// A provider can send response.completed while keeping the HTTP body open.
// The asynchronous zstd decoder may already be waiting for another frame.
func TestDecompressResponseBodyCloseUnblocksZstdReadAhead(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	payload := []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
	closeErr := errors.New("transport close result")
	raw := &openEndedCompressedBody{
		Reader:  bytes.NewReader(compressZstd(t, payload)),
		blocked: make(chan struct{}), closed: make(chan struct{}), closeErr: closeErr,
	}
	t.Cleanup(func() { _ = raw.Close() })
	resp := &http.Response{Header: http.Header{"Content-Encoding": {"zstd"}}, Body: raw}
	decompressResponseBody(resp)
	got := make([]byte, len(payload))
	_, err := io.ReadFull(resp.Body, got)
	require.NoError(t, err)
	require.Equal(t, payload, got, "completed SSE bytes must remain unchanged")
	select {
	case <-raw.blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("decoder did not read ahead into the open transport")
	}
	done := make(chan error, 1)
	go func() { done <- resp.Body.Close() }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, closeErr)
	case <-time.After(2 * time.Second):
		_ = raw.Close() // Release the decoder even when the regression is present.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("decoder did not stop after closing its transport")
		}
		t.Fatal("closing a completed stream waited for upstream EOF")
	}
}

type openEndedCompressedBody struct {
	*bytes.Reader
	blocked, closed      chan struct{}
	blockOnce, closeOnce sync.Once
	closeErr             error
}

func (b *openEndedCompressedBody) Read(p []byte) (int, error) {
	if b.Reader.Len() > 0 {
		return b.Reader.Read(p)
	}
	b.blockOnce.Do(func() { close(b.blocked) })
	<-b.closed
	return 0, net.ErrClosed
}

func (b *openEndedCompressedBody) Close() error {
	b.closeOnce.Do(func() { close(b.closed) })
	return b.closeErr
}

func TestDecompressResponseBodyZstdUsage(t *testing.T) {
	payload := []byte(`{"usage":{"input_tokens":123,"output_tokens":45,"cache_read_input_tokens":67}}`)
	compressed := compressZstd(t, payload)
	resp := newEncodedResponse("zstd", compressed)

	decompressResponseBody(resp)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, payload, body)
	require.Equal(t, int64(123), gjson.GetBytes(body, "usage.input_tokens").Int())
	require.Equal(t, int64(45), gjson.GetBytes(body, "usage.output_tokens").Int())
	require.Equal(t, int64(67), gjson.GetBytes(body, "usage.cache_read_input_tokens").Int())
	require.Empty(t, resp.Header.Get("Content-Encoding"))
	require.Empty(t, resp.Header.Get("Content-Length"))
	require.Equal(t, int64(-1), resp.ContentLength)
	require.NoError(t, resp.Body.Close())
}

func TestDecompressResponseBodyExistingEncodings(t *testing.T) {
	payload := []byte(`{"ok":true}`)
	tests := []struct {
		name     string
		encoding string
		compress func(*testing.T, []byte) []byte
	}{
		{name: "gzip", encoding: "gzip", compress: compressGzip},
		{name: "brotli", encoding: "br", compress: compressBrotli},
		{name: "deflate", encoding: "deflate", compress: compressDeflate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := newEncodedResponse(tt.encoding, tt.compress(t, payload))

			decompressResponseBody(resp)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, payload, body)
			require.Empty(t, resp.Header.Get("Content-Encoding"))
			require.Empty(t, resp.Header.Get("Content-Length"))
			require.Equal(t, int64(-1), resp.ContentLength)
			require.NoError(t, resp.Body.Close())
		})
	}
}

func TestDecompressResponseBodyWithoutEncodingLeavesBodyUntouched(t *testing.T) {
	originalBody := &responseTestBody{Reader: bytes.NewReader([]byte("plain"))}
	resp := &http.Response{
		Header:        make(http.Header),
		Body:          originalBody,
		ContentLength: 5,
	}

	decompressResponseBody(resp)

	require.Same(t, originalBody, resp.Body)
	require.Equal(t, int64(5), resp.ContentLength)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "plain", string(body))
	require.NoError(t, resp.Body.Close())
}

func TestDecompressResponseBodyInvalidZstdWarnsAndPreservesBody(t *testing.T) {
	previousLogger := slog.Default()
	var logOutput bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	payload := []byte("not a zstd response")
	resp := newEncodedResponse("zstd", payload)

	require.NotPanics(t, func() {
		decompressResponseBody(resp)
	})

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, payload, body)
	require.Equal(t, "zstd", resp.Header.Get("Content-Encoding"))
	require.Equal(t, int64(len(payload)), resp.ContentLength)
	require.Contains(t, logOutput.String(), "msg=zstd_decompress_failed")
	require.NoError(t, resp.Body.Close())
}

func TestDecompressResponseBodyEmptyZstdWarnsAndPreservesBody(t *testing.T) {
	previousLogger := slog.Default()
	var logOutput bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	resp := newEncodedResponse("zstd", nil)

	require.NotPanics(t, func() {
		decompressResponseBody(resp)
	})

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Empty(t, body)
	require.Equal(t, "zstd", resp.Header.Get("Content-Encoding"))
	require.Equal(t, int64(0), resp.ContentLength)
	require.Contains(t, logOutput.String(), "msg=zstd_decompress_failed")
	require.NoError(t, resp.Body.Close())
}

type responseTestBody struct {
	io.Reader
}

func (b *responseTestBody) Close() error {
	return nil
}

func newEncodedResponse(encoding string, body []byte) *http.Response {
	header := make(http.Header)
	header.Set("Content-Encoding", encoding)
	header.Set("Content-Length", "123")
	return &http.Response{
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func compressZstd(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	_, err = zw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func compressGzip(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func compressBrotli(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := brotli.NewWriter(&buf)
	_, err := zw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func compressDeflate(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	require.NoError(t, err)
	_, err = zw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}
