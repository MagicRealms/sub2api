package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// --- Plan Repository ---

type scheduledTestPlanRepository struct {
	db *sql.DB
}

func NewScheduledTestPlanRepository(db *sql.DB) service.ScheduledTestPlanRepository {
	return &scheduledTestPlanRepository{db: db}
}

func (r *scheduledTestPlanRepository) Create(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	kind := plan.Kind
	if kind == "" {
		kind = "account"
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_plans (account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, next_run_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at
	`, plan.AccountID, plan.ModelID, kind, plan.Prompt, plan.ReasoningEffort, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.NextRunAt)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) GetByID(ctx context.Context, id int64) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans WHERE id = $1
	`, id)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) ListByAccountID(ctx context.Context, accountID int64) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans WHERE account_id = $1 AND kind = 'account'
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) ListByKind(ctx context.Context, kind string) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans WHERE kind = $1
		ORDER BY created_at ASC
	`, kind)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) ListDue(ctx context.Context, now time.Time) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans
		WHERE enabled = true AND next_run_at <= $1
		ORDER BY next_run_at ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) Update(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	kind := plan.Kind
	if kind == "" {
		kind = "account"
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET account_id = $2, model_id = $3, kind = $4, prompt = $5, reasoning_effort = $6, cron_expression = $7, enabled = $8, max_results = $9, auto_recover = $10, next_run_at = $11, updated_at = NOW()
		WHERE id = $1
		RETURNING id, account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at
	`, plan.ID, plan.AccountID, plan.ModelID, kind, plan.Prompt, plan.ReasoningEffort, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.NextRunAt)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) UpdateWithoutRescheduling(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET account_id = $2, model_id = $3, prompt = $4, reasoning_effort = $5,
		    enabled = $6, max_results = $7, updated_at = NOW()
		WHERE id = $1 AND kind = 'channel_iq'
		RETURNING id, account_id, model_id, kind, prompt, reasoning_effort, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at
	`, plan.ID, plan.AccountID, plan.ModelID, plan.Prompt, plan.ReasoningEffort, plan.Enabled, plan.MaxResults)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id = $1`, id)
	return err
}

func (r *scheduledTestPlanRepository) ClaimDue(ctx context.Context, id int64, now time.Time, nextRunAt time.Time) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans
		SET next_run_at = $3, updated_at = NOW()
		WHERE id = $1 AND enabled = true AND next_run_at <= $2
	`, id, now, nextRunAt)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (r *scheduledTestPlanRepository) ClaimDueWithResult(ctx context.Context, id int64, now time.Time, nextRunAt time.Time, run *service.ScheduledTestResult, keepCount int) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	claimed, err := tx.ExecContext(ctx, `
		UPDATE scheduled_test_plans SET next_run_at = $3, updated_at = NOW()
		WHERE id = $1 AND kind = 'channel_iq' AND enabled = true AND next_run_at <= $2
	`, id, now, nextRunAt)
	if err != nil {
		return false, err
	}
	rows, err := claimed.RowsAffected()
	if err != nil || rows == 0 {
		return false, err
	}
	run.PlanID = id
	err = tx.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_results (plan_id, status, model_id, reasoning_effort, response_text, error_message, latency_ms, started_at, finished_at, created_at)
		VALUES ($1, 'running', $2, $3, '', '', 0, $4, $4, NOW()) RETURNING id
	`, id, run.ModelID, run.ReasoningEffort, run.StartedAt).Scan(&run.ID)
	if err != nil {
		return false, err
	}
	if keepCount > 0 {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM scheduled_test_results WHERE id IN (
				SELECT id FROM scheduled_test_results WHERE plan_id = $1
				ORDER BY created_at DESC, id DESC OFFSET $2
			)
		`, id, keepCount)
		if err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit channel IQ claim: %w", err)
	}
	return true, nil
}

func (r *scheduledTestPlanRepository) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans SET last_run_at = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1
	`, id, lastRunAt, nextRunAt)
	return err
}

