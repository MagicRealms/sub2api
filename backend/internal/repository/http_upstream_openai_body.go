package repository

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// observeOpenAIHTTP2Body feeds the existing circuit with the terminal transport
// result. It never buffers, alters, or retries model output. A fallback changes
// only subsequent requests; the caller receives the original bytes and error.
func (s *httpUpstreamService) observeOpenAIHTTP2Body(ctx context.Context, body io.ReadCloser, profile service.HTTPUpstreamProfile, mode, proxyKey string) io.ReadCloser {
	if body == nil || profile != service.HTTPUpstreamProfileOpenAI || mode != upstreamProtocolModeOpenAIH2 || !isOpenAIHTTP2FallbackProxy(proxyKey) {
		return body
	}
	return &openAIHTTP2ObservedBody{
		ReadCloser: body,
		onResult: func(err error) {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return
			}
			// Count failures across the configured time window, including when
			// healthy concurrent streams finish between two protocol errors.
			s.recordOpenAIHTTP2Failure(profile, mode, proxyKey, err)
		},
	}
}

type openAIHTTP2ObservedBody struct {
	io.ReadCloser
	once     sync.Once
	onResult func(error)
}

func (b *openAIHTTP2ObservedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.once.Do(func() { b.onResult(err) })
	}
	return n, err
}
