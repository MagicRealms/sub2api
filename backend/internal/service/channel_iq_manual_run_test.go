//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type iqManualPlanRepo struct {
	ScheduledTestPlanRepository
	mu        sync.Mutex
	plan      ScheduledTestPlan
	result    *ScheduledTestResult
	completed chan *ScheduledTestResult
}

func (r *iqManualPlanRepo) ListByKind(context.Context, string) ([]*ScheduledTestPlan, error) {
	copy := r.plan
	return []*ScheduledTestPlan{&copy}, nil
}

func (r *iqManualPlanRepo) StartManualRun(_ context.Context, id int64, result *ScheduledTestResult, _ int) (*ScheduledTestResult, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.result != nil && (r.result.Status == "running" || result.RequestKey == r.result.RequestKey) {
		copy := *r.result
		return &copy, false, nil
	}
	result.ID, result.PlanID, result.CreatedAt = 91, id, time.Now()
	copy := *result
	r.result = &copy
	return result, true, nil
}

func (r *iqManualPlanRepo) UpdateLastRunAt(context.Context, int64, time.Time) error {
	return nil
}

type iqManualResultRepo struct {
	ScheduledTestResultRepository
	plan *iqManualPlanRepo
}

func (r *iqManualResultRepo) CompleteRunning(ctx context.Context, result *ScheduledTestResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.plan.mu.Lock()
	copy := *result
	copy.RequestKey = r.plan.result.RequestKey
	r.plan.result = &copy
	r.plan.mu.Unlock()
	r.plan.completed <- &copy
	return nil
}

func (r *iqManualResultRepo) ListByPlanID(context.Context, int64, int) ([]*ScheduledTestResult, error) {
	r.plan.mu.Lock()
	defer r.plan.mu.Unlock()
	if r.plan.result == nil {
		return nil, nil
	}
	copy := *r.plan.result
	return []*ScheduledTestResult{&copy}, nil
}

type iqManualTester func(context.Context, int64, string, string, string) (*ScheduledTestResult, error)

func (f iqManualTester) RunPromptTestBackgroundWithOptions(ctx context.Context, id int64, model, prompt, effort string) (*ScheduledTestResult, error) {
	return f(ctx, id, model, prompt, effort)
}

func newIQManualTestService(tester iqManualTester) (*ChannelIQService, *iqManualPlanRepo) {
	repo := &iqManualPlanRepo{
		plan:      ScheduledTestPlan{ID: 1, AccountID: 2, ModelID: "gpt-6.1-sol", ReasoningEffort: "high", Note: "selected route"},
		completed: make(chan *ScheduledTestResult, 1),
	}
	results := &iqManualResultRepo{plan: repo}
	return &ChannelIQService{planRepo: repo, resultRepo: results, scheduled: NewScheduledTestService(repo, results), accountTest: tester}, repo
}

func TestChannelIQManualRunPublishesRunningAndDeduplicatesUntilComplete(t *testing.T) {
	var calls atomic.Int32
	entered, release := make(chan context.Context, 1), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	svc, repo := newIQManualTestService(func(ctx context.Context, _ int64, _, _, _ string) (*ScheduledTestResult, error) {
		calls.Add(1)
		entered <- ctx
		select {
		case <-release:
			return &ScheduledTestResult{Status: "success", ResponseText: "<html><svg></svg></html>", FinishedAt: time.Now()}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	card, err := svc.RunNow(ctx, "one-click")
	require.NoError(t, err)
	require.Equal(t, "running", card.Status)
	require.EqualValues(t, 91, card.ID)
	require.Equal(t, "selected route", card.Note)
	require.Nil(t, card.Evaluation)
	var probeCtx context.Context
	select {
	case probeCtx = <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("background probe was not started")
	}
	cancel()
	require.NoError(t, probeCtx.Err(), "closing the page must not cancel its persisted test")
	items, err := svc.listResults(context.Background(), &repo.plan)
	require.NoError(t, err)
	require.Equal(t, "running", items[0].Status)
	for _, key := range []string{"one-click", "second-tab", ""} {
		replayed, err := svc.RunNow(context.Background(), key)
		require.NoError(t, err)
		require.Equal(t, card.ID, replayed.ID)
	}
	require.EqualValues(t, 1, calls.Load())
	releaseOnce.Do(func() { close(release) })
	select {
	case result := <-repo.completed:
		require.Equal(t, "success", result.Status)
		require.Equal(t, card.ID, result.ID)
	case <-time.After(2 * time.Second):
		t.Fatal("running card was not completed")
	}
	completed, err := svc.RunNow(context.Background(), "one-click")
	require.NoError(t, err)
	require.Equal(t, "success", completed.Status)
	require.EqualValues(t, 1, calls.Load(), "replaying a completed request must not start another test")
}

func TestChannelIQManualRunPersistsFailuresInTheSameCard(t *testing.T) {
	for _, tc := range []struct {
		name  string
		panic bool
	}{{"probe error", false}, {"probe panic", true}} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := newIQManualTestService(func(context.Context, int64, string, string, string) (*ScheduledTestResult, error) {
				if tc.panic {
					panic("probe crashed")
				}
				return nil, errors.New("upstream failed")
			})
			card, err := svc.RunNow(context.Background(), "failed-click")
			require.NoError(t, err)
			select {
			case result := <-repo.completed:
				require.Equal(t, card.ID, result.ID)
				require.Equal(t, "failed", result.Status)
				require.NotEmpty(t, result.ErrorMessage)
			case <-time.After(2 * time.Second):
				t.Fatal("failed probe left a running card")
			}
		})
	}
}
