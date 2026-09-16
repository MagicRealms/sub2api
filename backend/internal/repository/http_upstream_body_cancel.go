package repository

import (
	"context"
	"io"
)

// Closing a response ends this upstream request, including decoder read-ahead.
// Cancel only the child request, leaving the caller's context usable for usage
// recording and account recovery after a completed response.
type cancelBeforeCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBeforeCloseBody) Close() error {
	b.cancel()
	return b.ReadCloser.Close()
}
