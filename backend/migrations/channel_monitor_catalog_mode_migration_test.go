package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorCatalogModeMigration(t *testing.T) {
	content, err := FS.ReadFile("252_channel_monitor_catalog_mode.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS channel_monitors_check_mode_check")
	require.Contains(t, sql, "CHECK (check_mode IN ('probe', 'quota', 'quota_probe', 'catalog'))")
	require.NotContains(t, sql, "UPDATE channel_monitors")
	require.NotContains(t, sql, "DELETE FROM")
}
