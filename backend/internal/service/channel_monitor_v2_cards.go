package service

import (
	"context"
	"sort"
)

// Group cards use the same V2 facts, histograms, coverage and health thresholds.
// Observed health is shown even for sparse traffic, with confidence marked separately.
// Presence flags survive volume redaction without exposing absolute counts.
type ChannelMonitorV2GroupCard struct {
	ChannelMonitorV2MatrixRow
	HasTraffic      bool `json:"has_traffic"`
	HasCacheSamples bool `json:"has_cache_samples"`
}

type ChannelMonitorV2GroupCards struct {
	Coverage ChannelMonitorV2Coverage    `json:"coverage"`
	Items    []ChannelMonitorV2GroupCard `json:"items"`
}

func (s *ChannelMonitorV2Service) GroupCards(ctx context.Context, filter ChannelMonitorV2Filter, admin bool) (*ChannelMonitorV2GroupCards, error) {
	cfg, err := s.getEnabledConfig(ctx)
	if err != nil {
		return nil, err
	}
	// Expand presentation names while keeping the original model-filter semantics
	// (including the configured "Other models" selection) and viewer scope.
	filter.ExpandModels = true
	matrix, err := s.repo.GetMatrix(ctx, filter, *cfg, ChannelMonitorV2GroupByPlatformGroupModel, admin)
	if err != nil || matrix == nil {
		return nil, err
	}
	result := selectChannelMonitorV2GroupCards(matrix)
	for i := range result.Items {
		card := &result.Items[i]
		card.Health = channelMonitorV2ObservedHealth(card.Metrics, cfg.HealthThresholds)
		for j := range card.Buckets {
			bucket := &card.Buckets[j]
			bucket.Health = channelMonitorV2ObservedHealth(bucket.Metrics, cfg.HealthThresholds)
		}
	}
	if !admin {
		hideTP := s.hideThroughputForViewer(ctx, admin)
		for i := range result.Items {
			redactChannelMonitorV2Metric(&result.Items[i].Metrics, hideTP)
			for j := range result.Items[i].Buckets {
				redactChannelMonitorV2Metric(&result.Items[i].Buckets[j].Metrics, hideTP)
			}
		}
	}
	return result, nil
}

// A card describes observed requests, not statistical certainty. Reuse the
// configured scoring budgets/weights while allowing one real sample to be
// displayed. Retain the configured minimum and flag low confidence explicitly.
// Empty/ignored-only buckets stay unknown, as do absent TTFT/cache measurements.
func channelMonitorV2ObservedHealth(metrics ChannelMonitorV2Metric, thresholds ChannelMonitorV2HealthThresholds) ChannelMonitorV2Health {
	thresholds = NormalizeChannelMonitorV2HealthThresholds(thresholds)
	observed := thresholds
	observed.MinimumSample = 1
	health := ChannelMonitorV2HealthForWithThresholds(metrics, observed)
	health.MinimumSample = thresholds.MinimumSample
	health.Thresholds = thresholds
	if metrics.RequestCount > 0 {
		health.SampleLimited = metrics.RequestCount < thresholds.MinimumSample ||
			(health.TTFTScore != nil && metrics.TTFT.SampleCount < thresholds.MinimumSample) ||
			(health.CacheScore != nil && metrics.CacheRateDenominator < thresholds.MinimumSample)
	}
	return health
}

func selectChannelMonitorV2GroupCards(matrix *ChannelMonitorV2Matrix) *ChannelMonitorV2GroupCards {
	groups := make(map[int64]ChannelMonitorV2MatrixRow)
	for _, row := range matrix.Items {
		if row.GroupID == nil || *row.GroupID <= 0 {
			continue
		}
		current, ok := groups[*row.GroupID]
		if !ok || betterChannelMonitorV2CardModel(row, current) {
			groups[*row.GroupID] = row
		}
	}
	result := &ChannelMonitorV2GroupCards{Coverage: matrix.Coverage, Items: make([]ChannelMonitorV2GroupCard, 0, len(groups))}
	for _, row := range groups {
		hasTraffic := row.Metrics.RequestCount > 0
		if !hasTraffic {
			row.Model = ""
		}
		// Copy buckets before public redaction; callers may retain the matrix.
		row.Buckets = append([]ChannelMonitorV2TrendPoint{}, row.Buckets...)
		result.Items = append(result.Items, ChannelMonitorV2GroupCard{
			ChannelMonitorV2MatrixRow: row,
			HasTraffic:                hasTraffic,
			HasCacheSamples:           hasTraffic && row.Metrics.CacheRateDenominator > 0,
		})
	}
	sort.Slice(result.Items, func(i, j int) bool {
		a, b := result.Items[i], result.Items[j]
		if a.Platform != b.Platform {
			return a.Platform < b.Platform
		}
		if a.GroupName != b.GroupName {
			return a.GroupName < b.GroupName
		}
		return *a.GroupID < *b.GroupID
	})
	return result
}

func betterChannelMonitorV2CardModel(a, b ChannelMonitorV2MatrixRow) bool {
	// A sampled model always beats an empty configured placeholder. Prefer a
	// meaningful cache denominator, then highest cache rate. Ties favor more
	// traffic, then model name, so refreshes cannot randomly shuffle the choice.
	aTraffic, bTraffic := a.Metrics.RequestCount > 0, b.Metrics.RequestCount > 0
	if aTraffic != bTraffic {
		return aTraffic
	}
	aCache, bCache := a.Metrics.CacheRateDenominator > 0, b.Metrics.CacheRateDenominator > 0
	if aCache != bCache {
		return aCache
	}
	if aCache && a.Metrics.CacheRate != b.Metrics.CacheRate {
		return a.Metrics.CacheRate > b.Metrics.CacheRate
	}
	if a.Metrics.RequestCount != b.Metrics.RequestCount {
		return a.Metrics.RequestCount > b.Metrics.RequestCount
	}
	return a.Model < b.Model
}
