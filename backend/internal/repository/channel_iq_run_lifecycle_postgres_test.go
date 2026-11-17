package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestChannelIQRunLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("CHANNEL_IQ_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated CHANNEL_IQ_TEST_DSN")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("iq_lifecycle_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	db.SetMaxOpenConns(24)
	t.Cleanup(func() { _ = db.Close(); _, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = base.Close() })
	_, err = db.Exec(`CREATE TABLE accounts (id BIGINT PRIMARY KEY); INSERT INTO accounts VALUES (1)`)
	require.NoError(t, err)
	for _, file := range []string{"066_add_scheduled_test_tables.sql", "070_add_scheduled_test_auto_recover.sql", "247_channel_iq_plan_fields.sql", "257_channel_iq_test_notes.sql", "258_channel_iq_run_lifecycle.sql"} {
		migration, err := migrations.FS.ReadFile(file)
		require.NoError(t, err)
		_, err = db.Exec(string(migration))
		require.NoError(t, err)
	}
	migration, err := migrations.FS.ReadFile("258_channel_iq_run_lifecycle.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err, "the migration must be repeatable")
	ctx := context.Background()
	plans, results := &scheduledTestPlanRepository{db: db}, &scheduledTestResultRepository{db: db}
	newPlan := func(enabled bool) int64 {
		t.Helper()
		_, err := db.Exec(`TRUNCATE scheduled_test_plans CASCADE`)
		require.NoError(t, err)
		var id int64
		require.NoError(t, db.QueryRow(`INSERT INTO scheduled_test_plans (account_id, kind, enabled, next_run_at) VALUES (1, 'channel_iq', $1, NOW() - INTERVAL '1 minute') RETURNING id`, enabled).Scan(&id))
		return id
	}
	newRun := func(key string) *service.ScheduledTestResult {
		now := time.Now()
		deadline := now.Add(30 * time.Minute)
		return &service.ScheduledTestResult{ModelID: "gpt-6.1-sol", Note: "test note", RequestKey: key, StartedAt: now, DeadlineAt: &deadline}
	}

	t.Run("concurrent tabs and completed retries reuse one result", func(t *testing.T) {
		id := newPlan(false)
		var started atomic.Int32
		var wg sync.WaitGroup
		gate := make(chan struct{})
		ids := make(chan int64, 16)
		errs := make(chan error, 16)
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-gate
				result, created, err := plans.StartManualRun(ctx, id, newRun(fmt.Sprintf("click-%d", i)), 6)
				if err != nil {
					errs <- err
					return
				}
				if created {
					started.Add(1)
				}
				ids <- result.ID
			}(i)
		}
		close(gate)
		wg.Wait()
		close(ids)
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.EqualValues(t, 1, started.Load())
		stored, err := results.ListByPlanID(ctx, id, 6)
		require.NoError(t, err)
		require.Len(t, stored, 1)
		for resultID := range ids {
			require.Equal(t, stored[0].ID, resultID)
		}
		stored[0].Status, stored[0].FinishedAt = "success", time.Now()
		require.NoError(t, results.CompleteRunning(ctx, stored[0]))
		replay, created, err := plans.StartManualRun(ctx, id, newRun(stored[0].RequestKey), 6)
		require.NoError(t, err)
		require.False(t, created)
		require.Equal(t, "success", replay.Status)
		require.Equal(t, stored[0].ID, replay.ID)
	})

	t.Run("manual and automatic starts cannot overlap", func(t *testing.T) {
		id := newPlan(false)
		manual, created, err := plans.StartManualRun(ctx, id, newRun("manual"), 6)
		require.NoError(t, err)
		require.True(t, created)
		_, err = db.Exec(`UPDATE scheduled_test_plans SET enabled = true WHERE id = $1`, id)
		require.NoError(t, err)
		now := time.Now()
		claimed, err := plans.ClaimDueWithResult(ctx, id, now, now.Add(time.Hour), newRun(""), 6)
		require.NoError(t, err)
		require.False(t, claimed)
		replayed, created, err := plans.StartManualRun(ctx, id, newRun("other-tab"), 6)
		require.NoError(t, err)
		require.False(t, created)
		require.Equal(t, manual.ID, replayed.ID)

		id = newPlan(true)
		claimed, err = plans.ClaimDueWithResult(ctx, id, now, now.Add(time.Hour), newRun(""), 6)
		require.NoError(t, err)
		require.True(t, claimed)
		replayed, created, err = plans.StartManualRun(ctx, id, newRun("manual-after-auto"), 6)
		require.NoError(t, err)
		require.False(t, created)
		require.Equal(t, "running", replayed.Status)
	})

	t.Run("manual timeout does not depend on the next hourly slot", func(t *testing.T) {
		id := newPlan(false)
		run := newRun("deadline")
		_, created, err := plans.StartManualRun(ctx, id, run, 6)
		require.NoError(t, err)
		require.True(t, created)
		count, err := results.FailStaleChannelIQRunning(ctx, time.Now().Add(time.Minute))
		require.NoError(t, err)
		require.Zero(t, count)
		count, err = results.FailStaleChannelIQRunning(ctx, run.DeadlineAt.Add(time.Second))
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
	})

	t.Run("finishing manual work preserves the current schedule", func(t *testing.T) {
		id := newPlan(false)
		next := time.Now().Add(5 * time.Minute).Truncate(time.Second)
		_, err := db.Exec(`UPDATE scheduled_test_plans SET enabled = true, next_run_at = $2 WHERE id = $1`, id, next)
		require.NoError(t, err)
		require.NoError(t, plans.UpdateLastRunAt(ctx, id, time.Now()))
		stored, err := plans.GetByID(ctx, id)
		require.NoError(t, err)
		require.True(t, next.Equal(*stored.NextRunAt))
	})
}
