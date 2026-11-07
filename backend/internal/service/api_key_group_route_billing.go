package service

import (
	"context"
	"time"
)

// Compare complete customer charges, not just nominal group multipliers. This
// also protects custom model prices, peak periods and independent image rates.
func higherOrderedRouteCost(actual, floor *CostBreakdown, rate, imageRate float64) *CostBreakdown {
	if floor == nil || (actual != nil && actual.ActualCost >= floor.ActualCost) {
		return actual
	}
	floor.RoutingRateMultiplier = &rate
	floor.RoutingImageMultiplier = &imageRate
	if actual != nil {
		upstreamTotal := actual.TotalCost
		floor.RoutingUpstreamTotalCost = &upstreamTotal
	}
	return floor
}

func orderedRouteAccountTotal(cost *CostBreakdown) float64 {
	if cost == nil {
		return 0
	}
	if cost.RoutingUpstreamTotalCost != nil {
		return *cost.RoutingUpstreamTotalCost
	}
	return cost.TotalCost
}

func orderedRoutingRecordedMultipliers(cost *CostBreakdown, rate, imageRate float64) (float64, float64) {
	if cost != nil && cost.RoutingRateMultiplier != nil {
		rate = *cost.RoutingRateMultiplier
	}
	if cost != nil && cost.RoutingImageMultiplier != nil {
		imageRate = *cost.RoutingImageMultiplier
	}
	return rate, imageRate
}

func (s *GatewayService) calculateRecordUsageCost(ctx context.Context, result *ForwardResult, key *APIKey, model string, rate, imageRate float64, at time.Time) *CostBreakdown {
	actual := s.calculateRecordUsageCostForGroup(ctx, result, key, model, rate, imageRate, at)
	floor := orderedRoutingFloorKey(key)
	if floor == nil {
		return actual
	}
	base := s.ResolveUserGroupRateMultiplier(ctx, key.UserID, *floor.GroupID, floor.Group.RateMultiplier)
	floorRate, floorImageRate := computePeakAwareMultipliers(floor, base, at)
	if key.RoutingModel != "" {
		model = key.RoutingModel
	}
	floorCost := s.calculateRecordUsageCostForGroup(ctx, result, floor, model, floorRate, floorImageRate, at)
	return higherOrderedRouteCost(actual, floorCost, floorRate, floorImageRate)
}

func (s *OpenAIGatewayService) calculateOpenAIRecordUsageCost(
	ctx context.Context, result *OpenAIForwardResult, key *APIKey, models []string,
	rate, imageRate, videoRate, searchRate float64, tokens UsageTokens,
	tier string, longContextGate *bool, longContextExempt bool, at time.Time,
) (*CostBreakdown, error) {
	actual, err := s.calculateOpenAIRecordUsageCostForGroup(ctx, result, key, models, rate, imageRate, videoRate, searchRate, tokens, tier, longContextGate, longContextExempt, at)
	if err != nil {
		return nil, err
	}
	floor := orderedRoutingFloorKey(key)
	if floor == nil {
		return actual, nil
	}
	// Each group's Fast policy is evaluated independently before comparing.
	if groupBillsOpenAIFastAtStandard(key, &Account{Platform: PlatformOpenAI}, tier) {
		standard, e := s.calculateOpenAIRecordUsageCostForGroup(ctx, result, key, models, rate, imageRate, videoRate, searchRate, tokens, "", longContextGate, longContextExempt, at)
		if e != nil {
			return nil, e
		}
		actual.ActualCost = standard.ActualCost
	}
	base := s.ResolveUserGroupRateMultiplier(ctx, key.UserID, *floor.GroupID, floor.Group.RateMultiplier)
	floorRate, floorImageRate := computePeakAwareMultipliers(floor, base, at)
	floorVideoRate := resolveVideoRateMultiplier(floor, base)
	floorModels := models
	if key.RoutingModel != "" {
		floorModels = usageBillingModelCandidates(key.RoutingModel, models...)
	}
	floorCost, err := s.calculateOpenAIRecordUsageCostForGroup(ctx, result, floor, floorModels, floorRate, floorImageRate, floorVideoRate, base, tokens, tier, longContextGate, longContextExempt, at)
	if err != nil {
		return nil, err
	}
	if groupBillsOpenAIFastAtStandard(floor, &Account{Platform: PlatformOpenAI}, tier) {
		standard, e := s.calculateOpenAIRecordUsageCostForGroup(ctx, result, floor, floorModels, floorRate, floorImageRate, floorVideoRate, base, tokens, "", longContextGate, longContextExempt, at)
		if e != nil {
			return nil, e
		}
		floorCost.ActualCost = standard.ActualCost
	}
	return higherOrderedRouteCost(actual, floorCost, floorRate, floorImageRate), nil
}
