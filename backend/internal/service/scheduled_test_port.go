package service

import (
	"context"
	"time"
)

// ScheduledTestPlan represents a scheduled test plan domain model.
type ScheduledTestPlan struct {
	ID        int64  `json:"id"`
	AccountID int64  `json:"account_id"`
	ModelID   string `json:"model_id"`
	// Kind identifies the owner of a plan. Empty values are normalized to
	// "account" for the existing account connectivity monitor. The dedicated
	// channel IQ plan uses service.ChannelIQPlanKind.
	Kind            string     `json:"kind"`
	Prompt          string     `json:"prompt"`
	Note            string     `json:"note"`
	ReasoningEffort string     `json:"reasoning_effort"`
	CronExpression  string     `json:"cron_expression"`
	Enabled         bool       `json:"enabled"`
	MaxResults      int        `json:"max_results"`
	AutoRecover     bool       `json:"auto_recover"`
	LastRunAt       *time.Time `json:"last_run_at"`
	NextRunAt       *time.Time `json:"next_run_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ScheduledTestResult represents a single test execution result.
type ScheduledTestResult struct {
	ID              int64     `json:"id"`
	PlanID          int64     `json:"plan_id"`
	Status          string    `json:"status"`
	ModelID         string    `json:"model_id"`
	ReasoningEffort string    `json:"reasoning_effort"`
	Note            string    `json:"note"`
	ResponseText    string    `json:"response_text"`
	ErrorMessage    string    `json:"error_message"`
	LatencyMs       int64     `json:"latency_ms"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// ScheduledTestPlanRepository defines the data access interface for test plans.
type ScheduledTestPlanRepository interface {
	Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error)
	ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error)
	ListByKind(ctx context.Context, kind string) ([]*ScheduledTestPlan, error)
	ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error)
	Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	UpdateWithoutRescheduling(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	Delete(ctx context.Context, id int64) error
	// ClaimDue advances next_run_at only when the plan is still due. This keeps
	// overlapping scheduler ticks or multiple app instances from running a
	// slow plan more than once.
	ClaimDue(ctx context.Context, id int64, now time.Time, nextRunAt time.Time) (bool, error)
	// ClaimDueWithResult atomically records a channel IQ attempt when claiming its hour.
	ClaimDueWithResult(ctx context.Context, id int64, now time.Time, nextRunAt time.Time, result *ScheduledTestResult, keepCount int) (bool, error)
	UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error
}

// ScheduledTestResultRepository defines the data access interface for test results.
type ScheduledTestResultRepository interface {
	Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error)
	CompleteRunning(ctx context.Context, result *ScheduledTestResult) error
	FailStaleChannelIQRunning(ctx context.Context, before time.Time) (int64, error)
	ListByPlanID(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error)
	PruneOldResults(ctx context.Context, planID int64, keepCount int) error
}
