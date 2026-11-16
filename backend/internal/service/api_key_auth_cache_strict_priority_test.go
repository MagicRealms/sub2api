package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotStrictPriorityRoundtrip(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformComposite} {
		for _, setting := range []string{"legacy", "enabled", "disabled"} {
			t.Run(platform+"/"+setting, func(t *testing.T) {
				key := profitAuthTestAPIKey()
				key.Group.Platform = platform
				key.Group.ProfitControlEnabled = false
				if setting != "legacy" {
					enabled := setting == "enabled"
					key.Group.StrictPriorityEnabled = &enabled
				}
				svc := &APIKeyService{}
				payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
				require.NoError(t, err)
				var restored APIKeyAuthCacheEntry
				require.NoError(t, json.Unmarshal(payload, &restored))
				materialized, used, err := svc.applyAuthCacheEntry(key.Key, &restored)
				require.NoError(t, err)
				require.True(t, used)
				require.NotNil(t, materialized.Group)
				require.Equal(t, key.Group.StrictPriorityEnabled, materialized.Group.StrictPriorityEnabled)
				ctx := context.WithValue(context.Background(), ctxkey.Group, materialized.Group)
				gateway, _ := strictPriorityTestService()
				// The group is outside the old global opt-in list, matching the
				// composite subscription group that exposed the cache omission.
				gateway.cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs = []int64{6, 10}
				ctx = gateway.withOpenAIGroupPrivacyRequirement(ctx, materialized.GroupID)
				require.Equal(t, setting == "enabled", gateway.useOpenAIStrictPriority(ctx, materialized.GroupID, PlatformOpenAI, "", true))
				gateway.cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs = []int64{*materialized.GroupID}
				require.Equal(t, setting != "disabled", gateway.useOpenAIStrictPriority(ctx, materialized.GroupID, PlatformOpenAI, "", true))
			})
		}
	}
}

func TestAPIKeyAuthSnapshotStrictPrioritySelectsPrimaryAfterCacheHit(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	gateway, accounts := strictPriorityTestService()
	gateway.cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs = []int64{6, 10}
	gateway.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("false")
	gateway.cache = &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:backup": accounts[2].ID}}
	key := profitAuthTestAPIKey()
	groupID := strictPriorityTestGroup
	key.GroupID = &groupID
	key.Group.ID = groupID
	key.Group.Platform = PlatformComposite
	key.Group.ProfitControlEnabled = false
	enabled := true
	key.Group.StrictPriorityEnabled = &enabled
	auth := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: auth.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))
	materialized, used, err := auth.applyAuthCacheEntry(key.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	ctx := context.WithValue(context.Background(), ctxkey.Group, materialized.Group)
	result, decision, err := gateway.SelectAccountWithScheduler(ctx, &groupID, "", "backup", "gpt-6.1-sol", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, accounts[0].ID, result.Account.ID, "a healthy primary must win over backup stickiness after an auth cache hit")
	require.Equal(t, openAIAccountScheduleLayerStrictPriority, decision.Layer)
	if result.ReleaseFunc != nil {
		result.ReleaseFunc()
	}
}

func TestAPIKeyAuthSnapshotV28RejectedWithoutStrictPriority(t *testing.T) {
	service := &APIKeyService{}
	materialized, used, err := service.applyAuthCacheEntry("sk-stale-priority", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 28},
	})
	require.NoError(t, err)
	require.False(t, used, "old cache entries must be rebuilt instead of silently dropping the group switch")
	require.Nil(t, materialized)
}
