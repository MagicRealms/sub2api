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

func TestReportingStatsUseBeijingRegardlessOfServerTimezone(t *testing.T) {
	useGroupUsageRepositoryTestTimezone(t, "America/Los_Angeles")
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	today := usagestats.DashboardDay(time.Now())
	ctx := context.Background()
	mock.ExpectQuery(`(?s)FROM usage_logs.*WHERE account_id = \$1 AND created_at >= \$2`).WithArgs(int64(7), today).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "tokens", "cost", "standard_cost", "user_cost"}).AddRow(1, 100, 2., 3., 1.))
	account, err := repo.GetAccountTodayStats(ctx, 7)
	require.NoError(t, err)
	require.EqualValues(t, 1, account.Requests)
	start, end := today.AddDate(0, 0, -7), today.AddDate(0, 0, 1)
	mock.ExpectQuery(`(?s)FROM usage_logs.*WHERE api_key_id = ANY`).WithArgs(pq.Array([]int64{7}), start, end, today).
		WillReturnRows(sqlmock.NewRows([]string{"api_key_id", "total_cost", "today_cost"}).AddRow(7, 3., 1.))
	keys, err := repo.GetBatchAPIKeyUsageStats(ctx, []int64{7}, start, end)
	require.NoError(t, err)
	require.Equal(t, 1., keys[7].TodayActualCost)
	mock.ExpectQuery(`(?s)FROM api_keys k LEFT JOIN usage_logs`).WithArgs(pq.Array([]int64{7}), today).
		WillReturnRows(sqlmock.NewRows([]string{"id", "total", "today"}).AddRow(7, 3., 1.))
	displayed, err := repo.GetBatchAPIKeyDisplayUsageStats(ctx, []int64{7})
	require.NoError(t, err)
	require.Equal(t, 1., displayed[7].TodayActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReportingTrendQueriesSpecifyBeijing(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	start, end := time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC), time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)TO_CHAR\(u.created_at AT TIME ZONE \$6`).WithArgs(start, end, 10, start, end, "Asia/Shanghai").
		WillReturnRows(sqlmock.NewRows([]string{"date", "id", "name", "requests", "tokens"}))
	_, err := repo.GetAPIKeyUsageTrend(context.Background(), start, end, "day", 10)
	require.NoError(t, err)
	mock.ExpectQuery(`(?s)TO_CHAR\(created_at AT TIME ZONE \$4`).WithArgs(int64(7), start, end, "Asia/Shanghai").
		WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "input", "output", "create", "read", "total", "cost", "actual"}))
	_, err = repo.GetUserUsageTrendByUserID(context.Background(), 7, start, end, "day")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
