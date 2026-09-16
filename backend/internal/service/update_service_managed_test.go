//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSourceBuildNeverFetchesOrReplacesBinary(t *testing.T) {
	// Nil dependencies deliberately make any accidental cache/network call fail.
	svc := NewUpdateService(nil, nil, "0.2.5-mr.1", "source")
	for _, force := range []bool{false, true} {
		info, err := svc.CheckUpdate(t.Context(), force)
		require.NoError(t, err)
		require.Equal(t, "0.2.5-mr.1", info.CurrentVersion)
		require.Equal(t, "source", info.BuildType)
		require.False(t, info.HasUpdate)
		require.Nil(t, info.ReleaseInfo)
	}
	require.ErrorIs(t, svc.PerformUpdate(t.Context()), ErrManagedDeployment)
	require.ErrorIs(t, svc.Rollback(), ErrManagedDeployment)
	require.ErrorIs(t, svc.RollbackToVersion(t.Context(), "0.2.4"), ErrManagedDeployment)
	versions, err := svc.ListRollbackVersions(t.Context())
	require.NoError(t, err)
	require.Empty(t, versions)
}
