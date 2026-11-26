//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsGroupExclusionUsesCurrentPolicy(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID})
	account := mustCreateAccount(t, client, &service.Account{Name: "ops-scope", Platform: service.PlatformOpenAI})
	normal := mustCreateGroup(t, client, &service.Group{Name: "ops-scope-normal", Platform: service.PlatformOpenAI})
	excluded := mustCreateGroup(t, client, &service.Group{Name: "ops-scope-excluded", Platform: service.PlatformOpenAI})
	start := time.Date(2021, 1, 19, 0, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)
	at := start.Add(70 * time.Minute)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM ops_metrics_hourly WHERE bucket_start >= $1 AND bucket_start < $2", start, end)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM ops_error_logs WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE id = $1", key.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", account.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id IN ($1, $2)", normal.ID, excluded.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	repo := NewOpsRepository(integrationDB)
	groupRepo := NewGroupRepository(client, integrationDB)
	for _, g := range []*service.Group{normal, excluded} {
		_, err := integrationDB.ExecContext(ctx, `
INSERT INTO usage_logs (user_id, api_key_id, account_id, group_id, request_id, model, input_tokens, output_tokens, duration_ms, request_type, created_at)
VALUES ($1,$2,$3,$4,$5,'ops-scope-model',10,20,1000,$6,$7)`, user.ID, key.ID, account.ID, g.ID, fmt.Sprintf("ops-scope-success-%d", g.ID), service.RequestTypeSync, at)
		require.NoError(t, err)
	}
	var excludedErrorID int64
	for i, groupID := range []*int64{&normal.ID, &excluded.ID, nil} {
		errID, err := repo.InsertErrorLog(ctx, &service.OpsInsertErrorLogInput{
			RequestID: fmt.Sprintf("ops-scope-error-%d-%d", user.ID, i), UserID: &user.ID, GroupID: groupID,
			Platform: service.PlatformOpenAI, Model: "ops-scope-model", ErrorPhase: "upstream",
			ErrorType: "api_error", ErrorOwner: "provider", Severity: "error", StatusCode: 502, CreatedAt: at,
			ErrorMessage: "upstream request failed",
		})
		require.NoError(t, err)
		if groupID != nil && *groupID == excluded.ID {
			excludedErrorID = errID
		}
	}
	require.NoError(t, repo.UpsertHourlyMetrics(ctx, start, end))

	// Change the policy after the historical hourly rows have been populated.
	group, err := groupRepo.GetByIDLite(ctx, excluded.ID)
	require.NoError(t, err)
	require.False(t, *group.ExcludeFromOps)
	on := true
	group.ExcludeFromOps = &on
	require.NoError(t, groupRepo.Update(ctx, group))
	otherUpdate := *group
	otherUpdate.ExcludeFromOps = nil
	otherUpdate.Description = "changed without updating Ops scope"
	require.NoError(t, groupRepo.Update(ctx, &otherUpdate))
	require.True(t, *otherUpdate.ExcludeFromOps)
	scopeKey, err := repo.OpsGroupExclusionKey(ctx)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprint(excluded.ID), scopeKey)

	for _, mode := range []service.OpsQueryMode{service.OpsQueryModeRaw, service.OpsQueryModeAuto, service.OpsQueryModePreagg} {
		for _, platform := range []string{"", service.PlatformOpenAI} {
			filter := &service.OpsDashboardFilter{StartTime: start, EndTime: end, QueryMode: mode, Platform: platform}
			summary, err := repo.GetDashboardOverview(ctx, filter)
			require.NoError(t, err)
			require.EqualValues(t, 1, summary.SuccessCount)
			require.EqualValues(t, 2, summary.ErrorCountTotal)
			require.EqualValues(t, 2, summary.UpstreamErrorCountExcl429529)
			require.EqualValues(t, 1, summary.OutputTPS.SampleCount)

			filter.GroupID = &excluded.ID
			summary, err = repo.GetDashboardOverview(ctx, filter)
			require.NoError(t, err)
			require.EqualValues(t, 1, summary.SuccessCount)
			require.EqualValues(t, 1, summary.ErrorCountTotal)
		}
	}

	for _, tc := range []struct {
		groupID  *int64
		failures int64
		total    int64
	}{
		{nil, 2, 3},
		{&excluded.ID, 1, 2},
	} {
		dashboard := &service.OpsDashboardFilter{StartTime: start, EndTime: end, GroupID: tc.groupID}
		trend, err := repo.GetThroughputTrend(ctx, dashboard, 3600)
		require.NoError(t, err)
		require.EqualValues(t, tc.total, trend.Points[1].RequestCount)
		if tc.groupID == nil {
			require.Len(t, trend.ByPlatform, 1)
			require.EqualValues(t, tc.total, trend.ByPlatform[0].RequestCount)
			dashboard.Platform = service.PlatformOpenAI
			byGroup, err := repo.GetThroughputTrend(ctx, dashboard, 3600)
			require.NoError(t, err)
			require.Len(t, byGroup.TopGroups, 1)
			require.Equal(t, normal.ID, byGroup.TopGroups[0].GroupID)
		}
		distribution, err := repo.GetErrorDistribution(ctx, dashboard)
		require.NoError(t, err)
		require.EqualValues(t, tc.failures, distribution.Total)
		list, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{GroupID: tc.groupID, StartTime: &start, EndTime: &end, ApplyGroupExclusions: true})
		require.NoError(t, err)
		require.EqualValues(t, tc.failures, list.Total)
		_, total, err := repo.ListRequestDetails(ctx, &service.OpsRequestDetailFilter{GroupID: tc.groupID, StartTime: &start, EndTime: &end})
		require.NoError(t, err)
		require.EqualValues(t, tc.total, total)
	}
	log, err := repo.GetErrorLogByID(ctx, excludedErrorID)
	require.NoError(t, err)
	require.Equal(t, &excluded.ID, log.GroupID)
	userErrors, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{UserID: &user.ID, StartTime: &start, EndTime: &end})
	require.NoError(t, err)
	require.EqualValues(t, 3, userErrors.Total)

	off := false
	group.ExcludeFromOps = &off
	require.NoError(t, groupRepo.Update(ctx, group))
	summary, err := repo.GetDashboardOverview(ctx, &service.OpsDashboardFilter{StartTime: start, EndTime: end, QueryMode: service.OpsQueryModePreagg})
	require.NoError(t, err)
	require.EqualValues(t, 2, summary.SuccessCount)
	require.EqualValues(t, 3, summary.ErrorCountTotal)
}
