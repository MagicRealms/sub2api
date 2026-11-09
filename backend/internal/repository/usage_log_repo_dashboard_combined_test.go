//go:build unit

package repository

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUserDashboardCombinedScansPreserveRatesAndTotals(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	mock.ExpectQuery(`SELECT COUNT\(\*\), COUNT\(\*\) FILTER`).
		WithArgs(int64(42), "active").
		WillReturnRows(sqlmock.NewRows([]string{"total", "active"}).AddRow(3, 2))
	columns := []string{"requests", "input", "output", "cache_create", "cache_read", "cost", "actual", "duration",
		"today_requests", "today_input", "today_output", "today_cache_create", "today_cache_read", "today_cost", "today_actual", "recent_requests", "recent_tokens"}
	mock.ExpectQuery(`(?s)COUNT\(\*\).*FILTER.*FROM usage_logs WHERE user_id = \$1`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(10, 100, 20, 10, 30, 3., 2., 150., 4, 40, 8, 4, 12, 1.2, .8, 9, 99))
	mock.ExpectQuery(`(?s)FROM usage_logs ul.*GROUP BY`).
		WithArgs(int64(42), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"platform", "requests", "tokens", "cost", "today_requests", "today_tokens", "today_cost"}))
	stats, err := repo.GetUserDashboardStats(t.Context(), 42)
	require.NoError(t, err)
	require.EqualValues(t, 3, stats.TotalAPIKeys)
	require.EqualValues(t, 2, stats.ActiveAPIKeys)
	require.EqualValues(t, 160, stats.TotalTokens)
	require.EqualValues(t, 64, stats.TodayTokens)
	require.EqualValues(t, 1, stats.Rpm)
	require.EqualValues(t, 19, stats.Tpm)
	require.Equal(t, 2., stats.TotalActualCost)
	require.Equal(t, .8, stats.TodayActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageTrendTimezoneUsesBeijingForExplicitOffsets(t *testing.T) {
	for _, tc := range []struct{ value, zone string }{
		{"2026-11-08T16:30:00+08:00", "UTC-08:00:00"},
		{"2026-11-08T16:30:00-05:30", "UTC+05:30:00"},
		{"2026-11-08T16:30:00Z", "UTC"},
	} {
		at, err := time.Parse(time.RFC3339, tc.value)
		require.NoError(t, err)
		// Parse may reuse time.Local when its current offset matches. Explicit
		// offset inputs must exercise an unnamed zone independently of other
		// tests that initialize the process timezone to Asia/Shanghai.
		if _, offset := at.Zone(); offset != 0 {
			at = at.In(time.FixedZone("", offset))
		}
		require.Equal(t, "Asia/Shanghai", usageTrendTimezone(at))
	}
}

func TestUsageTrendTimezoneUsesBeijingForNamedZones(t *testing.T) {
	for _, name := range []string{"Asia/Shanghai", "America/Los_Angeles"} {
		loc, err := time.LoadLocation(name)
		require.NoError(t, err)
		for _, month := range []time.Month{time.January, time.July} {
			at := time.Date(2026, month, 8, 12, 0, 0, 0, loc)
			require.Equal(t, "Asia/Shanghai", usageTrendTimezone(at))
		}
	}
}
