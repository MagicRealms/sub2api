package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const strictPriorityTestGroup int64 = 101200

type strictPriorityAtomicSlots struct {
	schedulerTestConcurrencyCache
	mu     sync.Mutex
	active map[int64]int
	peak   map[int64]int
}

func (c *strictPriorityAtomicSlots) AcquireAccountSlot(_ context.Context, id int64, limit int, _ string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active[id] >= limit {
		return false, nil
	}
	c.active[id]++
	if c.active[id] > c.peak[id] {
		c.peak[id] = c.active[id]
	}
	return true, nil
}

func (c *strictPriorityAtomicSlots) ReleaseAccountSlot(_ context.Context, id int64, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active[id]--
	return nil
}

func strictPriorityTestService() (*OpenAIGatewayService, []Account) {
	accounts := make([]Account, 3)
	for i := range accounts {
		accounts[i] = Account{
			ID: int64(90101 + i), Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true, Concurrency: 200, Priority: i + 1,
			GroupIDs: []int64{strictPriorityTestGroup},
		}
	}
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs = []int64{strictPriorityTestGroup}
	cfg.Gateway.OpenAIWS.LBTopK = 3
	cfg.Gateway.Scheduling.FallbackWaitTimeout = 3 * time.Second
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 100
	return &OpenAIGatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}, accounts
}

func strictPrioritySelect(svc *OpenAIGatewayService, excluded map[int64]struct{}) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	groupID := strictPriorityTestGroup
	return svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "", "gpt-6.1-sol", excluded, OpenAIUpstreamTransportAny, false)
}

func TestOpenAIStrictPriority_HealthyPrimaryTwentyConcurrentRequests(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc, accounts := strictPriorityTestService()
	accounts[0].Concurrency = 20
	slots := &strictPriorityAtomicSlots{active: map[int64]int{}, peak: map[int64]int{}}
	svc.concurrencyService = NewConcurrencyService(slots)
	// Explicit opt-in remains strict even with the global advanced switch off.
	svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("false")
	svc.ensureOpenAIAccountScheduler()
	var wg sync.WaitGroup
	results := make(chan *AccountSelectionResult, 20)
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, _, err := strictPrioritySelect(svc, nil)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	for result := range results {
		require.NotNil(t, result)
		require.Equal(t, accounts[0].ID, result.Account.ID)
		require.True(t, result.Acquired)
		if result.ReleaseFunc != nil {
			result.ReleaseFunc()
		}
	}
	require.Equal(t, 20, slots.peak[accounts[0].ID])
	require.Zero(t, slots.peak[accounts[1].ID])
	require.Zero(t, slots.peak[accounts[2].ID])
	require.Zero(t, slots.active[accounts[0].ID])
}

func TestOpenAIStrictPriority_OrderedFailoverAndRecovery(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc, accounts := strictPriorityTestService()
	svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true", "true")
	failed := map[int64]struct{}{}
	for _, expected := range accounts {
		result, decision, err := strictPrioritySelect(svc, failed)
		require.NoError(t, err)
		require.Equal(t, expected.ID, result.Account.ID)
		require.Equal(t, openAIAccountScheduleLayerStrictPriority, decision.Layer)
		result.ReleaseFunc()
		failed[expected.ID] = struct{}{}
	}
	result, _, err := strictPrioritySelect(svc, failed)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, result)
	result, _, err = strictPrioritySelect(svc, nil)
	require.NoError(t, err)
	require.Equal(t, accounts[0].ID, result.Account.ID)
	result.ReleaseFunc()
}

func TestOpenAIStrictPriority_FullPrimaryQueuesWithoutSpilling(t *testing.T) {
	for _, knownFull := range []bool{false, true} {
		t.Run(map[bool]string{false: "slot_race", true: "known_full"}[knownFull], func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc, accounts := strictPriorityTestService()
			var acquired []int64
			cache := schedulerTestConcurrencyCache{
				acquireResults: map[int64]bool{accounts[0].ID: false}, acquiredIDs: &acquired,
			}
			if knownFull {
				cache.loadMap = map[int64]*AccountLoadInfo{accounts[0].ID: {AccountID: accounts[0].ID, CurrentConcurrency: 200, LoadRate: 100}}
			}
			svc.concurrencyService = NewConcurrencyService(cache)
			svc.cache = &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:backup": accounts[2].ID}}
			groupID := strictPriorityTestGroup
			result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "backup", "gpt-6.1-sol", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			require.False(t, result.Acquired)
			require.Equal(t, accounts[0].ID, result.Account.ID)
			require.Equal(t, accounts[0].ID, result.WaitPlan.AccountID)
			require.Positive(t, result.WaitPlan.Timeout)
			require.Positive(t, result.WaitPlan.MaxWaiting)
			for _, id := range acquired {
				require.Equal(t, accounts[0].ID, id)
			}
		})
	}
}

