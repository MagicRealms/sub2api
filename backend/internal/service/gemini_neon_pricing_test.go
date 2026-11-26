package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGeminiNeonPricing(t *testing.T) {
	const model = "gemini-4-neon"
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	catalog := &PricingService{}
	catalog.pricingData, err = catalog.parsePricingData(data)
	require.NoError(t, err)
	require.Contains(t, catalog.ListModelNamesByProvider("gemini"), model)

	for _, source := range []struct {
		name    string
		pricing *PricingService
	}{
		{name: "catalog", pricing: catalog},
		{name: "built-in fallback"},
	} {
		t.Run(source.name, func(t *testing.T) {
			billing := NewBillingService(&config.Config{}, source.pricing)
			require.True(t, billing.HasIdentifiedTokenPricing(model))
			require.Zero(t, billing.CacheStoragePricePerTokenHour(model))
			tokens := UsageTokens{InputTokens: 1000, OutputTokens: 1000, CacheReadTokens: 1000}
			for _, tier := range []struct {
				name                     string
				input, output, cacheRead float64
			}{
				{name: "standard", input: 0.003, output: 0.015, cacheRead: 0.00015},
				{name: "priority", input: 0.0054, output: 0.027, cacheRead: 0.00027},
				{name: "flex", input: 0.0015, output: 0.0075, cacheRead: 0.000075},
			} {
				t.Run(tier.name, func(t *testing.T) {
					cost, err := billing.CalculateCostWithServiceTier("models/"+model, tokens, 0.5, tier.name)
					require.NoError(t, err)
					require.InDelta(t, tier.input, cost.InputCost, 1e-12)
					require.InDelta(t, tier.output, cost.OutputCost, 1e-12)
					require.InDelta(t, tier.cacheRead, cost.CacheReadCost, 1e-12)
					require.InDelta(t, (tier.input+tier.output+tier.cacheRead)*0.5, cost.ActualCost, 1e-12)
				})
			}

			plaza := &ModelPlazaService{billingService: billing}
			official := plaza.lookupOfficialPricing(context.Background(), model, make(map[string]*PlazaOfficialPricing))
			require.NotNil(t, official)
			require.Equal(t, "USD", official.Currency)
			require.InDelta(t, 15e-6, *official.OutputPrice, 1e-12)
		})
	}
}
