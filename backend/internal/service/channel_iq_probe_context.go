package service

import (
	"context"
	"time"
)

const channelIQTestTimeout = 30 * time.Minute

func newChannelIQProbeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), channelIQTestTimeout)
}
