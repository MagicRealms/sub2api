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
	run := &service.ScheduledTestResult{ModelID: "gpt-6.1-sol", ReasoningEffort: "high", Note: "nightly probe", StartedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE scheduled_test_plans SET next_run_at").
		WithArgs(int64(1), now, next).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO scheduled_test_results").
		WithArgs(int64(1), run.ModelID, run.ReasoningEffort, run.Note, now).
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
		WithArgs(int64(1), "", "", "", now).WillReturnError(errors.New("insert failed"))
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

func TestChannelIQCompletionNeverSkipsOrRegressesClaimedNextHour(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	finished := time.Date(2026, 11, 4, 18, 0, 1, 0, time.UTC)
	nextHour := finished.Add(-time.Second)
	mock.ExpectExec("next_run_at = GREATEST\\(next_run_at, \\$3\\)").
		WithArgs(int64(1), finished, nextHour).WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &scheduledTestPlanRepository{db: db}
	require.NoError(t, repo.UpdateAfterRun(context.Background(), 1, finished, nextHour))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledTestPlanReadPreservesNote(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	nextRunAt := createdAt.Add(90 * time.Minute)
	mock.ExpectQuery("SELECT id, account_id, model_id, kind, prompt, note").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "account_id", "model_id", "kind", "prompt", "note", "reasoning_effort", "cron_expression",
			"enabled", "max_results", "auto_recover", "last_run_at", "next_run_at", "created_at", "updated_at",
		}).AddRow(7, 12, "gpt-6.1-sol", "channel_iq", "prompt", "nightly check", "high", "@every 90m", true, 6, false, nil, nextRunAt, createdAt, createdAt))

	plan, err := (&scheduledTestPlanRepository{db: db}).GetByID(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "nightly check", plan.Note)
	require.Equal(t, "@every 90m", plan.CronExpression)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledTestResultListPreservesNote(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	startedAt := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)
	mock.ExpectQuery("SELECT id, plan_id, status, model_id, reasoning_effort, note").
		WithArgs(int64(7), 6).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "plan_id", "status", "model_id", "reasoning_effort", "note", "response_text", "error_message",
			"latency_ms", "started_at", "finished_at", "created_at",
		}).AddRow(33, 7, "success", "gpt-6.1-sol", "high", "nightly check", "<html/>", "", 1000, startedAt, finishedAt, finishedAt))

	results, err := (&scheduledTestResultRepository{db: db}).ListByPlanID(context.Background(), 7, 6)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "nightly check", results[0].Note)
	require.NoError(t, mock.ExpectationsWereMet())
}
