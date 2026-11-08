package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2GroupCardsSelectionAndPrivacy(t *testing.T) {
	group, empty, failures := int64(7), int64(8), int64(9)
	p50 := int64(1234)
	row := func(id *int64, model string, rate float64, requests, denominator int64) ChannelMonitorV2MatrixRow {
		metric := ChannelMonitorV2Metric{RequestCount: requests, SuccessRequests: requests, CacheRate: rate, CacheRateNumerator: 90, CacheRateDenominator: denominator, RPM: 3, TPM: 120,
			TTFT: ChannelMonitorV2Latency{SampleCount: requests, P50Ms: &p50}}
		return ChannelMonitorV2MatrixRow{GroupID: id, GroupName: model, Platform: "openai", Model: model, Metrics: metric,
			Buckets: []ChannelMonitorV2TrendPoint{{BucketStart: time.Now(), Metrics: metric}}}
	}
	matrix := &ChannelMonitorV2Matrix{Items: []ChannelMonitorV2MatrixRow{
		row(&group, "low-cache", .3, 1000, 10000),
		row(&group, "unlisted-model", .9, 10, 1000),
		row(&group, "tie-less-traffic", .9, 2, 1000),
		row(&group, "no-cache-denominator", 1, 2000, 0),
		row(&empty, "unused-configured-model", 0, 0, 0),
		row(&failures, "failed-model", 0, 5, 0),
		row(nil, "no-group", 1, 10000, 100000),
	}}
	repo := &channelMonitorV2RepoStub{config: ChannelMonitorV2Config{Enabled: true, Platforms: []ChannelMonitorV2PlatformConfig{{Platform: "openai", Models: []string{"listed-model"}}}}, matrix: matrix}
	svc := NewChannelMonitorV2Service(repo)
	filter := ChannelMonitorV2Filter{RestrictGroups: true, AllowedGroupIDs: []int64{7, 8, 9}, Models: []string{ChannelMonitorV2OtherModel}}
	result, err := svc.GroupCards(context.Background(), filter, false)
	require.NoError(t, err)
	require.Len(t, result.Items, 3)
	require.True(t, repo.matrixFilter.ExpandModels)
	require.Equal(t, filter.AllowedGroupIDs, repo.matrixFilter.AllowedGroupIDs)
	require.Equal(t, filter.Models, repo.matrixFilter.Models)
	require.Equal(t, []string{"listed-model"}, repo.matrixConfig.Platforms[0].Models)
	require.False(t, repo.admin)
	byID := map[int64]ChannelMonitorV2GroupCard{}
	for _, card := range result.Items {
		byID[*card.GroupID] = card
	}
	card := byID[group]
	require.Equal(t, "unlisted-model", card.Model)
	require.True(t, card.HasTraffic)
	require.True(t, card.HasCacheSamples)
	require.Equal(t, .9, card.Metrics.CacheRate)
	require.Equal(t, &p50, card.Metrics.TTFT.P50Ms)
	require.Zero(t, card.Metrics.RequestCount)
	require.Zero(t, card.Metrics.CacheRateDenominator)
	require.Zero(t, card.Metrics.TTFT.SampleCount)
	require.Zero(t, card.Metrics.RPM)
	require.Zero(t, card.Buckets[0].Metrics.RequestCount)
	require.True(t, card.Health.SampleLimited)
	require.NotEqual(t, "unknown", card.Buckets[0].Health.Overall)
	require.True(t, card.Buckets[0].Health.SampleLimited)
	require.Empty(t, matrix.Items[1].Buckets[0].Health.Overall)
	require.Equal(t, int64(10), matrix.Items[1].Metrics.RequestCount)
	require.Equal(t, int64(10), matrix.Items[1].Buckets[0].Metrics.RequestCount)
	require.False(t, byID[empty].HasTraffic)
	require.Empty(t, byID[empty].Model)
	require.True(t, byID[failures].HasTraffic)
	require.False(t, byID[failures].HasCacheSamples)
	admin, err := svc.GroupCards(context.Background(), filter, true)
	require.NoError(t, err)
	for _, card := range admin.Items {
		if *card.GroupID == group {
			require.Equal(t, int64(10), card.Metrics.RequestCount)
		}
	}
}

