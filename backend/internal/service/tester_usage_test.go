//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func testerSQLService(t *testing.T) (*UsageService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()); _ = client.Close() })
	return NewUsageService(nil, nil, client, nil), mock
}

func TestTesterResetOnlyAdvancesOwnWindow(t *testing.T) {
	svc, mock := testerSQLService(t)
	now := time.Now().UTC()
	mock.ExpectQuery(`INSERT INTO tester_usage_history \(user_id, visible_after\).*`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"visible_after"}).AddRow(now)).RowsWillBeClosed()
	since, err := svc.ResetTesterHistory(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, now, *since)
	// No billing/usage/counter UPDATE or DELETE is expected by this driver.
}

func TestTesterResetDeniedAfterLiveRoleChange(t *testing.T) {
	svc, mock := testerSQLService(t)
	mock.ExpectQuery(`(?s)INSERT INTO tester_usage_history.*WHERE id = \$1 AND role = 'tester' AND status = 'active'.*GREATEST`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"visible_after"}))
	_, err := svc.ResetTesterHistory(context.Background(), 7)
	require.Equal(t, http.StatusForbidden, infraerrors.Code(err))
}

func TestTesterMonitoringBoundsAndIsolation(t *testing.T) {
	svc, mock := testerSQLService(t)
	start := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	since, end := start.Add(time.Hour), start.Add(2*time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT visible_after FROM tester_usage_history WHERE user_id = $1")).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"visible_after"}).AddRow(since))
	payload, err := json.Marshal([]TesterMetric{{IsTotal: true, Requests: 2, ActualCost: .96}, {Model: "model-a", GroupID: 6, Requests: 2, ActualCost: .96}})
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(testerMetricsSQL)).WithArgs(int64(7), since, end, int64(70), int64(6), "model-a").WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow(payload)).RowsWillBeClosed()
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) FROM ops_error_logs e.*e.user_id = \$1.*e.status_code >= 400`).WithArgs(int64(7), since, end, int64(70), int64(6), "model-a").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	result, err := svc.TesterMonitoring(context.Background(), 7, start, end, 70, 6, "model-a")
	require.NoError(t, err)
	require.Equal(t, since, result.StartTime)
	require.Equal(t, int64(2), result.Summary.Requests)
	require.Equal(t, .96, result.Summary.ActualCost)
	require.Nil(t, result.Summary.FirstTokenP50, "unknown timing must not look like a zero-latency result")
	require.Len(t, result.Models, 1)
	require.Equal(t, int64(1), result.ErrorRecords)
	require.Contains(t, testerMetricsSQL, "ul.user_id = $1")
	require.Contains(t, testerMetricsSQL, "ul.created_at >= $2 AND ul.created_at < $3")
	require.NotContains(t, testerMetricsSQL, "credentials")
	require.NotContains(t, testerMetricsSQL, "account_rate_multiplier")
}

func TestTesterClearedPastRangeIsEmpty(t *testing.T) {
	svc, mock := testerSQLService(t)
	start := time.Now().Add(-2 * time.Hour)
	mock.ExpectQuery("SELECT visible_after FROM tester_usage_history").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"visible_after"}).AddRow(start.Add(3 * time.Hour)))
	result, err := svc.TesterMonitoring(context.Background(), 7, start, start.Add(time.Hour), 0, 0, "")
	require.NoError(t, err)
	require.Zero(t, result.Summary.Requests)
	require.Empty(t, result.Models)
}

func TestTesterInvalidRangesNeverQuery(t *testing.T) {
	start := time.Now()
	for _, tc := range []struct {
		uid, key, group int64
		start, end      time.Time
	}{
		{0, 0, 0, start, start.Add(time.Hour)}, {7, -1, 0, start, start.Add(time.Hour)},
		{7, 0, -1, start, start.Add(time.Hour)}, {7, 0, 0, start, start},
		{7, 0, 0, start, start.Add(32 * 24 * time.Hour)}, {7, 0, 0, time.Time{}, start},
	} {
		var svc *UsageService
		_, err := svc.TesterMonitoring(context.Background(), tc.uid, tc.start, tc.end, tc.key, tc.group, "")
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	}
}
