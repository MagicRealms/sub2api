package repository

import (
	"context"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestChannelMonitorV2RefreshesCoarseRollupsInsideCurrentBucket(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	start := time.Date(2026, 11, 8, 19, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)
	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	for _, seconds := range []int{300, 3600, 43200, 86400} {
		interval := fmt.Sprintf("%d seconds", seconds)
		for _, table := range []string{"channel_monitor_v2_latency_histograms_rollup", "channel_monitor_v2_error_metrics_rollup", "channel_monitor_v2_user_metrics_rollup", "channel_monitor_v2_metrics_rollup"} {
			mock.ExpectExec(fmt.Sprintf(channelMonitorV2FixedRollupDeleteSQL, table)).WithArgs(interval, seconds, start, end).WillReturnResult(sqlmock.NewResult(0, 1))
		}
		for _, query := range []string{channelMonitorV2MetricsRollupSQL, channelMonitorV2UserMetricsRollupSQL, channelMonitorV2HistogramRollupSQL, channelMonitorV2ErrorRollupSQL} {
			mock.ExpectExec(query).WithArgs(interval, seconds, start, end).WillReturnResult(sqlmock.NewResult(0, 1))
		}
	}
	mock.ExpectRollback()
	require.NoError(t, (&channelMonitorV2Repository{db: db}).recomputeFixedRollups(context.Background(), tx, start, end))
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
