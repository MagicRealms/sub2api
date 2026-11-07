//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type orderedGroupRepoTest struct {
	GroupRepository
	groups map[int64]*Group
}

func (r orderedGroupRepoTest) GetByID(_ context.Context, id int64) (*Group, error) {
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, ErrGroupNotFound
}

type orderedModelsRepoTest struct {
	AccountRepository
	accounts []Account
	err      error
}

func (r orderedModelsRepoTest) ListByGroup(context.Context, int64) ([]Account, error) {
	return r.accounts, r.err
}

func TestOrderedKeyGroupRoutesNormalization(t *testing.T) {
	one := int64(1)
	for _, tc := range []struct {
		name    string
		legacy  *int64
		ids     []int64
		want    []int64
		invalid bool
	}{
		{"legacy", &one, nil, []int64{1}, false},
		{"selection_order", nil, []int64{8, 2, 7}, []int64{8, 2, 7}, false},
		{"clear", nil, []int64{}, []int64{}, false},
		{"duplicate", nil, []int64{1, 1}, nil, true},
		{"negative", nil, []int64{-1}, nil, true},
		{"zero", nil, []int64{0}, nil, true},
		{"conflicting_legacy", &one, []int64{2, 1}, nil, true},
		{"too_many", nil, make([]int64, 17), nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := normalizeAPIKeyGroupRoutes(tc.legacy, tc.ids)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, actual)
		})
	}
}

func TestOrderedKeyRouteAuthorizationAndRequestIsolation(t *testing.T) {
	first := &Group{ID: 1, Status: StatusActive, RateMultiplier: .32}
	second := &Group{ID: 2, Status: StatusActive, RateMultiplier: .13}
	private := &Group{ID: 3, Status: StatusActive, IsExclusive: true}
	disabled := &Group{ID: 4, Status: StatusDisabled}
	svc := &APIKeyService{groupRepo: orderedGroupRepoTest{groups: map[int64]*Group{1: first, 2: second, 3: private, 4: disabled}}}
	key := &APIKey{ID: 10, UserID: 9, GroupID: &first.ID, Group: first, GroupIDs: []int64{1, 3, 4, 2, 99}, User: &User{ID: 9}}
	groups, err := svc.OrderedRouteGroups(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, []*Group{first, second}, groups)
	require.ErrorIs(t, svc.validateAPIKeyGroupRoutes(context.Background(), key.User, []int64{1, 3}), ErrGroupNotAllowed)
	ctx := WithAPIKeyGroupRoute(context.Background(), key.ID, second, first)
	routed, err := svc.ResolveAPIKeyGroupRoute(ctx, key)
	require.NoError(t, err)
	require.Equal(t, second, routed.Group)
	require.Equal(t, first, routed.RoutingBillingFloor)
	require.Equal(t, first, key.Group)
	require.Equal(t, int64(1), *key.GroupID)
	_, err = svc.ResolveAPIKeyGroupRoute(WithAPIKeyGroupRoute(context.Background(), 11, second, first), key)
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	require.False(t, APIKeyGroupRouteBilled(ctx))
	MarkAPIKeyGroupRouteBilled(ctx)
	require.True(t, APIKeyGroupRouteBilled(ctx))
	require.False(t, APIKeyGroupRouteBilled(context.Background()))
}

func TestOrderedKeyBillingNeverDiscountsHigherCostRoute(t *testing.T) {
	for _, rates := range [][2]float64{{.32, .13}, {.13, .32}, {.32, .32}} {
		first := &Group{ID: 1, RateMultiplier: rates[0]}
		actual := &Group{ID: 2, RateMultiplier: rates[1]}
		key := &APIKey{UserID: 9, GroupID: &actual.ID, Group: actual, RoutingBillingFloor: first}
		svc := &OpenAIGatewayService{billingService: NewBillingService(&config.Config{}, nil)}
		cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), &OpenAIForwardResult{WebSearchCalls: 1}, key, []string{"gpt-6.1-sol"}, rates[1], rates[1], rates[1], rates[1], UsageTokens{}, "", nil, false, time.Now())
		require.NoError(t, err)
		require.InDelta(t, .01*.32, cost.ActualCost, 1e-12)
	}
}

