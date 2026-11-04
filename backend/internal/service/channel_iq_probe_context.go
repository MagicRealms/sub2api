package service

import (
	"context"
	"time"
)

const channelIQTestTimeout = 30 * time.Minute

func newChannelIQProbeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(WithHTTPUpstreamRedirectsDisabled(context.Background()), channelIQTestTimeout)
}

func newChannelIQProbeWindowContext(deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(WithHTTPUpstreamRedirectsDisabled(context.Background()), deadline)
}
