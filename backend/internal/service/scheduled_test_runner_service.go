package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const scheduledTestDefaultMaxWorkers = 10

// ScheduledTestRunnerService periodically scans due test plans and executes them.
type ScheduledTestRunnerService struct {
	planRepo       ScheduledTestPlanRepository
	scheduledSvc   *ScheduledTestService
	accountTestSvc *AccountTestService
	rateLimitSvc   *RateLimitService
	cfg            *config.Config

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewScheduledTestRunnerService creates a new runner.
func NewScheduledTestRunnerService(
	planRepo ScheduledTestPlanRepository,
	scheduledSvc *ScheduledTestService,
	accountTestSvc *AccountTestService,
	rateLimitSvc *RateLimitService,
	cfg *config.Config,
) *ScheduledTestRunnerService {
	return &ScheduledTestRunnerService{
		planRepo:       planRepo,
		scheduledSvc:   scheduledSvc,
		accountTestSvc: accountTestSvc,
		rateLimitSvc:   rateLimitSvc,
		cfg:            cfg,
	}
}

// Start begins the cron ticker (every minute).
func (s *ScheduledTestRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] started (tick=every minute)")
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *ScheduledTestRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] cron stop timed out")
			}
		}
	})
}

func (s *ScheduledTestRunnerService) runScheduled() {
	// Delay 10s so execution lands at ~:10 of each minute instead of :00.
	time.Sleep(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	now := time.Now()
	staleBefore := now.Add(-channelIQTestTimeout - 2*time.Minute)
	if count, err := s.scheduledSvc.FailStaleChannelIQRunning(ctx, staleBefore); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] stale channel IQ recovery error: %v", err)
	} else if count > 0 {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] marked %d interrupted channel IQ runs as failed", count)
	}
	plans, err := s.planRepo.ListDue(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] found %d due plans", len(plans))

	sem := make(chan struct{}, scheduledTestDefaultMaxWorkers)
	var wg sync.WaitGroup

	for _, plan := range plans {
		nextRun, err := computeNextRun(plan.CronExpression, now)
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d computeNextRun error: %v", plan.ID, err)
			continue
		}
		var runningResult *ScheduledTestResult
		var claimed bool
		if plan.Kind == ChannelIQPlanKind {
			runningResult = &ScheduledTestResult{
				ModelID: plan.ModelID, ReasoningEffort: plan.ReasoningEffort, StartedAt: now,
			}
			claimed, err = s.planRepo.ClaimDueWithResult(ctx, plan.ID, now, nextRun, runningResult, channelIQResultLimit)
		} else {
			claimed, err = s.planRepo.ClaimDue(ctx, plan.ID, now, nextRun)
		}
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d claim error: %v", plan.ID, err)
			continue
		}
		if !claimed {
			// Another scheduler tick or application instance claimed the plan.
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(p *ScheduledTestPlan, pending *ScheduledTestResult) {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOnePlan(ctx, p, pending)
		}(plan, runningResult)
	}

	wg.Wait()
}

func (s *ScheduledTestRunnerService) runOnePlan(ctx context.Context, plan *ScheduledTestPlan, pending *ScheduledTestResult) {
	var result *ScheduledTestResult
	var err error
	if plan.Kind == ChannelIQPlanKind {
		probeCtx, cancel := newChannelIQProbeContext()
		defer cancel()
		ctx = probeCtx
		result, err = s.accountTestSvc.RunPromptTestBackgroundWithOptions(ctx, plan.AccountID, plan.ModelID, channelIQPromptForExecution(plan.Prompt), plan.ReasoningEffort)
	} else {
		result, err = s.accountTestSvc.RunTestBackground(ctx, plan.AccountID, plan.ModelID)
	}
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d RunTestBackground error: %v", plan.ID, err)
		if pending == nil {
			return
		}
		result = &ScheduledTestResult{
			Status: "failed", ErrorMessage: err.Error(), StartedAt: pending.StartedAt,
			FinishedAt: time.Now(), LatencyMs: time.Since(pending.StartedAt).Milliseconds(),
		}
	}
	if result == nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d returned no result", plan.ID)
		return
	}
	result.ModelID = plan.ModelID
	result.ReasoningEffort = plan.ReasoningEffort
	persistCtx, cancel := scheduledTestPersistenceContext(ctx)
	defer cancel()
	maxResults := plan.MaxResults
	if plan.Kind == ChannelIQPlanKind {
		maxResults = channelIQResultLimit
	}

	if pending != nil {
		result.ID = pending.ID
		err = s.scheduledSvc.CompleteRunningResult(persistCtx, result)
	} else {
		err = s.scheduledSvc.SaveResult(persistCtx, plan.ID, maxResults, result)
	}
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
	}

	// Auto-recover account if test succeeded and auto_recover is enabled.
	if result.Status == "success" && plan.AutoRecover {
		s.tryRecoverAccount(persistCtx, plan.AccountID, plan.ID)
	}

	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d computeNextRun error: %v", plan.ID, err)
		return
	}

	if err := s.planRepo.UpdateAfterRun(persistCtx, plan.ID, time.Now(), nextRun); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}
}

// tryRecoverAccount attempts to recover an account from recoverable runtime state.
func (s *ScheduledTestRunnerService) tryRecoverAccount(ctx context.Context, accountID int64, planID int64) {
	if s.rateLimitSvc == nil {
		return
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, accountID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover failed: %v", planID, err)
		return
	}
	if recovery == nil {
		return
	}

	if recovery.ClearedError {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d recovered from error status", planID, accountID)
	}
	if recovery.ClearedRateLimit {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d cleared rate-limit/runtime state", planID, accountID)
	}
}
