package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// Pricing admission is independent of the optional Redis balance reservation.
// A missing price must not turn a successful generation into legacy zero-cost
// usage. Explicitly configured free prices remain valid.
func (d inflightEstimateDeps) validateRequestPricing(ctx context.Context, key *APIKey, req InflightEstimateRequest) error {
	if req.Kind != InflightEstimateToken || strings.TrimSpace(req.Model) == "" || key == nil ||
		(d.cfg != nil && d.cfg.RunMode == config.RunModeSimple) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.billing == nil {
		return ErrModelPricingUnavailable
	}
	primary, fallbacks, upstream := inflightBillingModelCandidates(ctx, d, key, req.Model)
	priced := func(models []string) bool {
		for _, model := range models {
			_, err := d.billing.CalculateTokenCostForRequest(TokenCostRequest{
				Ctx: ctx, Group: key.Group, Model: model,
				Tokens: UsageTokens{InputTokens: 1, OutputTokens: 1}, RateMultiplier: 1,
				PricingAt: APIKeyGroupRoutePricingAt(ctx, time.Now()), Resolver: d.resolver,
			})
			if err == nil {
				return true
			}
		}
		return false
	}
	if priced(primary) || priced(fallbacks) {
		return nil
	}
	// Account aliases use the same snapshot mapping fallback as reservation and
	// billing. Do not add an unbounded per-model negative cache or a new DB scan.
	if d.accountMappedModels != nil {
		mapped := d.accountMappedModels(ctx, key, upstream)
		allPriced := len(mapped) > 0
		for _, model := range mapped {
			if !priced([]string{model}) {
				allPriced = false
				break
			}
		}
		if allPriced {
			return nil
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrModelPricingUnavailable
}

func (s *GatewayService) ValidateRequestPricing(ctx context.Context, key *APIKey, req InflightEstimateRequest) error {
	if s == nil {
		return ErrModelPricingUnavailable
	}
	return s.inflightEstimateDeps().validateRequestPricing(ctx, key, req)
}

func (s *OpenAIGatewayService) ValidateRequestPricing(ctx context.Context, key *APIKey, req InflightEstimateRequest) error {
	if s == nil {
		return ErrModelPricingUnavailable
	}
	return s.inflightEstimateDeps().validateRequestPricing(ctx, key, req)
}
