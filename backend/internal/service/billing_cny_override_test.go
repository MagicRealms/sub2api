//go:build unit

package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekLocalOverrideWinsPerField(t *testing.T) {
	const catalog = `{"deepseek-flash":{"input_cost_per_token":9,"output_cost_per_token":8,"cache_read_input_token_cost":7}}`
	for _, tc := range []struct {
		name, patch                       string
		input, output, cacheRead          float64
		inputOverridden, outputOverridden bool
		cacheReadOverridden               bool
	}{
		{"input only", `{"input_cost_per_token":0.000001}`, 1e-6, 6e-7, 3e-9, true, false, false},
		{"output only", `{"output_cost_per_token":0.000004}`, 1.5e-7, 4e-6, 3e-9, false, true, false},
		{"cache only", `{"cache_read_input_token_cost":0.00000002}`, 1.5e-7, 6e-7, 2e-8, false, false, true},
		{"explicit zero", `{"input_cost_per_token":0,"output_cost_per_token":0,"cache_read_input_token_cost":0}`, 0, 0, 0, true, true, true},
		{"null is not a price", `{"input_cost_per_token":null}`, 1.5e-7, 6e-7, 3e-9, false, false, false},
		{"unrelated override", `{"max_input_tokens":1048576}`, 1.5e-7, 6e-7, 3e-9, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ps := newPricingServiceWithOverride(t, `{"deepseek-flash":`+tc.patch+`}`)
			data, _, err := ps.buildPricingData([]byte(catalog))
			require.NoError(t, err)
			ps.pricingData = data
			bs := NewBillingService(&config.Config{}, ps)
			pricing, err := bs.GetModelPricing("deepseek-flash")
			require.NoError(t, err)
			require.InDelta(t, tc.input, pricing.InputPricePerToken, 1e-15)
			require.InDelta(t, tc.output, pricing.OutputPricePerToken, 1e-15)
			require.InDelta(t, tc.cacheRead, pricing.CacheReadPricePerToken, 1e-15)
			resolver := NewModelPricingResolver(nil, bs)
			for _, period := range []struct {
				at         time.Time
				multiplier float64
			}{
				{time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC), 1},
				{time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC), 2},
			} {
				cost, err := bs.CalculateCostUnified(CostInput{
					Model: "deepseek-flash", Resolver: resolver, RateMultiplier: 0.2,
					Tokens:    UsageTokens{InputTokens: 1000000, OutputTokens: 1000000, CacheReadTokens: 1000000},
					PricingAt: period.at,
				})
				require.NoError(t, err)
				inputMultiplier, outputMultiplier, cacheReadMultiplier := period.multiplier, period.multiplier, period.multiplier
				if period.multiplier > 1 {
					if tc.inputOverridden {
						inputMultiplier = 1
					}
					if tc.outputOverridden {
						outputMultiplier = 1
					}
					if tc.cacheReadOverridden {
						cacheReadMultiplier = 1
					}
				}
				want := (tc.input*inputMultiplier + tc.output*outputMultiplier + tc.cacheRead*cacheReadMultiplier) * 1000000
				require.InDelta(t, want, cost.TotalCost, 1e-12)
				require.InDelta(t, want*0.2, cost.ActualCost, 1e-12)
			}
		})
	}
}

