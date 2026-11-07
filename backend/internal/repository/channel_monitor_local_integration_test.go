//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorLocalGroupRoundTripAndClear(t *testing.T) {
	tx := testEntTx(t)
	ctx := context.Background()
	repo := NewChannelMonitorRepository(tx.Client(), integrationDB)
	// The Ent test schema does not reproduce legacy migration CHECK constraints.
	// Start with the deployed guard, then prove the catalog migration works twice.
	_, err := tx.Client().ExecContext(ctx, `ALTER TABLE channel_monitors DROP CONSTRAINT IF EXISTS channel_monitors_check_mode_check;
ALTER TABLE channel_monitors ADD CONSTRAINT channel_monitors_check_mode_check CHECK (check_mode IN ('probe', 'quota', 'quota_probe'))`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("252_channel_monitor_catalog_mode.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = tx.Client().ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	// CRUD writes use the transaction context in this repository. Inspect directly
	// through the same client and adapt conversion, without touching production.
	key, group := int64(77), int64(6)
	m := &service.ChannelMonitor{Name: "pinned", Provider: "openai", APIMode: "responses", CheckMode: service.MonitorCheckModeCatalog, Endpoint: "https://example.com", APIKey: "cipher", PrimaryModel: "gpt-image-2.6", Enabled: false, IntervalSeconds: 300, CreatedBy: 1, LocalAPIKeyID: &key, ProbeGroupID: &group}
	require.NoError(t, repo.Create(ctx, m))
	row, err := tx.Client().ChannelMonitor.Get(ctx, m.ID)
	require.NoError(t, err)
	restored := entToServiceMonitor(row)
	require.Equal(t, &key, restored.LocalAPIKeyID)
	require.Equal(t, &group, restored.ProbeGroupID)
	require.Equal(t, service.MonitorCheckModeCatalog, restored.CheckMode)
	m.LocalAPIKeyID = nil
	m.ProbeGroupID = nil
	require.NoError(t, repo.Update(ctx, m))
	row, err = tx.Client().ChannelMonitor.Get(ctx, m.ID)
	require.NoError(t, err)
	require.Nil(t, row.LocalAPIKeyID)
	require.Nil(t, row.ProbeGroupID)
}
