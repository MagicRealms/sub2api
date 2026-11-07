package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type ChannelMonitorHistoryResetResult struct {
	MonitorsReset  int64 `json:"monitors_reset"`
	HistoryDeleted int64 `json:"history_deleted"`
	RollupsDeleted int64 `json:"rollups_deleted"`
}

// monitorID=0 resets all monitors. Keep the probe gate locked through the short
// transaction: a new check must start after the reset, and an old check must not
// write its result back after history has been cleared.
func (s *ChannelMonitorService) ResetHistory(ctx context.Context, monitorID int64) (*ChannelMonitorHistoryResetResult, error) {
	if monitorID < 0 {
		return nil, infraerrors.BadRequest("INVALID_MONITOR_ID", "invalid monitor ID")
	}
	s.probeMu.Lock()
	defer s.probeMu.Unlock()
	if (monitorID == 0 && len(s.probing) > 0) || s.probing[monitorID] {
		return nil, infraerrors.Conflict("MONITOR_CHECK_RUNNING", "A channel check is running. Wait for it to finish before resetting history.")
	}
	if monitorID > 0 {
		if _, err := s.repo.GetByID(ctx, monitorID); err != nil {
			return nil, err
		}
	}
	return s.repo.ResetHistory(ctx, monitorID)
}
