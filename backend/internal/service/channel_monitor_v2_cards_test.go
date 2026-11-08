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
