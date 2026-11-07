package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Serialize reset and daily aggregation. Acquire this in a separate statement
// before reading histories so Read Committed takes a fresh snapshot after wait.
const channelMonitorHistoryLock int64 = 743115320807

func lockChannelMonitorHistory(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", channelMonitorHistoryLock)
	return err
}

func (r *channelMonitorRepository) ResetHistory(ctx context.Context, monitorID int64) (*service.ChannelMonitorHistoryResetResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockChannelMonitorHistory(ctx, tx); err != nil {
		return nil, err
	}
	result := &service.ChannelMonitorHistoryResetResult{}
	for _, step := range []struct {
		query string
		count *int64
	}{
		{"DELETE FROM channel_monitor_daily_rollups WHERE ($1::bigint = 0 OR monitor_id = $1)", &result.RollupsDeleted},
		{"DELETE FROM channel_monitor_histories WHERE ($1::bigint = 0 OR monitor_id = $1)", &result.HistoryDeleted},
		{"UPDATE channel_monitors SET last_checked_at = NULL, updated_at = NOW() WHERE ($1::bigint = 0 OR id = $1)", &result.MonitorsReset},
	} {
		res, err := tx.ExecContext(ctx, step.query, monitorID)
		if err != nil {
			return nil, fmt.Errorf("reset channel monitor history: %w", err)
		}
		*step.count, err = res.RowsAffected()
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
