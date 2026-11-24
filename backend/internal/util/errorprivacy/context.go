package errorprivacy

import (
	"context"
	"sync/atomic"
)

type upstreamMarkerKey struct{}

// WithUpstreamMarker shares only an atomic flag across ordered route attempts;
// transport workers never retain or inspect a gin.Context to classify errors.
func WithUpstreamMarker(ctx context.Context, marker *atomic.Bool) context.Context {
	return context.WithValue(ctx, upstreamMarkerKey{}, marker)
}

func MarkUpstream(ctx context.Context) {
	if ctx == nil {
		return
	}
	if marker, ok := ctx.Value(upstreamMarkerKey{}).(*atomic.Bool); ok && marker != nil {
		marker.Store(true)
	}
}
