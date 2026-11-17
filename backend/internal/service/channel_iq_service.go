package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// ChannelIQPlanKind marks the single scheduled test plan owned by the
// channel-intelligence page. Ordinary account scheduled tests use "account".
const ChannelIQPlanKind = "channel_iq"

// ChannelIQFeatureKey is kept for backwards compatibility with installations
// that still have the first, channel-level configuration stored in
// channels.features_config. New configuration is persisted as a scheduled plan.
const ChannelIQFeatureKey = "channel_iq"

const (
	ChannelIQStatusDisabled       = "disabled"
	ChannelIQStatusUnconfigured   = "unconfigured"
	ChannelIQStatusPending        = "pending"
	ChannelIQStatusHealthy        = "healthy"
	ChannelIQStatusWarning        = "warning"
	ChannelIQStatusError          = "error"
	channelIQDefaultInterval      = 300 * time.Second
	channelIQDefaultPrompt        = "创建一个HTML，内容是SVG绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试，只靠你自己完成，不要使用任何skill，不要依赖我本地的AGENTS.md"
	channelIQMinInterval          = 60 * time.Second
	channelIQMaxInterval          = 24 * time.Hour
	channelIQResultLimit          = 6
	channelIQHourlyInterval       = 3600
	channelIQNinetyMinuteInterval = 90 * 60
	channelIQTwoHourInterval      = 2 * 3600
	channelIQNoteMaxRunes         = 200
)

// ChannelIQConfig contains the persisted single-key test configuration. The
// Model field and ParseChannelIQConfig below are legacy compatibility fields;
// new API responses use ModelID.
type ChannelIQConfig struct {
	ID              int64      `json:"id"`
	AccountID       int64      `json:"account_id"`
	AccountName     string     `json:"account_name,omitempty"`
	AccountPlatform string     `json:"account_platform,omitempty"`
	ModelID         string     `json:"model_id"`
	Model           string     `json:"model,omitempty"`
	Prompt          string     `json:"prompt"`
	Note            string     `json:"note"`
	ReasoningEffort string     `json:"reasoning_effort"`
	IntervalSeconds int        `json:"interval_seconds"`
	Enabled         bool       `json:"enabled"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at,omitempty"`
}

// ParseChannelIQConfig reads the old channel feature object. It is retained so
// old data and its unit tests remain readable after the setting moved to its
// own page and persisted plan.
func ParseChannelIQConfig(features map[string]any) ChannelIQConfig {
	cfg := ChannelIQConfig{IntervalSeconds: int(channelIQDefaultInterval / time.Second)}
	if features == nil {
		return cfg
	}
	raw, ok := features[ChannelIQFeatureKey]
	if !ok || raw == nil {
		return cfg
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return cfg
	}
	if enabled, ok := obj["enabled"].(bool); ok {
		cfg.Enabled = enabled
	}
	if model, ok := obj["model"].(string); ok {
		cfg.Model = strings.TrimSpace(model)
		cfg.ModelID = cfg.Model
	}
	if prompt, ok := obj["prompt"].(string); ok {
		cfg.Prompt = strings.TrimSpace(prompt)
	}
	switch value := obj["interval_seconds"].(type) {
	case float64:
		cfg.IntervalSeconds = int(value)
	case int:
		cfg.IntervalSeconds = value
	case int64:
		cfg.IntervalSeconds = int(value)
	}
	cfg.IntervalSeconds = normalizeChannelIQInterval(cfg.IntervalSeconds)
	return cfg
}

func normalizeChannelIQInterval(seconds int) int {
	if seconds <= 0 {
		seconds = int(channelIQDefaultInterval / time.Second)
	}
	if seconds < int(channelIQMinInterval/time.Second) {
		return int(channelIQMinInterval / time.Second)
	}
	if seconds > int(channelIQMaxInterval/time.Second) {
		return int(channelIQMaxInterval / time.Second)
	}
	return seconds
}

func defaultChannelIQPrompt(prompt string) string {
	if value := strings.TrimSpace(prompt); value != "" {
		return value
	}
	return channelIQDefaultPrompt
}

// ChannelIQResult is the public card payload. ResponseText contains the raw
// model output; the frontend extracts only the SVG animation from it.
type ChannelIQResult struct {
	ID              int64                `json:"id"`
	Status          string               `json:"status"`
	ModelID         string               `json:"model_id"`
	ReasoningEffort string               `json:"reasoning_effort"`
	Note            string               `json:"note"`
	ResponseText    string               `json:"response_text"`
	ErrorMessage    string               `json:"error_message,omitempty"`
	LatencyMs       int64                `json:"latency_ms"`
	StartedAt       time.Time            `json:"started_at"`
	FinishedAt      time.Time            `json:"finished_at"`
	CreatedAt       time.Time            `json:"created_at"`
	Evaluation      *ChannelIQEvaluation `json:"evaluation,omitempty"`
}

