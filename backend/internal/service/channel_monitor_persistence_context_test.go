//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type monitorPersistenceContextRepo struct {
	ChannelMonitorRepository
	insert func(context.Context, []*ChannelMonitorHistoryRow) error
	mark   func(context.Context, int64, time.Time) error
}

func (r *monitorPersistenceContextRepo) InsertHistoryBatch(ctx context.Context, rows []*ChannelMonitorHistoryRow) error {
	return r.insert(ctx, rows)
}

func (r *monitorPersistenceContextRepo) MarkChecked(ctx context.Context, id int64, at time.Time) error {
	return r.mark(ctx, id, at)
}

func TestChannelMonitorPersistenceSurvivesProbeCancellation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "canceled"
		if expired {
			name = "expired"
		}
		t.Run(name, func(t *testing.T) {
			type traceKey struct{}
			parent := context.WithValue(context.Background(), traceKey{}, "probe-trace")
			var cancel context.CancelFunc
			if expired {
				parent, cancel = context.WithDeadline(parent, time.Now().Add(-time.Second))
			} else {
				parent, cancel = context.WithCancel(parent)
				cancel()
			}
			defer cancel()
			checkedAt := time.Now()
			var persisted context.Context
			marked := false
			repo := &monitorPersistenceContextRepo{
				insert: func(ctx context.Context, rows []*ChannelMonitorHistoryRow) error {
					require.NoError(t, ctx.Err())
					require.Equal(t, "probe-trace", ctx.Value(traceKey{}))
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					require.Positive(t, time.Until(deadline))
					require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
					require.Len(t, rows, 1)
					require.Equal(t, int64(23), rows[0].MonitorID)
					require.Equal(t, MonitorStatusError, rows[0].Status)
					require.Equal(t, "probe timed out", rows[0].Message)
					require.Equal(t, checkedAt, rows[0].CheckedAt)
					persisted = ctx
					return nil
				},
				mark: func(ctx context.Context, id int64, _ time.Time) error {
					require.NoError(t, ctx.Err())
					require.Equal(t, int64(23), id)
					marked = true
					return nil
				},
			}
			svc := &ChannelMonitorService{repo: repo}
			svc.persistCheckResults(parent, &ChannelMonitor{ID: 23}, []*CheckResult{{Model: "test-model", Status: MonitorStatusError, Message: "probe timed out", CheckedAt: checkedAt}})
			require.True(t, marked)
			require.ErrorIs(t, persisted.Err(), context.Canceled)
		})
	}
}

func TestChannelMonitorPersistenceParentCanceledDuringWrite(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	marked := false
	repo := &monitorPersistenceContextRepo{
		insert: func(ctx context.Context, _ []*ChannelMonitorHistoryRow) error {
			cancel()
			require.NoError(t, ctx.Err())
			return nil
		},
		mark: func(ctx context.Context, _ int64, _ time.Time) error {
			require.NoError(t, ctx.Err())
			marked = true
			return nil
		},
	}
	(&ChannelMonitorService{repo: repo}).persistCheckResults(parent, &ChannelMonitor{ID: 1}, []*CheckResult{{Model: "test-model"}})
	require.True(t, marked)
}

func TestChannelMonitorPersistenceDoesNotMarkFailedHistoryFresh(t *testing.T) {
	marked := false
	var persisted context.Context
	repo := &monitorPersistenceContextRepo{
		insert: func(ctx context.Context, _ []*ChannelMonitorHistoryRow) error {
			persisted = ctx
			return errors.New("history database unavailable")
		},
		mark: func(context.Context, int64, time.Time) error {
			marked = true
			return nil
		},
	}
	(&ChannelMonitorService{repo: repo}).persistCheckResults(context.Background(), &ChannelMonitor{ID: 1}, []*CheckResult{{Model: "test-model"}})
	require.False(t, marked)
	// The persistence budget must be released on both success and error paths.
	_, ok := persisted.Deadline()
	require.True(t, ok)
	require.ErrorIs(t, persisted.Err(), context.Canceled)
}

func TestChannelMonitorPersistenceSkipsEmptyResults(t *testing.T) {
	repo := &monitorPersistenceContextRepo{
		insert: func(context.Context, []*ChannelMonitorHistoryRow) error {
			t.Fatal("empty history must not be inserted")
			return nil
		},
		mark: func(context.Context, int64, time.Time) error {
			t.Fatal("empty result must not mark monitor fresh")
			return nil
		},
	}
	(&ChannelMonitorService{repo: repo}).persistCheckResults(context.Background(), &ChannelMonitor{ID: 1}, nil)
}
