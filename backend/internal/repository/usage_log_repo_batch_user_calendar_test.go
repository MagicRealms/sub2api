//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestBatchUserUsageTodayUsesBeijingCalendar(t *testing.T) {
	useGroupUsageRepositoryTestTimezone(t, "America/Los_Angeles")
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	now := time.Now()
	start, end := now.AddDate(0, 0, -30), now

	mock.ExpectQuery(`(?s)SELECT.*ul.user_id.*GROUP BY ul.user_id`).
		WithArgs(pq.Array([]int64{7}), start, end, usagestats.DashboardDay(now)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "platform", "total_cost", "today_cost"}).
			AddRow(7, "openai", 12.5, 1.25))

	stats, err := repo.GetBatchUserUsageStats(context.Background(), []int64{7, 7, -1}, start, end)
	require.NoError(t, err)
	require.InDelta(t, 12.5, stats[7].TotalActualCost, 1e-7)
	require.InDelta(t, 1.25, stats[7].TodayActualCost, 1e-7)
	require.NoError(t, mock.ExpectationsWereMet())
}