func TestOpenAIStrictPriority_LoadHealthAndBackupStickyCannotOverridePriority(t *testing.T) {
	for _, stickyWeighted := range []string{"false", "true"} {
		t.Run(stickyWeighted, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc, accounts := strictPriorityTestService()
			svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true", stickyWeighted)
			svc.cache = &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:backup": accounts[2].ID}}
			svc.cfg.Gateway.OpenAIWS.SchedulerScoreWeights = config.GatewayOpenAIWSSchedulerScoreWeights{Load: 100, TTFT: 100, ErrorRate: 100, SessionSticky: 100}
			svc.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{accounts[0].ID: {AccountID: accounts[0].ID, CurrentConcurrency: 199, LoadRate: 99}}})
			scheduler := svc.ensureOpenAIAccountScheduler().(*defaultOpenAIAccountScheduler)
			slow := 120000
			scheduler.ReportResult(accounts[0].ID, false, &slow)
			groupID := strictPriorityTestGroup
			result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "backup", "gpt-6.1-sol", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			require.Equal(t, accounts[0].ID, result.Account.ID)
			result.ReleaseFunc()
		})
	}
}

func TestOpenAIStrictPriority_EligibilityBeforePriority(t *testing.T) {
	for _, reason := range []string{"disabled", "cooldown", "model", "capability", "foreign_group"} {
		t.Run(reason, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc, accounts := strictPriorityTestService()
			switch reason {
			case "disabled":
				accounts[0].Schedulable = false
			case "cooldown":
				until := time.Now().Add(time.Hour)
				accounts[0].RateLimitResetAt = &until
			case "model":
				accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"gpt-other": "gpt-other"}}
			case "capability":
				accounts[0].Credentials = map[string]any{"openai_capabilities": []any{"embeddings"}}
			case "foreign_group":
				accounts[0].GroupIDs = []int64{strictPriorityTestGroup + 1}
			}
			svc.accountRepo = schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}}
			groupID := strictPriorityTestGroup
			result, _, err := svc.SelectAccountWithSchedulerForCapability(context.Background(), &groupID, "", "", "gpt-6.1-sol", nil, OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityResponses, false, false, true)
			require.NoError(t, err)
			require.Equal(t, accounts[1].ID, result.Account.ID)
			result.ReleaseFunc()
		})
	}
}

func TestOpenAIStrictPriority_HardPreviousResponseOwnerPreserved(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc, accounts := strictPriorityTestService()
	groupID := strictPriorityTestGroup
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(context.Background(), groupID, "resp_strict_owner", accounts[2].ID, time.Hour))
	result, decision, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "resp_strict_owner", "", "gpt-6.1-sol", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, accounts[2].ID, result.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
	result.ReleaseFunc()
}

func TestOpenAIStrictPriority_SafeMovableResponseReturnsToPrimary(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc, accounts := strictPriorityTestService()
	svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true", "false")
	groupID := strictPriorityTestGroup
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(context.Background(), groupID, "resp_strict_movable", accounts[2].ID, time.Hour))
	result, decision, err := svc.SelectAccountWithSchedulerForCapability(context.Background(), &groupID, "resp_strict_movable", "", "gpt-6.1-sol", nil, OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityResponses, false, true, true)
	require.NoError(t, err)
	require.Equal(t, accounts[0].ID, result.Account.ID)
	require.True(t, decision.PreferOrderedFailover())
	require.False(t, decision.StickyPreviousHit)
	result.ReleaseFunc()
}

func TestOpenAIStrictPriority_HardResponseNeverEscapesUnavailableOrBusyOwner(t *testing.T) {
	for _, state := range []string{"full", "excluded", "disabled"} {
		t.Run(state, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc, accounts := strictPriorityTestService()
			groupID := strictPriorityTestGroup
			require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(context.Background(), groupID, "resp_strict_owned", accounts[2].ID, time.Hour))
			excluded := map[int64]struct{}{}
			if state == "excluded" {
				excluded[accounts[2].ID] = struct{}{}
			}
			if state == "disabled" {
				accounts[2].Schedulable = false
			}
			if state == "full" {
				svc.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{accounts[2].ID: false}})
			}
			result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "resp_strict_owned", "", "gpt-6.1-sol", excluded, OpenAIUpstreamTransportAny, false)
			if state == "full" {
				require.NoError(t, err)
				require.Equal(t, accounts[2].ID, result.Account.ID)
				require.NotNil(t, result.WaitPlan)
			} else {
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				require.Nil(t, result)
			}
		})
	}
}