func TestOrderedKeyBillingComparesCustomPricesAndImageRates(t *testing.T) {
	firstPrice, actualPrice := .50, .10
	first := &Group{ID: 1, RateMultiplier: .10, ImageRateIndependent: true, ImageRateMultiplier: .8, ImagePrice1K: &firstPrice}
	actual := &Group{ID: 2, RateMultiplier: .50, ImageRateIndependent: true, ImageRateMultiplier: .2, ImagePrice1K: &actualPrice}
	key := &APIKey{UserID: 9, GroupID: &actual.ID, Group: actual, RoutingBillingFloor: first}
	svc := &GatewayService{billingService: NewBillingService(&config.Config{}, nil)}
	result := &ForwardResult{ImageCount: 1, ImageSize: "1K"}
	cost := svc.calculateRecordUsageCost(context.Background(), result, key, "image-test", .5, .2, time.Now())
	require.InDelta(t, .4, cost.ActualCost, 1e-12)
	require.InDelta(t, .10, orderedRouteAccountTotal(cost), 1e-12)
	// A model available only in the image group has no unrelated token-group floor.
	key.RoutingBillingFloor = nil
	cost = svc.calculateRecordUsageCost(context.Background(), result, key, "image-test", .5, .2, time.Now())
	require.InDelta(t, .02, cost.ActualCost, 1e-12)
}

func TestConfiguredGroupModelsRetainUnavailablePremiumBaseline(t *testing.T) {
	svc := &GatewayService{accountRepo: orderedModelsRepoTest{accounts: []Account{{Platform: PlatformOpenAI, Status: StatusDisabled, Credentials: map[string]any{"model_mapping": map[string]any{"custom-premium": "custom-premium"}}}}}}
	models, err := svc.ConfiguredGroupRouteModels(context.Background(), &Group{ID: 1, Platform: PlatformOpenAI}, nil)
	require.NoError(t, err)
	require.Contains(t, models, "custom-premium")
	svc.accountRepo = orderedModelsRepoTest{err: errors.New("database unavailable")}
	_, err = svc.ConfiguredGroupRouteModels(context.Background(), &Group{ID: 1}, nil)
	require.Error(t, err, "a catalog outage must never lower the billing baseline")
}

type orderedUserRateRepoTest struct {
	UserGroupRateRepository
	rates map[int64]float64
}

func (r orderedUserRateRepoTest) GetByUserAndGroup(_ context.Context, _, groupID int64) (*float64, error) {
	rate, exists := r.rates[groupID]
	if !exists {
		return nil, nil
	}
	return &rate, nil
}

func TestOrderedKeyTokenBillingHonorsUserOverridesAndPeakFloor(t *testing.T) {
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, timezone.Location())
	first := &Group{ID: 1, RateMultiplier: .05, SubscriptionType: SubscriptionTypeSubscription,
		PeakRateEnabled: true, PeakStart: "11:00", PeakEnd: "13:00", PeakRateMultiplier: 2}
	actual := &Group{ID: 2, RateMultiplier: .5}
	key := &APIKey{UserID: 9, GroupID: &actual.ID, Group: actual, RoutingBillingFloor: first}
	svc := &OpenAIGatewayService{billingService: NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"ordered-token-model": {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6},
	}}), userGroupRateResolver: newUserGroupRateResolver(orderedUserRateRepoTest{rates: map[int64]float64{1: .4, 2: .2}}, nil, time.Minute, nil, "test")}
	cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), &OpenAIForwardResult{}, key, []string{"ordered-token-model"}, .2, .2, .2, .2, UsageTokens{InputTokens: 1000, OutputTokens: 100}, "", nil, false, at)
	require.NoError(t, err)
	require.InDelta(t, .0012*.8, cost.ActualCost, 1e-12)
	require.NotNil(t, cost.RoutingRateMultiplier)
	require.InDelta(t, .8, *cost.RoutingRateMultiplier, 1e-12)
}