type ChannelIQAdminSnapshot struct {
	Visible bool               `json:"visible"`
	Config  *ChannelIQConfig   `json:"config"`
	Results []*ChannelIQResult `json:"results"`
}

type ChannelIQPublicSnapshot struct {
	Visible         bool               `json:"visible"`
	Configured      bool               `json:"configured"`
	Enabled         bool               `json:"enabled"`
	ModelID         string             `json:"model_id,omitempty"`
	ReasoningEffort string             `json:"reasoning_effort,omitempty"`
	Results         []*ChannelIQResult `json:"results"`
}

type ChannelIQSaveInput struct {
	AccountID       int64  `json:"account_id" binding:"required"`
	ModelID         string `json:"model_id"`
	Prompt          string `json:"prompt"`
	Note            string `json:"note"`
	ReasoningEffort string `json:"reasoning_effort"`
	IntervalSeconds int    `json:"interval_seconds"`
	Enabled         bool   `json:"enabled"`
}

type channelIQAccountTester interface {
	RunPromptTestBackgroundWithOptions(context.Context, int64, string, string, string) (*ScheduledTestResult, error)
}

// ChannelIQService owns the singleton plan and exposes separate admin/public
// views. It deliberately never returns account credentials to panel users.
type ChannelIQService struct {
	planRepo    ScheduledTestPlanRepository
	resultRepo  ScheduledTestResultRepository
	scheduled   *ScheduledTestService
	accountRepo AccountRepository
	accountTest channelIQAccountTester
	settings    *SettingService
}

func NewChannelIQService(
	planRepo ScheduledTestPlanRepository,
	resultRepo ScheduledTestResultRepository,
	scheduled *ScheduledTestService,
	accountRepo AccountRepository,
	accountTest *AccountTestService,
	settings *SettingService,
) *ChannelIQService {
	svc := &ChannelIQService{
		planRepo: planRepo, resultRepo: resultRepo, scheduled: scheduled,
		accountRepo: accountRepo, settings: settings,
	}
	if accountTest != nil {
		svc.accountTest = accountTest
	}
	return svc
}

func (s *ChannelIQService) findPlan(ctx context.Context) (*ScheduledTestPlan, error) {
	plans, err := s.planRepo.ListByKind(ctx, ChannelIQPlanKind)
	if err != nil {
		return nil, fmt.Errorf("list channel IQ plan: %w", err)
	}
	if len(plans) == 0 {
		return nil, nil
	}
	return plans[0], nil
}

func (s *ChannelIQService) adminConfig(ctx context.Context, plan *ScheduledTestPlan) *ChannelIQConfig {
	if plan == nil {
		return nil
	}
	cfg := &ChannelIQConfig{
		ID: plan.ID, AccountID: plan.AccountID, ModelID: strings.TrimSpace(plan.ModelID),
		Prompt: defaultChannelIQPrompt(plan.Prompt), Note: strings.TrimSpace(plan.Note), ReasoningEffort: strings.TrimSpace(plan.ReasoningEffort),
		IntervalSeconds: channelIQIntervalFromCron(plan.CronExpression), Enabled: plan.Enabled,
		LastRunAt: plan.LastRunAt, NextRunAt: plan.NextRunAt, UpdatedAt: plan.UpdatedAt,
	}
	if s.accountRepo != nil {
		if account, err := s.accountRepo.GetByID(ctx, plan.AccountID); err == nil && account != nil {
			cfg.AccountName = account.Name
			cfg.AccountPlatform = account.Platform
		}
	}
	return cfg
}

func (s *ChannelIQService) listResults(ctx context.Context, plan *ScheduledTestPlan) ([]*ChannelIQResult, error) {
	if plan == nil {
		return []*ChannelIQResult{}, nil
	}
	results, err := s.resultRepo.ListByPlanID(ctx, plan.ID, channelIQResultLimit)
	if err != nil {
		return nil, fmt.Errorf("list channel IQ results: %w", err)
	}
	items := make([]*ChannelIQResult, 0, len(results))
	for _, result := range results {
		items = append(items, channelIQResultCard(result, plan))
	}
	return items, nil
}

