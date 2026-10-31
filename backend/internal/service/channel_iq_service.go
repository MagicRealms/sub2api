package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ChannelIQFeatureKey is the JSON key stored in channels.features_config.
const ChannelIQFeatureKey = "channel_iq"

const (
	ChannelIQStatusDisabled     = "disabled"
	ChannelIQStatusUnconfigured = "unconfigured"
	ChannelIQStatusPending      = "pending"
	ChannelIQStatusHealthy      = "healthy"
	ChannelIQStatusWarning      = "warning"
	ChannelIQStatusError        = "error"
	channelIQDefaultInterval    = 300 * time.Second
	channelIQDefaultTimeout     = 60 * time.Second
	channelIQDefaultPrompt      = "请用一句话回答：2+2等于多少？只输出结果。"
	channelIQMaxPreviewRunes    = 240
	channelIQMinInterval        = 60 * time.Second
	channelIQMaxInterval        = 24 * time.Hour
)

// ChannelIQConfig controls the optional live AI ability probe for a channel.
// It intentionally lives in channels.features_config so existing channel CRUD
// and backup/restore paths keep the setting without a schema migration.
type ChannelIQConfig struct {
	Enabled         bool   `json:"enabled"`
	Model           string `json:"model"`
	Prompt          string `json:"prompt"`
	IntervalSeconds int    `json:"interval_seconds"`
}

// ParseChannelIQConfig reads a tolerant JSON-decoded feature config. Older
// channels simply get a disabled config and therefore never trigger upstream
// traffic until an administrator enables the feature.
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
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = int(channelIQDefaultInterval / time.Second)
	}
	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	if interval < channelIQMinInterval {
		cfg.IntervalSeconds = int(channelIQMinInterval / time.Second)
	} else if interval > channelIQMaxInterval {
		cfg.IntervalSeconds = int(channelIQMaxInterval / time.Second)
	}
	return cfg
}

func (c ChannelIQConfig) interval() time.Duration {
	return time.Duration(c.IntervalSeconds) * time.Second
}

func (c ChannelIQConfig) prompt() string {
	if strings.TrimSpace(c.Prompt) == "" {
		return channelIQDefaultPrompt
	}
	return strings.TrimSpace(c.Prompt)
}

// ChannelIQStatus is the public, deliberately compact view shown to all
// authenticated panel users. It does not include the configured prompt or an
// account ID; the response preview is capped to avoid leaking a full answer.
type ChannelIQStatus struct {
	ChannelID       int64      `json:"channel_id"`
	ChannelName     string     `json:"channel_name"`
	GroupID         int64      `json:"group_id,omitempty"`
	GroupName       string     `json:"group_name,omitempty"`
	Platform        string     `json:"platform,omitempty"`
	Model           string     `json:"model,omitempty"`
	Status          string     `json:"status"`
	LatencyMs       int64      `json:"latency_ms,omitempty"`
	ResponsePreview string     `json:"response_preview,omitempty"`
	Error           string     `json:"error,omitempty"`
	CheckedAt       *time.Time `json:"checked_at,omitempty"`
	NextCheckAt     *time.Time `json:"next_check_at,omitempty"`
	IntervalSeconds int        `json:"interval_seconds,omitempty"`
}

type channelIQCachedResult struct {
	status ChannelIQStatus
	when   time.Time
}

// ChannelIQService runs lightweight, rate-limited probes using the same
// account test implementation as the admin connectivity test. Results are
// kept in memory because the feature is an operational status view; channel
// configuration itself remains durable in features_config.
type ChannelIQService struct {
	channelRepo ChannelRepository
	groupRepo   GroupRepository
	accountRepo AccountRepository
	accountTest *AccountTestService

	mu       sync.Mutex
	results  map[int64]channelIQCachedResult
	inFlight map[int64]bool
	probeSem chan struct{}
}

func NewChannelIQService(
	channelRepo ChannelRepository,
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	accountTest *AccountTestService,
) *ChannelIQService {
	return &ChannelIQService{
		channelRepo: channelRepo,
		groupRepo:   groupRepo,
		accountRepo: accountRepo,
		accountTest: accountTest,
		results:     make(map[int64]channelIQCachedResult),
		inFlight:    make(map[int64]bool),
		// Keep a page refresh from creating a burst of upstream requests when
		// many channels are enabled at once.
		probeSem: make(chan struct{}, 2),
	}
}

// List returns the latest status for every channel and schedules stale probes
// asynchronously. The first request after enabling a channel intentionally
// returns "pending" so the panel never blocks on a slow upstream.
func (s *ChannelIQService) List(ctx context.Context) ([]ChannelIQStatus, error) {
	channels, err := s.channelRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels for IQ status: %w", err)
	}
	items := make([]ChannelIQStatus, 0, len(channels))
	for i := range channels {
		channel := channels[i]
		cfg := ParseChannelIQConfig(channel.FeaturesConfig)
		item := s.baseStatus(channel, cfg)
		if !cfg.Enabled {
			items = append(items, item)
			continue
		}
		if cached, ok := s.cached(channel.ID); ok {
			item = cached.status
			if time.Since(cached.when) >= cfg.interval() {
				s.trigger(channel)
			}
		} else {
			item.Status = ChannelIQStatusPending
			s.trigger(channel)
		}
		item.NextCheckAt = s.nextCheck(channel.ID, cfg)
		items = append(items, item)
	}
	return items, nil
}