// --- Result Repository ---

type scheduledTestResultRepository struct {
	db *sql.DB
}

func NewScheduledTestResultRepository(db *sql.DB) service.ScheduledTestResultRepository {
	return &scheduledTestResultRepository{db: db}
}

func (r *scheduledTestResultRepository) Create(ctx context.Context, result *service.ScheduledTestResult) (*service.ScheduledTestResult, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_results (plan_id, status, model_id, reasoning_effort, response_text, error_message, latency_ms, started_at, finished_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id, plan_id, status, model_id, reasoning_effort, response_text, error_message, latency_ms, started_at, finished_at, created_at
	`, result.PlanID, result.Status, result.ModelID, result.ReasoningEffort, result.ResponseText, result.ErrorMessage, result.LatencyMs, result.StartedAt, result.FinishedAt)

	out := &service.ScheduledTestResult{}
	if err := row.Scan(
		&out.ID, &out.PlanID, &out.Status, &out.ModelID, &out.ReasoningEffort, &out.ResponseText, &out.ErrorMessage,
		&out.LatencyMs, &out.StartedAt, &out.FinishedAt, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *scheduledTestResultRepository) CompleteRunning(ctx context.Context, result *service.ScheduledTestResult) error {
	updated, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_results SET status = $2, model_id = $3, reasoning_effort = $4,
		response_text = $5, error_message = $6, latency_ms = $7, started_at = $8, finished_at = $9
		WHERE id = $1 AND status = 'running'
	`, result.ID, result.Status, result.ModelID, result.ReasoningEffort, result.ResponseText,
		result.ErrorMessage, result.LatencyMs, result.StartedAt, result.FinishedAt)
	if err != nil {
		return err
	}
	rows, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("channel IQ running result %d was not found", result.ID)
	}
	return nil
}

func (r *scheduledTestResultRepository) FailStaleChannelIQRunning(ctx context.Context, before time.Time) (int64, error) {
	updated, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_results AS result
		SET status = 'failed', error_message = '测试进程中断或超时', finished_at = NOW(),
		    latency_ms = GREATEST(0, (EXTRACT(EPOCH FROM (NOW() - result.started_at)) * 1000)::bigint)
		FROM scheduled_test_plans AS plan
		WHERE result.plan_id = plan.id AND plan.kind = 'channel_iq'
		  AND result.status = 'running' AND result.started_at < $1
	`, before)
	if err != nil {
		return 0, err
	}
	return updated.RowsAffected()
}

func (r *scheduledTestResultRepository) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*service.ScheduledTestResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plan_id, status, model_id, reasoning_effort, response_text, error_message, latency_ms, started_at, finished_at, created_at
		FROM scheduled_test_results
		WHERE plan_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, planID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*service.ScheduledTestResult
	for rows.Next() {
		r := &service.ScheduledTestResult{}
		if err := rows.Scan(
			&r.ID, &r.PlanID, &r.Status, &r.ModelID, &r.ReasoningEffort, &r.ResponseText, &r.ErrorMessage,
			&r.LatencyMs, &r.StartedAt, &r.FinishedAt, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *scheduledTestResultRepository) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM scheduled_test_results
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY plan_id ORDER BY created_at DESC) AS rn
				FROM scheduled_test_results
				WHERE plan_id = $1
			) ranked
			WHERE rn > $2
		)
	`, planID, keepCount)
	return err
}

// --- scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanPlan(row scannable) (*service.ScheduledTestPlan, error) {
	p := &service.ScheduledTestPlan{}
	if err := row.Scan(
		&p.ID, &p.AccountID, &p.ModelID, &p.Kind, &p.Prompt, &p.ReasoningEffort, &p.CronExpression, &p.Enabled, &p.MaxResults, &p.AutoRecover,
		&p.LastRunAt, &p.NextRunAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return p, nil
}

func scanPlans(rows *sql.Rows) ([]*service.ScheduledTestPlan, error) {
	var plans []*service.ScheduledTestPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
