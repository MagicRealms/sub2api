package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestListDueAutoUpstreamModelSync(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	require.Contains(t, listDueAutoUpstreamModelSyncSQL, "<= $1::bigint -")
	query := regexp.QuoteMeta(listDueAutoUpstreamModelSyncSQL)
	mock.ExpectQuery(query).
		WithArgs(now.UnixMilli()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "interval_hours", "last_attempt_ms"}).
			AddRow(int64(9), "0.5", now.Add(-31*time.Minute).UnixMilli()).
			AddRow(int64(10), "3", now.Add(-time.Hour).UnixMilli()).
			AddRow(int64(11), "0.016666666666666666", now.Add(-time.Minute).UnixMilli()).
			AddRow(int64(12), "0.016666666666666666", now.Add(-30*time.Second).UnixMilli()))

	repo := newAccountRepositoryWithSQL(client, db, nil)
	candidates, err := repo.ListDueAutoUpstreamModelSync(context.Background(), now)

	require.NoError(t, err)
	require.Equal(t, []service.AutoUpstreamModelSyncCandidate{
		{AccountID: 9, IntervalHours: 0.5},
		{AccountID: 11, IntervalHours: service.AutoUpstreamModelSyncOneMinuteHours},
	}, candidates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimAutoUpstreamModelSyncIsConditionalAndIntervalBound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	require.Contains(t, claimAutoUpstreamModelSyncSQL, "<= $2::bigint - $4::bigint")
	mock.ExpectExec(regexp.QuoteMeta(claimAutoUpstreamModelSyncSQL)).
		WithArgs(int64(9), now.UnixMilli(), "0.5", int64(30*time.Minute/time.Millisecond)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(claimAutoUpstreamModelSyncSQL)).
		WithArgs(int64(9), now.UnixMilli(), "0.5", int64(30*time.Minute/time.Millisecond)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(claimAutoUpstreamModelSyncSQL)).
		WithArgs(int64(11), now.UnixMilli(), "0.016666666666666666", int64(time.Minute/time.Millisecond)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := newAccountRepositoryWithSQL(client, db, nil)
	claimed, err := repo.ClaimAutoUpstreamModelSync(context.Background(), 9, 0.5, now)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed, err = repo.ClaimAutoUpstreamModelSync(context.Background(), 9, 0.5, now)
	require.NoError(t, err)
	require.False(t, claimed)
	claimed, err = repo.ClaimAutoUpstreamModelSync(context.Background(), 11, service.AutoUpstreamModelSyncOneMinuteHours, now)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, mock.ExpectationsWereMet())
}
