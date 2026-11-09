package service

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAISelectionCoolingPoolProvidesRetryAfter(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc, accounts := strictPriorityTestService()
	svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
	now := time.Now()
	for _, account := range accounts {
		svc.recordOpenAIAccountModelTransientFailure(&account, "gpt-6.1-sol", now)
		svc.recordOpenAIAccountModelTransientFailure(&account, "gpt-6.1-sol", now)
	}
	result, _, err := strictPrioritySelect(svc, nil)
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	var retry interface{ RetryAfter() time.Duration }
	require.True(t, errors.As(err, &retry), "a transient-only empty pool must expose a bounded retry hint")
	require.Greater(t, retry.RetryAfter(), time.Duration(0))
	require.LessOrEqual(t, retry.RetryAfter(), openAIModelTransientShortCooldown)
}

func TestOpenAISelectionRetryAfterUsesEarliestDeadlineAndClearsOnRecovery(t *testing.T) {
	svc, accounts := strictPriorityTestService()
	now := time.Now()
	for i, account := range accounts {
		for n := 0; n < 2+i; n++ {
			svc.recordOpenAIAccountModelTransientFailure(&account, "gpt-6.1-sol", now)
		}
	}
	stats := openAISelectionFilterStats{pool: len(accounts), reasons: map[string]int{"runtime_blocked": len(accounts)}}
	until := svc.openAISelectionModelCooldownRetryAt(accounts, "gpt-6.1-sol", stats)
	require.WithinDuration(t, now.Add(openAIModelTransientShortCooldown), until, time.Millisecond)
	svc.clearOpenAIAccountModelTransientState(accounts[0].ID, "gpt-6.1-sol")
	require.True(t, svc.openAISelectionModelCooldownRetryAt(accounts, "gpt-6.1-sol", stats).IsZero())
	require.True(t, svc.openAISelectionModelCooldownRetryAt(accounts, "another-model", stats).IsZero())
}

func TestOpenAISelectionRetryAfterNeverGuessesForOtherExclusions(t *testing.T) {
	svc, accounts := strictPriorityTestService()
	for _, account := range accounts {
		for n := 0; n < 2; n++ {
			svc.recordOpenAIAccountModelTransientFailure(&account, "gpt-6.1-sol", time.Now())
		}
	}
	for _, reasons := range []map[string]int{
		nil, {"model_not_supported": 3}, {"runtime_blocked": 2, "excluded": 1},
		{"runtime_blocked": 2, "privacy_not_set": 1}, {"runtime_blocked": 2},
	} {
		require.True(t, svc.openAISelectionModelCooldownRetryAt(accounts, "gpt-6.1-sol", openAISelectionFilterStats{reasons: reasons}).IsZero())
	}
	stats := openAISelectionFilterStats{pool: 3, reasons: map[string]int{"runtime_blocked": 3}}
	until := time.Now().Add(time.Hour)
	accounts[0].RateLimitResetAt = &until
	require.True(t, svc.openAISelectionModelCooldownRetryAt(accounts, "gpt-6.1-sol", stats).IsZero())
	accounts[0].RateLimitResetAt = nil
	accounts[0].TempUnschedulableUntil = &until
	require.True(t, svc.openAISelectionModelCooldownRetryAt(accounts, "gpt-6.1-sol", stats).IsZero())
}
