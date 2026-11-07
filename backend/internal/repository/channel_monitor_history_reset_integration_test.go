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

func newHistoryResetMonitor(t *testing.T, repo service.ChannelMonitorRepository, enabled bool) *service.ChannelMonitor {
	t.Helper()
	m := &service.ChannelMonitor{
		Name:     fmt.Sprintf("history-reset-%d", time.Now().UnixNano()),
		Provider: service.MonitorProviderOpenAI, APIMode: service.MonitorAPIModeResponses,
		Endpoint: "https://example.com", APIKey: "encrypted-test-key", PrimaryModel: "gpt-test",
		ExtraModels: []string{"gpt-extra"}, GroupName: "preserve-group", Enabled: enabled,
		IntervalSeconds: 600, JitterSeconds: 30, CreatedBy: 1,
	}
	require.NoError(t, repo.Create(context.Background(), m))
	t.Cleanup(func() { _ = repo.Delete(context.Background(), m.ID) })
	return m
}

func historyResetCount(t *testing.T, table string, id int64) int {
	t.Helper()
	// The table is chosen by the test, never by API input.
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE monitor_id=$1", id).Scan(&count))
	return count
}

func TestChannelMonitorHistoryResetSingleAndAll(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelMonitorRepository(testEntClient(t), integrationDB)
	a := newHistoryResetMonitor(t, repo, true)
	b := newHistoryResetMonitor(t, repo, false)
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -3)
	rows := []*service.ChannelMonitorHistoryRow{
		{MonitorID: a.ID, Model: a.PrimaryModel, Status: service.MonitorStatusOperational, CheckedAt: now},
		{MonitorID: a.ID, Model: a.PrimaryModel, Status: service.MonitorStatusError, CheckedAt: old},
		{MonitorID: b.ID, Model: b.PrimaryModel, Status: service.MonitorStatusOperational, CheckedAt: now},
	}
	require.NoError(t, repo.InsertHistoryBatch(ctx, rows))
	for _, m := range []*service.ChannelMonitor{a, b} {
		require.NoError(t, repo.MarkChecked(ctx, m.ID, now))
	}
	for _, day := range []time.Time{now, old} {
		_, err := repo.UpsertDailyRollupsFor(ctx, day)
		require.NoError(t, err)
	}
	require.Equal(t, 2, historyResetCount(t, "channel_monitor_daily_rollups", a.ID))
	beforeA, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	beforeB, err := repo.GetByID(ctx, b.ID)
	require.NoError(t, err)
	result, err := repo.ResetHistory(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, &service.ChannelMonitorHistoryResetResult{MonitorsReset: 1, HistoryDeleted: 2, RollupsDeleted: 2}, result)
	require.Zero(t, historyResetCount(t, "channel_monitor_histories", a.ID))
	require.Zero(t, historyResetCount(t, "channel_monitor_daily_rollups", a.ID))
	require.Equal(t, 1, historyResetCount(t, "channel_monitor_histories", b.ID))
	require.Equal(t, 1, historyResetCount(t, "channel_monitor_daily_rollups", b.ID))
	afterA, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Nil(t, afterA.LastCheckedAt)
	beforeA.LastCheckedAt, beforeA.UpdatedAt = nil, afterA.UpdatedAt
	require.Equal(t, beforeA, afterA, "only history-related metadata may change")
	afterB, err := repo.GetByID(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, beforeB, afterB, "another monitor is untouched")
	latest, err := repo.ListLatestForMonitorIDs(ctx, []int64{a.ID})
	require.NoError(t, err)
	require.Empty(t, latest[a.ID])
	stats, err := repo.ComputeAvailability(ctx, a.ID, 7)
	require.NoError(t, err)
	require.Empty(t, stats, "no stale availability after reset")

	// New checks start a clean availability window.
	require.NoError(t, repo.InsertHistoryBatch(ctx, rows[:1]))
	stats, err = repo.ComputeAvailability(ctx, a.ID, 7)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, 1, stats[0].TotalChecks)
	require.Equal(t, float64(100), stats[0].AvailabilityPct)

	// Preserve real billing data even when every monitor, including disabled ones, is reset.
	client := testEntClient(t)
	u := mustCreateUser(t, client, &service.User{Balance: 12.34})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Key: fmt.Sprintf("sk-reset-%d", time.Now().UnixNano()), Name: "billing-preserved", QuotaUsed: 2.5})
	account := mustCreateAccount(t, client, &service.Account{Name: "reset-billing-preserved"})
	usage := &service.UsageLog{UserID: u.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: fmt.Sprintf("reset-usage-%d", time.Now().UnixNano()), Model: "gpt-test", InputTokens: 10, OutputTokens: 1, ActualCost: 0.02, CreatedAt: now}
	_, err = newUsageLogRepositoryWithSQL(client, integrationDB).Create(ctx, usage)
	require.NoError(t, err)
	require.NotZero(t, usage.ID)
	result, err = repo.ResetHistory(ctx, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, result.MonitorsReset, int64(2))
	for _, m := range []*service.ChannelMonitor{a, b} {
		require.Zero(t, historyResetCount(t, "channel_monitor_histories", m.ID))
		require.Zero(t, historyResetCount(t, "channel_monitor_daily_rollups", m.ID))
		loaded, err := repo.GetByID(ctx, m.ID)
		require.NoError(t, err)
		require.Equal(t, m.Enabled, loaded.Enabled)
		require.Equal(t, m.APIKey, loaded.APIKey)
		require.Equal(t, m.IntervalSeconds, loaded.IntervalSeconds)
		require.Nil(t, loaded.LastCheckedAt)
	}
	var balance, quota, cost float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", u.ID).Scan(&balance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id=$1", key.ID).Scan(&quota))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT actual_cost FROM usage_logs WHERE id=$1", usage.ID).Scan(&cost))
	require.InDelta(t, 12.34, balance, 0.000001)
	require.InDelta(t, 2.5, quota, 0.000001)
	require.InDelta(t, 0.02, cost, 0.000001)
}

