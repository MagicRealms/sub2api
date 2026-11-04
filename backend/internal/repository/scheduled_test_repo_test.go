package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelIQClaimPersistsRunningResultAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Date(2026, 11, 4, 12, 0, 10, 0, time.UTC)
	next := now.Add(time.Hour).Truncate(time.Hour)
	run := &service.ScheduledTestResult{ModelID: "gpt-6.1-sol", ReasoningEffort: "high", StartedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE scheduled_test_plans SET next_run_at").
		WithArgs(int64(1), now, next).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO scheduled_test_results").
		WithArgs(int64(1), run.ModelID, run.ReasoningEffort, now).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(51)))
	mock.ExpectExec("DELETE FROM scheduled_test_results").
		WithArgs(int64(1), 6).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &scheduledTestPlanRepository{db: db}
	claimed, err := repo.ClaimDueWithResult(context.Background(), 1, now, next, run, 6)
	require.NoError(t, err)
	require.True(t, claimed)
	require.EqualValues(t, 51, run.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelIQClaimRollsBackWhenResultCannotBeInserted(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Date(2026, 11, 4, 12, 0, 10, 0, time.UTC)
	next := now.Add(time.Hour).Truncate(time.Hour)
	run := &service.ScheduledTestResult{StartedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE scheduled_test_plans SET next_run_at").
		WithArgs(int64(1), now, next).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO scheduled_test_results").
		WithArgs(int64(1), "", "", now).WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	repo := &scheduledTestPlanRepository{db: db}
	claimed, err := repo.ClaimDueWithResult(context.Background(), 1, now, next, run, 6)
	require.Error(t, err)
	require.False(t, claimed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelIQClaimDoesNotDuplicateAlreadyClaimedHour(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Date(2026, 11, 4, 12, 0, 10, 0, time.UTC)
	next := now.Add(time.Hour).Truncate(time.Hour)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE scheduled_test_plans SET next_run_at").
		WithArgs(int64(1), now, next).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	repo := &scheduledTestPlanRepository{db: db}
	claimed, err := repo.ClaimDueWithResult(context.Background(), 1, now, next, &service.ScheduledTestResult{StartedAt: now}, 6)
	require.NoError(t, err)
	require.False(t, claimed)
	require.NoError(t, mock.ExpectationsWereMet())
}