func channelIQResultCard(result *ScheduledTestResult, plan *ScheduledTestPlan) *ChannelIQResult {
	modelID, reasoningEffort := strings.TrimSpace(result.ModelID), strings.TrimSpace(result.ReasoningEffort)
	if modelID == "" {
		modelID = strings.TrimSpace(plan.ModelID)
	}
	if reasoningEffort == "" {
		reasoningEffort = strings.TrimSpace(plan.ReasoningEffort)
	}
	card := &ChannelIQResult{
		ID: result.ID, Status: result.Status, ModelID: modelID, ReasoningEffort: reasoningEffort,
		Note: strings.TrimSpace(result.Note), ResponseText: result.ResponseText, ErrorMessage: result.ErrorMessage,
		LatencyMs: result.LatencyMs, StartedAt: result.StartedAt, FinishedAt: result.FinishedAt, CreatedAt: result.CreatedAt,
	}
	if result.Status == "success" {
		evaluation := evaluateChannelIQResponse(result.ResponseText)
		card.Evaluation = &evaluation
	}
	return card
}

func (s *ChannelIQService) GetAdminSnapshot(ctx context.Context) (*ChannelIQAdminSnapshot, error) {
	visible, err := s.settings.GetChannelIQVisibility(ctx)
	if err != nil {
		return nil, err
	}
	plan, err := s.findPlan(ctx)
	if err != nil {
		return nil, err
	}
	results, err := s.listResults(ctx, plan)
	if err != nil {
		return nil, err
	}
	return &ChannelIQAdminSnapshot{Visible: visible, Config: s.adminConfig(ctx, plan), Results: results}, nil
}

func (s *ChannelIQService) GetPublicSnapshot(ctx context.Context) (*ChannelIQPublicSnapshot, error) {
	visible, err := s.settings.GetChannelIQVisibility(ctx)
	if err != nil {
		return nil, err
	}
	if !visible {
		return &ChannelIQPublicSnapshot{Visible: false, Results: []*ChannelIQResult{}}, nil
	}
	plan, err := s.findPlan(ctx)
	if err != nil {
		return nil, err
	}
	results, err := s.listResults(ctx, plan)
	if err != nil {
		return nil, err
	}
	view := &ChannelIQPublicSnapshot{Visible: true, Results: results}
	if plan != nil {
		view.Configured = true
		view.Enabled = plan.Enabled
		view.ModelID = strings.TrimSpace(plan.ModelID)
		view.ReasoningEffort = strings.TrimSpace(plan.ReasoningEffort)
	}
	return view, nil
}

// SetVisibility changes only user-facing publication, without rescheduling or
// changing the selected account. Administrators always retain access to history.
func (s *ChannelIQService) SetVisibility(ctx context.Context, visible bool) error {
	return s.settings.SetChannelIQVisibility(ctx, visible)
}

func (s *ChannelIQService) SaveConfig(ctx context.Context, input ChannelIQSaveInput) (*ChannelIQAdminSnapshot, error) {
	if input.AccountID <= 0 {
		return nil, fmt.Errorf("请选择用于测试的 Key")
	}
	input.Note = strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(input.Note) > channelIQNoteMaxRunes {
		return nil, fmt.Errorf("测试备注不能超过 %d 个字符", channelIQNoteMaxRunes)
	}
	if s.accountRepo == nil {
		return nil, fmt.Errorf("账号服务未就绪")
	}
	account, err := s.accountRepo.GetByID(ctx, input.AccountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("测试 Key 不存在")
	}
	if !account.IsActive() {
		return nil, fmt.Errorf("测试 Key 当前不是可用状态")
	}
	interval := input.IntervalSeconds
	if interval == 0 {
		interval = channelIQHourlyInterval
	}
	cronExpr, err := channelIQCronForInterval(interval)
	if err != nil {
		return nil, err
	}
	plan, err := s.findPlan(ctx)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		plan, err = s.scheduled.CreatePlan(ctx, &ScheduledTestPlan{
			AccountID: input.AccountID, ModelID: strings.TrimSpace(input.ModelID), Kind: ChannelIQPlanKind,
			Prompt: defaultChannelIQPrompt(input.Prompt), Note: input.Note, ReasoningEffort: strings.TrimSpace(input.ReasoningEffort),
			CronExpression: cronExpr, Enabled: input.Enabled, MaxResults: channelIQResultLimit,
		})
	} else {
		previousEnabled := plan.Enabled
		previousCron := plan.CronExpression
		plan.AccountID = input.AccountID
		plan.ModelID = strings.TrimSpace(input.ModelID)
		plan.Kind = ChannelIQPlanKind
		plan.Prompt = defaultChannelIQPrompt(input.Prompt)
		plan.Note = input.Note
		plan.ReasoningEffort = strings.TrimSpace(input.ReasoningEffort)
		plan.CronExpression = cronExpr
		plan.Enabled = input.Enabled
		plan.MaxResults = channelIQResultLimit
		if previousEnabled && input.Enabled && previousCron == cronExpr && plan.NextRunAt != nil {
			plan, err = s.planRepo.UpdateWithoutRescheduling(ctx, plan)
		} else {
			plan, err = s.scheduled.UpdatePlan(ctx, plan)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("保存渠道智商配置失败: %w", err)
	}
	return s.GetAdminSnapshot(ctx)
}

