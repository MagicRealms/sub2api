//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOrderedKeyRouteStopsRetryBeforeQueuedBillExecutes(t *testing.T) {
	group := &service.Group{ID: 1}
	parent := service.WithAPIKeyGroupRoute(context.Background(), 9, group, group)
	executed := false
	task, release := wrapUsageRecordTaskContext(parent, func(context.Context) { executed = true })
	defer release()
	require.True(t, service.APIKeyGroupRouteBilled(parent), "the HTTP router must see queued usage before a worker runs")
	require.False(t, executed)
	task(context.Background())
	require.True(t, executed)
}

func TestOrderedKeyRouteNilBillingTaskDoesNotStopRetry(t *testing.T) {
	group := &service.Group{ID: 1}
	parent := service.WithAPIKeyGroupRoute(context.Background(), 9, group, group)
	task, release := wrapUsageRecordTaskContext(parent, nil)
	defer release()
	require.Nil(t, task)
	require.False(t, service.APIKeyGroupRouteBilled(parent))
}