func TestChannelMonitorHistoryResetDoesNotRebuildStaleRollups(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo := NewChannelMonitorRepository(testEntClient(t), integrationDB)
	m := newHistoryResetMonitor(t, repo, false)
	now := time.Now().UTC()
	require.NoError(t, repo.InsertHistoryBatch(ctx, []*service.ChannelMonitorHistoryRow{{MonitorID: m.ID, Model: m.PrimaryModel, Status: service.MonitorStatusError, CheckedAt: now}}))
	_, err := repo.UpsertDailyRollupsFor(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 1, historyResetCount(t, "channel_monitor_daily_rollups", m.ID))
	// Pause aggregation at the reset lock while old histories still exist.
	gate, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = gate.Rollback() }()
	require.NoError(t, lockChannelMonitorHistory(ctx, gate))
	done := make(chan error, 1)
	go func() {
		_, err := repo.UpsertDailyRollupsFor(ctx, now)
		done <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype='advisory' AND classid=$1 AND objid=$2 AND objsubid=1 AND NOT granted)", channelMonitorHistoryLock>>32, channelMonitorHistoryLock&0xffffffff).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	// Complete the reset before allowing aggregation to resume. It must acquire a
	// fresh snapshot after waiting and cannot resurrect the deleted status rows.
	_, err = gate.ExecContext(ctx, "DELETE FROM channel_monitor_daily_rollups WHERE monitor_id=$1", m.ID)
	require.NoError(t, err)
	_, err = gate.ExecContext(ctx, "DELETE FROM channel_monitor_histories WHERE monitor_id=$1", m.ID)
	require.NoError(t, err)
	require.NoError(t, gate.Commit())
	require.NoError(t, <-done)
	require.Zero(t, historyResetCount(t, "channel_monitor_daily_rollups", m.ID))
}