func TestOpenAIStrictPriority_GuardianOwnerPreserved(t *testing.T) {
	svc, accounts := strictPriorityTestService()
	svc.cache = &schedulerTestGatewayCache{}
	groupID := strictPriorityTestGroup
	result, decision, err := svc.ensureOpenAIAccountScheduler().Select(context.Background(), OpenAIAccountScheduleRequest{
		GroupID: &groupID, Platform: PlatformOpenAI, SessionHash: "guardian-child", GuardianParentAccountID: accounts[2].ID,
		StrictPriority: true, RequestedModel: "gpt-6.1-sol", RequiredTransport: OpenAIUpstreamTransportAny,
	})
	require.NoError(t, err)
	require.Equal(t, accounts[2].ID, result.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerGuardianParent, decision.Layer)
	require.False(t, decision.PreferOrderedFailover())
	result.ReleaseFunc()
}

func TestOpenAIStrictPriority_EqualPriorityMayUseFreeSibling(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc, accounts := strictPriorityTestService()
	accounts[1].Priority = accounts[0].Priority
	svc.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{accounts[0].ID: false}})
	result, _, err := strictPrioritySelect(svc, nil)
	require.NoError(t, err)
	require.Equal(t, accounts[1].ID, result.Account.ID)
	result.ReleaseFunc()
}

func TestOpenAIStrictPriority_CanceledRequestDoesNotAcquire(t *testing.T) {
	svc, _ := strictPriorityTestService()
	var acquired []int64
	svc.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquired})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scheduler := svc.ensureOpenAIAccountScheduler()
	_, _, err := scheduler.Select(ctx, OpenAIAccountScheduleRequest{GroupID: func() *int64 { v := strictPriorityTestGroup; return &v }(), Platform: PlatformOpenAI, StrictPriority: true, RequestedModel: "gpt-6.1-sol"})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, acquired)
}

func TestOpenAIStrictPriority_OptInScope(t *testing.T) {
	svc, _ := strictPriorityTestService()
	groupID, other := strictPriorityTestGroup, strictPriorityTestGroup+1
	for _, tc := range []struct {
		name     string
		ctx      context.Context
		group    *int64
		platform string
		image    OpenAIImagesCapability
		token    bool
		want     bool
	}{
		{"opted_in", context.Background(), &groupID, PlatformOpenAI, "", true, true},
		{"other_group", context.Background(), &other, PlatformOpenAI, "", true, false},
		{"ungrouped", context.Background(), nil, PlatformOpenAI, "", true, false},
		{"other_provider", context.Background(), &groupID, PlatformGrok, "", true, false},
		{"image_intent", context.Background(), &groupID, PlatformOpenAI, "", false, false},
		{"media", WithOpenAIProfitControlSuppressed(context.Background()), &groupID, PlatformOpenAI, "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, svc.useOpenAIStrictPriority(tc.ctx, tc.group, tc.platform, tc.image, tc.token))
		})
	}
	svc.cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs = nil
	require.False(t, svc.useOpenAIStrictPriority(context.Background(), &groupID, PlatformOpenAI, "", true))
}

func TestOpenAIStrictPriority_StalePrimaryInvalidationAndPriorityChange(t *testing.T) {
	for _, changePriority := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled_in_db", true: "priority_changed"}[changePriority], func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc, accounts := strictPriorityTestService()
			stale := append([]Account(nil), accounts...)
			if changePriority {
				accounts[0].Priority = 4
			} else {
				accounts[0].Schedulable = false
			}
			svc.accountRepo = schedulerTestOpenAIAccountRepo{accounts: accounts}
			svc.schedulerSnapshot = &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{snapshotAccounts: []*Account{&stale[0], &stale[1], &stale[2]}, accountsByID: map[int64]*Account{stale[0].ID: &stale[0], stale[1].ID: &stale[1], stale[2].ID: &stale[2]}}}
			result, _, err := strictPrioritySelect(svc, nil)
			if changePriority {
				require.True(t, errors.Is(err, errOpenAIStrictPriorityChanged))
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, accounts[1].ID, result.Account.ID)
				result.ReleaseFunc()
			}
		})
	}
}

func TestOpenAIStrictPriority_StaleCompactSupportFallsThroughToCapableBackup(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc, accounts := strictPriorityTestService()
	stale := append([]Account(nil), accounts...)
	accounts[0].Extra = map[string]any{"openai_compact_supported": false}
	accounts[1].Extra = map[string]any{"openai_compact_supported": true}
	svc.accountRepo = schedulerTestOpenAIAccountRepo{accounts: accounts}
	svc.schedulerSnapshot = &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{snapshotAccounts: []*Account{&stale[0], &stale[1], &stale[2]}, accountsByID: map[int64]*Account{stale[0].ID: &stale[0], stale[1].ID: &stale[1], stale[2].ID: &stale[2]}}}
	groupID := strictPriorityTestGroup
	result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "", "gpt-6.1-sol", nil, OpenAIUpstreamTransportAny, true)
	require.NoError(t, err)
	require.Equal(t, accounts[1].ID, result.Account.ID)
	result.ReleaseFunc()
}
