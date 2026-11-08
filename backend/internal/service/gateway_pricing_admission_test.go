//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGatewayPricingAdmission(t *testing.T) {
	group := &Group{ID: 10, Platform: PlatformOpenAI, RateMultiplier: .3}
	key := &APIKey{User: &User{ID: 7}, GroupID: &group.ID, Group: group}
	cfg := &config.Config{}
	billing := NewBillingService(cfg, nil)
	d := inflightEstimateDeps{cfg: cfg, billing: billing, resolver: NewModelPricingResolver(nil, billing)}
	req := InflightEstimateRequest{Model: "unpriced-audit-custom-model", Kind: InflightEstimateToken}
	t.Run("unknown single-group model rejected", func(t *testing.T) {
		require.ErrorIs(t, d.validateRequestPricing(context.Background(), key, req), ErrModelPricingUnavailable)
	})
	t.Run("free multiplier does not hide missing price", func(t *testing.T) {
		g := *group
		g.RateMultiplier = 0
		k := *key
		k.Group = &g
		require.ErrorIs(t, d.validateRequestPricing(context.Background(), &k, req), ErrModelPricingUnavailable)
	})
	t.Run("explicit free price accepted", func(t *testing.T) {
		g := *group
		zero := 0.0
		g.ModelPricing = []ChannelModelPricing{{Models: []string{req.Model}, BillingMode: BillingModeToken, InputPrice: &zero, OutputPrice: &zero}}
		k := *key
		k.Group = &g
		require.NoError(t, d.validateRequestPricing(context.Background(), &k, req))
	})
	t.Run("channel alias priced without account lookup", func(t *testing.T) {
		deps := d
		deps.resolveMapping = func(context.Context, int64, string) ChannelMappingResult {
			return ChannelMappingResult{MappedModel: "claude-sonnet-4-5"}
		}
		deps.accountMappedModels = func(context.Context, *APIKey, string) []string {
			t.Fatal("priced channel must not load account mappings")
			return nil
		}
		require.NoError(t, deps.validateRequestPricing(context.Background(), key, req))
	})
	t.Run("account alias uses existing fallback", func(t *testing.T) {
		deps := d
		calls := 0
		deps.accountMappedModels = func(context.Context, *APIKey, string) []string {
			calls++
			return []string{"claude-sonnet-4-5"}
		}
		require.NoError(t, deps.validateRequestPricing(context.Background(), key, req))
		require.Equal(t, 1, calls)
	})
	t.Run("missing billing service rejected", func(t *testing.T) {
		deps := d
		deps.billing = nil
		require.ErrorIs(t, deps.validateRequestPricing(context.Background(), key, req), ErrModelPricingUnavailable)
	})
	t.Run("priced alias cannot admit an unpriced alternative account", func(t *testing.T) {
		deps := d
		deps.accountMappedModels = func(context.Context, *APIKey, string) []string {
			return []string{"claude-sonnet-4-5", "unpriced-account-target"}
		}
		require.ErrorIs(t, deps.validateRequestPricing(context.Background(), key, req), ErrModelPricingUnavailable)
	})
	t.Run("alias fallback also checks unmapped eligible accounts", func(t *testing.T) {
		accounts := []Account{
			{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{req.Model: "claude-sonnet-4-5"}}},
			{Platform: PlatformOpenAI},
		}
		deps := d
		deps.accountMappedModels = func(context.Context, *APIKey, string) []string {
			return accountMappedModelsFrom(accounts, req.Model, true)
		}
		require.ErrorIs(t, deps.validateRequestPricing(context.Background(), key, req), ErrModelPricingUnavailable)
		accounts = accounts[:1]
		require.NoError(t, deps.validateRequestPricing(context.Background(), key, req))
	})
	t.Run("simple mode preserved", func(t *testing.T) {
		deps := d
		deps.cfg = &config.Config{RunMode: config.RunModeSimple}
		require.NoError(t, deps.validateRequestPricing(context.Background(), key, req))
	})
	t.Run("non-metered and separate media pricing preserved", func(t *testing.T) {
		require.NoError(t, d.validateRequestPricing(context.Background(), key, InflightEstimateRequest{}))
		require.NoError(t, d.validateRequestPricing(context.Background(), key, InflightEstimateRequest{Model: req.Model, Kind: InflightEstimateImage}))
	})
	t.Run("cancelled request preserved", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, d.validateRequestPricing(ctx, key, req), context.Canceled)
	})
	t.Run("both production gateways enforce admission", func(t *testing.T) {
		generic := &GatewayService{cfg: cfg, billingService: billing, resolver: d.resolver}
		openai := &OpenAIGatewayService{cfg: cfg, billingService: billing, resolver: d.resolver}
		require.ErrorIs(t, generic.ValidateRequestPricing(context.Background(), key, req), ErrModelPricingUnavailable)
		require.ErrorIs(t, openai.ValidateRequestPricing(context.Background(), key, req), ErrModelPricingUnavailable)
	})
}
