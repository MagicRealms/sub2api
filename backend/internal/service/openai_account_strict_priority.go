package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

var errOpenAIStrictPriorityChanged = fmt.Errorf("%w: account priority changed during strict selection", ErrNoAvailableAccounts)

// useOpenAIStrictPriority is deliberately group-scoped and text-only. Images,
// media task owners, other providers and installations without an opt-in retain
// their current routing. This policy is independent of the weighted scheduler's
// global switch, so disabling scoring cannot silently restore capacity spillover.
func (s *OpenAIGatewayService) useOpenAIStrictPriority(ctx context.Context, groupID *int64, platform string, imageCapability OpenAIImagesCapability, useUpstreamTokenCost bool) bool {
	if s == nil || s.cfg == nil || groupID == nil || *groupID <= 0 ||
		platform != PlatformOpenAI || imageCapability != "" || !useUpstreamTokenCost {
		return false
	}
	if _, suppressed := ctx.Value(openAIProfitControlSuppressCtxKey{}).(struct{}); suppressed {
		return false
	}
	for _, id := range s.cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs {
		if id == *groupID {
			return true
		}
	}
	return false
}

// selectByStrictPriority runs only after the shared eligibility gates. A tier
// contains accounts with the same numeric priority; scores/load/sticky may pick
// within that tier, never across tiers. An eligible but full tier returns the
// existing bounded WaitPlan, even when a backup has free capacity. Only excluded
// or genuinely ineligible accounts allow the next tier to be considered.
func (s *defaultOpenAIAccountScheduler) selectByStrictPriority(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
	accounts []*Account,
	budget *openAISelectionProbeBudget,
	filterStats openAISelectionFilterStats,
) (*AccountSelectionResult, int, int, float64, error) {
	ordered := append([]*Account(nil), accounts...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Priority < ordered[j].Priority })
	compactBlocked := false
	for start := 0; start < len(ordered); {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, 0, err
		}
		end := start + 1
		for end < len(ordered) && ordered[end].Priority == ordered[start].Priority {
			end++
		}
		tier := ordered[start:end]
		loadMap := map[int64]*AccountLoadInfo{}
		if s.service.concurrencyService != nil {
			if loads, err := s.service.concurrencyService.GetAccountsLoadBatch(ctx, buildOpenAIAccountLoadRequest(tier)); err == nil {
				loadMap = loads
			}
		}
		attempt := s.trySelectByLoadBalancePool(ctx, req, tier, loadMap, budget)
		if attempt.err != nil && !attempt.noCompactCandidates {
			return nil, attempt.candidateCount, attempt.topK, attempt.loadSkew, attempt.err
		}
		if attempt.result != nil {
			return attempt.result, attempt.candidateCount, attempt.topK, attempt.loadSkew, nil
		}
		compactBlocked = compactBlocked || attempt.compactBlocked || attempt.noCompactCandidates
		if attempt.err == nil {
			result, count, topK, skew, err := s.finishLoadBalanceSelectionFallback(ctx, req, attempt, budget, filterStats)
			unavailable := errors.Is(err, ErrNoAvailableAccounts) || errors.Is(err, ErrNoAvailableCompactAccounts)
			if err == nil || !unavailable || errors.Is(err, errOpenAIStrictPriorityChanged) {
				return result, count, topK, skew, err
			}
			// Exhausting a defensive work budget is not evidence that a cheaper
			// tier is unhealthy; fail closed instead of routing around it.
			if budget.acquireExhausted() || (budget.limited && budget.rechecks >= openAIAccountSelectionProbeLimit) {
				return nil, count, topK, skew, err
			}
		}
		start = end
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, 0, err
	}
	return nil, 0, 0, 0, noAvailableOpenAISelectionError(req.RequestedModel, compactBlocked, filterStats.summary("strict_priority_exhausted"))
}
