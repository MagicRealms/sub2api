package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsCurrentRatesRespectMonitoringWindow(t *testing.T) {
	end := time.Date(2026, 9, 16, 12, 30, 30, 0, time.UTC)
	for _, tc := range []struct {
		name              string
		start, queryStart time.Time
	}{
		{"first minute after reset", end.Add(-10 * time.Second), end.Add(-10 * time.Second)},
		{"normal minute window", end.Add(-time.Hour), end.Add(-time.Minute)},
		{"old comparison window is empty", end, end},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &opsRepository{db: db}
			filter := &service.OpsDashboardFilter{StartTime: tc.start, EndTime: end}
			mock.ExpectQuery(`(?s)SELECT.*FROM usage_logs ul.*ul.created_at >= \$1.*ul.created_at < \$2`).
				WithArgs(tc.queryStart, end).
				WillReturnRows(sqlmock.NewRows([]string{"success_count", "token_consumed"}).AddRow(0, 0))
			mock.ExpectQuery(`(?s)SELECT.*FROM ops_error_logs.*created_at >= \$1.*created_at < \$2`).
				WithArgs(tc.queryStart, end).
				WillReturnRows(sqlmock.NewRows([]string{"total", "limited", "sla", "upstream", "429", "529"}).AddRow(0, 0, 0, 0, 0, 0))
			qps, tps, err := repo.queryCurrentRates(context.Background(), filter, end)
			require.NoError(t, err)
			require.Zero(t, qps)
			require.Zero(t, tps)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