// Run executes one channel probe synchronously for the manual refresh button.
// A concurrent scheduled probe is reused rather than duplicated.
func (s *ChannelIQService) Run(ctx context.Context, channelID int64) (*ChannelIQStatus, error) {
	channel, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	cfg := ParseChannelIQConfig(channel.FeaturesConfig)
	if !cfg.Enabled {
		status := s.baseStatus(*channel, cfg)
		return &status, nil
	}
	if s.isInFlight(channelID) {
		if cached, ok := s.cached(channelID); ok {
			status := cached.status
			status.Status = ChannelIQStatusPending
			return &status, nil
		}
		status := s.baseStatus(*channel, cfg)
		status.Status = ChannelIQStatusPending
		return &status, nil
	}
	status := s.runChannel(ctx, *channel)
	return &status, nil
}

func (s *ChannelIQService) baseStatus(channel Channel, cfg ChannelIQConfig) ChannelIQStatus {
	status := ChannelIQStatusUnconfigured
	if channel.Status != StatusActive {
		status = ChannelIQStatusDisabled
	} else if cfg.Enabled {
		status = ChannelIQStatusPending
	}
	return ChannelIQStatus{
		ChannelID:       channel.ID,
		ChannelName:     channel.Name,
		Model:           cfg.Model,
		Status:          status,
		IntervalSeconds: cfg.IntervalSeconds,
	}
}

func (s *ChannelIQService) cached(channelID int64) (channelIQCachedResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[channelID]
	return result, ok
}

func (s *ChannelIQService) nextCheck(channelID int64, cfg ChannelIQConfig) *time.Time {
	result, ok := s.cached(channelID)
	if !ok {
		return nil
	}
	next := result.when.Add(cfg.interval()).UTC()
	return &next
}

func (s *ChannelIQService) isInFlight(channelID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inFlight[channelID]
}

func (s *ChannelIQService) trigger(channel Channel) {
	s.mu.Lock()
	if s.inFlight[channel.ID] {
		s.mu.Unlock()
		return
	}
	s.inFlight[channel.ID] = true
	s.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), channelIQDefaultTimeout)
		defer cancel()
		s.execute(ctx, channel)
	}()
}

func (s *ChannelIQService) runChannel(ctx context.Context, channel Channel) ChannelIQStatus {
	s.mu.Lock()
	if s.inFlight[channel.ID] {
		if cached, ok := s.results[channel.ID]; ok {
			s.mu.Unlock()
			return cached.status
		}
		result := s.baseStatus(channel, ParseChannelIQConfig(channel.FeaturesConfig))
		s.mu.Unlock()
		return result
	}
	s.inFlight[channel.ID] = true
	s.mu.Unlock()
	return s.execute(ctx, channel)
}

func (s *ChannelIQService) execute(ctx context.Context, channel Channel) ChannelIQStatus {
	defer func() {
		s.mu.Lock()
		delete(s.inFlight, channel.ID)
		s.mu.Unlock()
	}()

	cfg := ParseChannelIQConfig(channel.FeaturesConfig)
	status := s.baseStatus(channel, cfg)
	if channel.Status != StatusActive || !cfg.Enabled {
		return status
	}
	if s.accountTest == nil || s.accountRepo == nil || s.groupRepo == nil {
		status.Status = ChannelIQStatusError
		status.Error = "渠道智商探测服务未就绪"
		return s.store(status)
	}
	s.mu.Lock()
	if s.probeSem == nil {
		s.probeSem = make(chan struct{}, 2)
	}
	probeSem := s.probeSem
	s.mu.Unlock()
	select {
	case probeSem <- struct{}{}:
		defer func() { <-probeSem }()
	case <-ctx.Done():
		status.Status = ChannelIQStatusError
		status.Error = "渠道智商探测排队超时"
		return s.store(status)
	}

	account, group, err := s.selectAccount(ctx, channel)
	if err != nil {
		status.Status = ChannelIQStatusError
		status.Error = err.Error()
		return s.store(status)
	}
	status.GroupID = group.ID
	status.GroupName = group.Name
	status.Platform = account.Platform
	if status.Model == "" {
		status.Model = "自动选择"
	}

	result, err := s.accountTest.RunPromptTestBackground(ctx, account.ID, cfg.Model, cfg.prompt())
	if err != nil {
		status.Status = ChannelIQStatusError
		status.Error = err.Error()
		return s.store(status)
	}
	status.LatencyMs = result.LatencyMs
	status.ResponsePreview = truncateChannelIQText(result.ResponseText)
	if result.ErrorMessage != "" || result.Status == "failed" {
		status.Status = ChannelIQStatusError
		status.Error = result.ErrorMessage
	} else if strings.TrimSpace(result.ResponseText) == "" {
		status.Status = ChannelIQStatusWarning
		status.Error = "上游返回成功，但没有可展示的文本"
	} else {
		status.Status = ChannelIQStatusHealthy
	}
	return s.store(status)
}

func (s *ChannelIQService) store(status ChannelIQStatus) ChannelIQStatus {
	now := time.Now().UTC()
	status.CheckedAt = &now
	s.mu.Lock()
	s.results[status.ChannelID] = channelIQCachedResult{status: status, when: now}
	s.mu.Unlock()
	return status
}

func (s *ChannelIQService) selectAccount(ctx context.Context, channel Channel) (*Account, *Group, error) {
	for _, groupID := range channel.GroupIDs {
		group, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil || group == nil || !group.IsActive() {
			continue
		}
		var accounts []Account
		if group.Platform == PlatformComposite {
			accounts, err = s.accountRepo.ListSchedulableByGroupID(ctx, group.ID)
		} else {
			accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, group.ID, group.Platform)
		}
		if err != nil {
			continue
		}
		for i := range accounts {
			if accounts[i].IsActive() {
				return &accounts[i], group, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("渠道没有可用的可调度账号")
}

func truncateChannelIQText(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= channelIQMaxPreviewRunes {
		return value
	}
	return string(runes[:channelIQMaxPreviewRunes]) + "…"
}