func TestOrderedKeyAuthSnapshotPreservesRouteOrderAndIsolation(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, UserID: 9, GroupIDs: []int64{6, 20, 16}, User: &User{ID: 9}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	require.Equal(t, []int64{6, 20, 16}, snapshot.GroupIDs)
	key.GroupIDs[0] = 100
	routed := svc.snapshotToAPIKey("test-key", snapshot)
	require.Equal(t, []int64{6, 20, 16}, routed.GroupIDs)
	routed.GroupIDs[1] = 200
	require.Equal(t, []int64{6, 20, 16}, snapshot.GroupIDs)
}

func TestOrderedKeyAccountQuotaUsesServingCost(t *testing.T) {
	actual := &CostBreakdown{TotalCost: .10, ActualCost: .02}
	floor := &CostBreakdown{TotalCost: .50, ActualCost: .40}
	cost := higherOrderedRouteCost(actual, floor, .8, .8)
	p := &postUsageBillingParams{
		Cost: cost, User: &User{ID: 1}, APIKey: &APIKey{ID: 2, Quota: 100},
		Account:               &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}},
		AccountRateMultiplier: 2,
	}
	cmd := buildUsageBillingCommand("ordered-quota", nil, p)
	require.InDelta(t, .40, cmd.BalanceCost, 1e-12)
	require.InDelta(t, .20, cmd.AccountQuotaCost, 1e-12, "customer floor must not inflate upstream account consumption")
}

type orderedBatchGroupRepoTest struct {
	orderedGroupRepoTest
	calls *int
}

func (r orderedBatchGroupRepoTest) GetByIDsLite(_ context.Context, ids []int64) ([]*Group, error) {
	*r.calls++
	// Deliberately unordered: the key, not DB order, defines priority.
	return []*Group{r.groups[2], r.groups[1]}, nil
}
func TestOrderedKeyRouteBatchPoliciesPreservePriority(t *testing.T) {
	calls := 0
	first := &Group{ID: 1, Status: StatusActive}
	second := &Group{ID: 2, Status: StatusActive}
	svc := &APIKeyService{groupRepo: orderedBatchGroupRepoTest{orderedGroupRepoTest{groups: map[int64]*Group{1: first, 2: second}}, &calls}}
	groups, err := svc.OrderedRouteGroups(context.Background(), &APIKey{GroupIDs: []int64{1, 2}, User: &User{ID: 9}})
	require.NoError(t, err)
	require.Equal(t, []*Group{first, second}, groups)
	require.Equal(t, 1, calls)
}

type orderedConfiguredModelsRepoTest struct{ orderedModelsRepoTest }

func (r orderedConfiguredModelsRepoTest) ListConfiguredGroupRouteAccounts(context.Context, int64) ([]Account, error) {
	return []Account{{Platform: PlatformOpenAI, Status: StatusDisabled, Credentials: map[string]any{"model_mapping": map[string]any{"disabled-premium-alias": "premium"}}}}, nil
}
func TestOrderedKeyCatalogUsesConfigurationRatherThanActiveAccounts(t *testing.T) {
	svc := &GatewayService{accountRepo: orderedConfiguredModelsRepoTest{}}
	models, err := svc.ConfiguredGroupRouteModels(context.Background(), &Group{ID: 1, Platform: PlatformOpenAI}, []string{"fallback-model"})
	require.NoError(t, err)
	require.Contains(t, models, "disabled-premium-alias")
	require.NotContains(t, models, "fallback-model")
}

func TestOrderedKeyPricingGuardRejectsUnknownButAllowsExplicitFree(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	svc := &GatewayService{billingService: billing, resolver: NewModelPricingResolver(nil, billing)}
	group := &Group{ID: 1}
	require.ErrorIs(t, svc.ValidateOrderedGroupModelPricing(context.Background(), group, "unpriced-custom-alias", "/v1/responses"), ErrModelPricingUnavailable)
	zero := 0.0
	group.ModelPricing = []ChannelModelPricing{{Models: []string{"unpriced-custom-alias"}, BillingMode: BillingModeToken, InputPrice: &zero, OutputPrice: &zero}}
	require.NoError(t, svc.ValidateOrderedGroupModelPricing(context.Background(), group, "unpriced-custom-alias", "/v1/responses"))
	require.NoError(t, svc.ValidateOrderedGroupModelPricing(context.Background(), group, "gpt-image-2.5", "/v1/images/generations"))
}
