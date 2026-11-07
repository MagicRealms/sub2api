//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type historyResetRepoStub struct {
	ChannelMonitorRepository
	called bool
	id     int64
}

func (r *historyResetRepoStub) GetByID(_ context.Context, id int64) (*ChannelMonitor, error) {
	if id != 7 {
		return nil, ErrChannelMonitorNotFound
	}
	return &ChannelMonitor{ID: id}, nil
}

func (r *historyResetRepoStub) ResetHistory(_ context.Context, id int64) (*ChannelMonitorHistoryResetResult, error) {
	r.called, r.id = true, id
	return &ChannelMonitorHistoryResetResult{MonitorsReset: 1, HistoryDeleted: 3, RollupsDeleted: 1}, nil
}

func TestChannelMonitorHistoryResetProtectsInFlightChecks(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   int64
		busy map[int64]bool
		deny bool
	}{
		{"selected check running", 7, map[int64]bool{7: true}, true},
		{"any running blocks global reset", 0, map[int64]bool{8: true}, true},
		{"another monitor can run", 7, map[int64]bool{8: true}, false},
		{"all idle", 0, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &historyResetRepoStub{}
			svc := NewChannelMonitorService(repo, nil)
			svc.probing = tc.busy
			result, err := svc.ResetHistory(context.Background(), tc.id)
			if tc.deny {
				require.Equal(t, "MONITOR_CHECK_RUNNING", infraerrors.Reason(err))
				require.False(t, repo.called)
			} else {
				require.NoError(t, err)
				require.True(t, repo.called)
				require.Equal(t, tc.id, repo.id)
				require.Equal(t, int64(3), result.HistoryDeleted)
			}
		})
	}
}

func TestChannelMonitorHistoryResetRejectsUnknownOrInvalidID(t *testing.T) {
	repo := &historyResetRepoStub{}
	svc := NewChannelMonitorService(repo, nil)
	_, err := svc.ResetHistory(context.Background(), 99)
	require.ErrorIs(t, err, ErrChannelMonitorNotFound)
	_, err = svc.ResetHistory(context.Background(), -1)
	require.Error(t, err)
	require.False(t, repo.called)
}