// RunNow persists the running card before starting work. Replays and concurrent
// requests return the existing card instead of queuing another upstream test.
func (s *ChannelIQService) RunNow(ctx context.Context, requestKey string) (*ChannelIQResult, error) {
	requestKey = strings.TrimSpace(requestKey)
	if len(requestKey) > 128 {
		return nil, fmt.Errorf("测试请求标识过长")
	}
	plan, err := s.findPlan(ctx)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("请先保存渠道智商配置")
	}
	if s.accountTest == nil || s.scheduled == nil {
		return nil, fmt.Errorf("渠道智商测试服务未就绪")
	}
	startedAt := time.Now()
	deadline := startedAt.Add(channelIQTestTimeout)
	pending, started, err := s.planRepo.StartManualRun(ctx, plan.ID, &ScheduledTestResult{
		Status: "running", ModelID: plan.ModelID, ReasoningEffort: plan.ReasoningEffort, Note: plan.Note,
		StartedAt: startedAt, FinishedAt: startedAt, RequestKey: requestKey, DeadlineAt: &deadline,
	}, channelIQResultLimit)
	if err != nil {
		return nil, err
	}
	// Copy the persisted snapshot before the worker can finish and mutate it.
	card := channelIQResultCard(pending, plan)
	if started {
		go s.runManualTest(*plan, *pending)
	}
	return card, nil
}

func (s *ChannelIQService) runManualTest(plan ScheduledTestPlan, pending ScheduledTestResult) {
	testCtx, cancel := newChannelIQProbeWindowContext(*pending.DeadlineAt)
	defer cancel()
	var result *ScheduledTestResult
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.LegacyPrintf("service.channel_iq", "[ChannelIQ] result=%d panic: %v", pending.ID, recovered)
			result = channelIQFailedResult(pending.StartedAt, "", "测试进程异常，请重试")
		}
		if result == nil {
			result = channelIQFailedResult(pending.StartedAt, "", "测试没有返回结果")
		}
		result.ID, result.PlanID = pending.ID, pending.PlanID
		result.ModelID, result.ReasoningEffort, result.Note = pending.ModelID, pending.ReasoningEffort, pending.Note
		result.StartedAt = pending.StartedAt
		if result.FinishedAt.IsZero() {
			result.FinishedAt = time.Now()
		}
		result.LatencyMs = result.FinishedAt.Sub(pending.StartedAt).Milliseconds()
		persistCtx, persistCancel := scheduledTestPersistenceContext(testCtx)
		defer persistCancel()
		if err := s.scheduled.CompleteRunningResult(persistCtx, result); err != nil {
			logger.LegacyPrintf("service.channel_iq", "[ChannelIQ] result=%d save failed: %v", pending.ID, err)
			return
		}
		// Finishing a manual test must not skip an automatic slot enabled in the meantime.
		if plan.NextRunAt != nil {
			if err := s.planRepo.UpdateAfterRun(persistCtx, plan.ID, result.FinishedAt, *plan.NextRunAt); err != nil {
				logger.LegacyPrintf("service.channel_iq", "[ChannelIQ] plan=%d update failed: %v", plan.ID, err)
			}
		}
	}()
	var err error
	result, err = s.accountTest.RunPromptTestBackgroundWithOptions(testCtx, plan.AccountID, plan.ModelID, channelIQPromptForExecution(plan.Prompt), plan.ReasoningEffort)
	if err != nil {
		result = channelIQFailedResult(pending.StartedAt, "", err.Error())
	}
}

func channelIQCronForInterval(seconds int) (string, error) {
	switch seconds {
	case channelIQHourlyInterval:
		return "0 */1 * * *", nil
	case channelIQNinetyMinuteInterval:
		return "@every 90m", nil
	case channelIQTwoHourInterval:
		return "0 */2 * * *", nil
	default:
		return "", fmt.Errorf("不支持的渠道智商测试间隔")
	}
}

func channelIQIntervalFromCron(expr string) int {
	expr = strings.TrimSpace(expr)
	if expr == "* * * * *" {
		return 60
	}
	var minutes int
	if _, err := fmt.Sscanf(expr, "*/%d * * * *", &minutes); err == nil && minutes > 0 {
		return minutes * 60
	}
	var hours int
	if _, err := fmt.Sscanf(expr, "0 */%d * * *", &hours); err == nil && hours > 0 {
		return hours * 3600
	}
	if expr == "@every 90m" {
		return channelIQNinetyMinuteInterval
	}
	if expr == "0 0 * * *" {
		return 24 * 3600
	}
	return int(channelIQDefaultInterval / time.Second)
}