func TestChannelMonitorV2GroupCardsStableTiesAndEmptyScope(t *testing.T) {
	id := int64(1)
	a := ChannelMonitorV2MatrixRow{GroupID: &id, Model: "a", Metrics: ChannelMonitorV2Metric{RequestCount: 2, CacheRateDenominator: 100, CacheRate: .5}}
	b := a
	b.Model = "b"
	for _, rows := range [][]ChannelMonitorV2MatrixRow{{a, b}, {b, a}} {
		result := selectChannelMonitorV2GroupCards(&ChannelMonitorV2Matrix{Items: rows})
		require.Equal(t, "a", result.Items[0].Model)
	}
	result := selectChannelMonitorV2GroupCards(&ChannelMonitorV2Matrix{})
	require.NotNil(t, result.Items)
	require.Empty(t, result.Items)
}

func TestChannelMonitorV2ObservedHealthSparseAndMissingSignals(t *testing.T) {
	thresholds := DefaultChannelMonitorV2HealthThresholds()
	thresholds.WarningCacheRate, thresholds.CriticalCacheRate = .2, .05
	for _, tt := range []struct {
		name                                        string
		requests, successes, p50, latencySamples    int64
		errorRate, cacheRate                        float64
		cacheTokens                                 int64
		wantOverall, wantError, wantTTFT, wantCache string
	}{
		{"single-success", 1, 1, 1000, 1, 0, .8, 100, "healthy", "healthy", "healthy", "healthy"},
		{"single-failure", 1, 0, 0, 0, 1, 0, 0, "critical", "critical", "unknown", "unknown"},
		{"degraded", 10, 9, 1000, 9, .1, .8, 100, "warning", "warning", "healthy", "healthy"},
		{"sync-success", 1, 1, 0, 0, 0, 0, 0, "healthy", "healthy", "unknown", "unknown"},
		{"slow-stream", 1, 1, 15000, 1, 0, 0, 0, "warning", "healthy", "critical", "unknown"},
		{"cache-miss", 1, 1, 1000, 1, 0, 0, 100, "healthy", "healthy", "healthy", "critical"},
		{"empty-or-ignored-only", 0, 0, 0, 0, 0, 0, 0, "unknown", "unknown", "unknown", "unknown"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := ChannelMonitorV2Metric{RequestCount: tt.requests, SuccessRequests: tt.successes, ErrorRate: tt.errorRate, CacheRate: tt.cacheRate, CacheRateDenominator: tt.cacheTokens,
				TTFT: ChannelMonitorV2Latency{SampleCount: tt.latencySamples}}
			if tt.latencySamples > 0 {
				m.TTFT.P50Ms = &tt.p50
			}
			h := channelMonitorV2ObservedHealth(m, thresholds)
			require.Equal(t, tt.wantOverall, h.Overall)
			require.Equal(t, tt.wantError, h.ErrorRate)
			require.Equal(t, tt.wantTTFT, h.TTFT)
			require.Equal(t, tt.wantCache, h.Cache)
			require.Equal(t, tt.requests > 0, h.SampleLimited)
			require.Equal(t, thresholds, h.Thresholds)
			require.Equal(t, thresholds.MinimumSample, h.MinimumSample)
			// The strict aggregate API still enforces the operator's minimum.
			require.Equal(t, "unknown", ChannelMonitorV2HealthForWithThresholds(m, thresholds).Overall)
		})
	}
}

func TestChannelMonitorV2ObservedHealthKeepsConfiguredThresholdsAndConfidence(t *testing.T) {
	thresholds := DefaultChannelMonitorV2HealthThresholds()
	thresholds.WarningErrorRate, thresholds.CriticalErrorRate = .01, .04
	p50 := int64(1000)
	m := ChannelMonitorV2Metric{RequestCount: 50, ErrorRate: .04, CacheRateDenominator: 100, CacheRate: .8,
		TTFT: ChannelMonitorV2Latency{SampleCount: 50, P50Ms: &p50}}
	h := channelMonitorV2ObservedHealth(m, thresholds)
	require.False(t, h.SampleLimited)
	require.Equal(t, ChannelMonitorV2HealthForWithThresholds(m, thresholds), h)
	require.Equal(t, "critical", h.ErrorRate)
	m.TTFT.SampleCount = 1
	require.True(t, channelMonitorV2ObservedHealth(m, thresholds).SampleLimited)
}