func TestCNYNominalPricingConfigBillsSameNumbersAndPopulatesPlaza(t *testing.T) {
	override, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "model-pricing-cny-nominal.json"))
	require.NoError(t, err)
	ps := newPricingServiceWithOverride(t, string(override))
	data, _, err := ps.buildPricingData([]byte(`{"gpt-5.4":{"input_cost_per_token":0.0000025,"output_cost_per_token":0.000015}}`))
	require.NoError(t, err)
	ps.pricingData = data
	bs := NewBillingService(&config.Config{}, ps)
	resolver := NewModelPricingResolver(nil, bs)
	plaza := &ModelPlazaService{billingService: bs, resolver: resolver}

	for _, tc := range []struct {
		model, platform                string
		input, output, cacheRead, rate float64
		write5m, write1h               float64
	}{
		{"deepseek-v4.1-flash-1017", PlatformDeepseek, 1, 4, 0.02, 0.2, 0, 0},
		{"deepseek-v4-pro", PlatformDeepseek, 4.5, 13.5, 0.15, 0.2, 0, 0},
		{"kimi-k3", PlatformKimi, 20, 100, 2, 0.3, 20, 40},
		{"kimi-k3.1-pro", PlatformKimi, 13, 68, 1.3, 0.3, 13, 26},
		{"kimi-k3.1-pro-highspeed", PlatformKimi, 26, 136, 1.3, 0.3, 26, 52},
		{"kimi-k2.8-code", PlatformKimi, 6.5, 27, 1.3, 0.3, 0, 0},
		{"glm-5.2", PlatformZhipu, 8, 28, 2, 0.35, 0, 0},
		{"glm-5.3", PlatformZhipu, 8, 28, 2, 0.35, 0, 0},
		{"glm-5.3-flash", PlatformZhipu, 0.8, 2.8, 0.23, 0.35, 0, 0},
	} {
		t.Run(tc.model, func(t *testing.T) {
			group := &Group{ID: 1, Platform: tc.platform, RateMultiplier: tc.rate, LongContextPricingEnabled: true}
			cost, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
				Ctx: context.Background(), Model: tc.model, Group: group,
				Resolver: resolver, RateMultiplier: tc.rate,
				PricingAt: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC),
				Tokens: UsageTokens{InputTokens: 1000000, OutputTokens: 1000000, CacheReadTokens: 1000000,
					CacheCreationTokens: 2000000, CacheCreation5mTokens: 1000000, CacheCreation1hTokens: 1000000},
			})
			require.NoError(t, err)
			require.InDelta(t, tc.input, cost.InputCost, 1e-9)
			require.InDelta(t, tc.output, cost.OutputCost, 1e-9)
			require.InDelta(t, tc.cacheRead, cost.CacheReadCost, 1e-9)
			require.InDelta(t, tc.write5m+tc.write1h, cost.CacheCreationCost, 1e-9)
			require.InDelta(t, (tc.input+tc.output+tc.cacheRead+tc.write5m+tc.write1h)*tc.rate, cost.ActualCost, 1e-9)
			official := plaza.lookupOfficialPricing(context.Background(), tc.model, make(map[string]*PlazaOfficialPricing))
			require.NotNil(t, official)
			require.Equal(t, "CNY", official.Currency, "人民币价卡的显示币种必须传递到广场")
			require.InDelta(t, tc.input, *official.InputPrice*1000000, 1e-9)
			require.InDelta(t, tc.output, *official.OutputPrice*1000000, 1e-9)
			require.InDelta(t, tc.cacheRead, *official.CacheReadPrice*1000000, 1e-9)
			modelPricing, err := bs.GetModelPricing(tc.model)
			require.NoError(t, err)
			if tc.platform == PlatformDeepseek {
				require.True(t, modelPricing.DeepSeekPeakPricingEnabled)
				peakCost, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
					Ctx: context.Background(), Model: tc.model, Group: group,
					Resolver: resolver, RateMultiplier: tc.rate,
					PricingAt: time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC),
					Tokens:    UsageTokens{InputTokens: 1000000, OutputTokens: 1000000, CacheReadTokens: 1000000},
				})
				require.NoError(t, err)
				require.InDelta(t, tc.input*2, peakCost.InputCost, 1e-9)
				require.InDelta(t, tc.output*2, peakCost.OutputCost, 1e-9)
				require.InDelta(t, tc.cacheRead*2, peakCost.CacheReadCost, 1e-9)
				require.InDelta(t, (tc.input+tc.output+tc.cacheRead)*2*tc.rate, peakCost.ActualCost, 1e-9)
			} else {
				require.False(t, modelPricing.DeepSeekPeakPricingEnabled)
			}
			if tc.write1h > 0 {
				require.InDelta(t, tc.write5m, *official.CacheWritePrice*1000000, 1e-9)
				require.InDelta(t, tc.write1h, *official.CacheWrite1hPrice*1000000, 1e-9)
			}
		})
	}
	// The configuration leaves other providers and their existing billing units intact.
	pricing, err := bs.GetModelPricing("gpt-5.4")
	require.NoError(t, err)
	require.InDelta(t, 2.5e-6, pricing.InputPricePerToken, 1e-15)
	usdOfficial := plaza.lookupOfficialPricing(context.Background(), "gpt-5.4", make(map[string]*PlazaOfficialPricing))
	require.Equal(t, "USD", usdOfficial.Currency, "没有人民币标记的模型继续展示美元")
}

func TestDeepSeekOverrideHotReloadAndRemoval(t *testing.T) {
	ps := newHotReloadPricingService(t, "", `{"deepseek-flash":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000004,"cache_read_input_token_cost":0.00000002}}`)
	bs := NewBillingService(&config.Config{}, ps)
	pricing, err := bs.GetModelPricing("deepseek-flash")
	require.NoError(t, err)
	require.InDelta(t, 1e-6, pricing.InputPricePerToken, 1e-15)
	require.NoError(t, os.WriteFile(ps.cfg.Pricing.OverrideFile, []byte(`{"deepseek-flash":{"input_cost_per_token":0.000002}}`), 0644))
	ps.reloadIfCustomFilesChanged()
	pricing, err = bs.GetModelPricing("deepseek-flash")
	require.NoError(t, err)
	require.InDelta(t, 2e-6, pricing.InputPricePerToken, 1e-15)
	require.InDelta(t, 6e-7, pricing.OutputPricePerToken, 1e-15)
	require.NoError(t, os.Remove(ps.cfg.Pricing.OverrideFile))
	ps.reloadIfCustomFilesChanged()
	pricing, err = bs.GetModelPricing("deepseek-flash")
	require.NoError(t, err)
	require.InDelta(t, 1.5e-7, pricing.InputPricePerToken, 1e-15)
}
