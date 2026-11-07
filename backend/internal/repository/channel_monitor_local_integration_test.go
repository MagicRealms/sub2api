//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorLocalGroupRoundTripAndClear(t *testing.T) {
	tx := testEntTx(t)
	ctx := context.Background()
	repo := NewChannelMonitorRepository(tx.Client(), integrationDB)
	// CRUD writes use the transaction context in this repository. Inspect directly
	// through the same client and adapt conversion, without touching production.
	key, group := int64(77), int64(6)
	m := &service.ChannelMonitor{Name: "pinned", Provider: "openai", APIMode: "responses", Endpoint: "https://example.com", APIKey: "cipher", PrimaryModel: "gpt-6.1-sol", Enabled: false, IntervalSeconds: 300, CreatedBy: 1, LocalAPIKeyID: &key, ProbeGroupID: &group}
	require.NoError(t, repo.Create(ctx, m))
	row, err := tx.Client().ChannelMonitor.Get(ctx, m.ID)
	require.NoError(t, err)
	restored := entToServiceMonitor(row)
	require.Equal(t, &key, restored.LocalAPIKeyID)
	require.Equal(t, &group, restored.ProbeGroupID)
	m.LocalAPIKeyID = nil
	m.ProbeGroupID = nil
	require.NoError(t, repo.Update(ctx, m))
	row, err = tx.Client().ChannelMonitor.Get(ctx, m.ID)
	require.NoError(t, err)
	require.Nil(t, row.LocalAPIKeyID)
	require.Nil(t, row.ProbeGroupID)
}
