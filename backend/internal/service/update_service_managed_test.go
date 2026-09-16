//go:build unit

package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSourceBuildNeverReplacesBinary(t *testing.T) {
	// Nil dependencies deliberately make any accidental cache/network call fail.
	svc := NewUpdateService(nil, nil, "0.2.5-mr.1", "source")
	require.ErrorIs(t, svc.PerformUpdate(t.Context()), ErrManagedDeployment)
	require.ErrorIs(t, svc.Rollback(), ErrManagedDeployment)
	require.ErrorIs(t, svc.RollbackToVersion(t.Context(), "0.2.4"), ErrManagedDeployment)
	versions, err := svc.ListRollbackVersions(t.Context())
	require.NoError(t, err)
	require.Empty(t, versions)
}

func TestSourceBuildChecksUpstreamBaseVersion(t *testing.T) {
	for _, tc := range []struct {
		tag        string
		wantUpdate bool
	}{
		{"v0.2.4", false},
		{"v0.2.5", false},
		{"v0.2.6", true},
		{"v0.3.0", true},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			client := &updateServiceGitHubClientStub{release: &GitHubRelease{
				TagName: tc.tag, HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/" + tc.tag,
			}}
			svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.5-mr.abcdef123456", "source")
			info, err := svc.CheckUpdate(t.Context(), false)
			require.NoError(t, err)
			require.Equal(t, "Wei-Shaw/sub2api", client.latestRepo)
			require.Equal(t, tc.wantUpdate, info.HasUpdate)
			require.Equal(t, "0.2.5-mr.abcdef123456", info.CurrentVersion)
			require.Equal(t, "source", info.BuildType)
			require.Equal(t, client.release.HTMLURL, info.ReleaseInfo.HTMLURL)
			// Notifications do not grant install or rollback permission.
			require.ErrorIs(t, svc.PerformUpdate(t.Context()), ErrManagedDeployment)
			require.ErrorIs(t, svc.RollbackToVersion(t.Context(), "0.2.4"), ErrManagedDeployment)
			require.Equal(t, 1, client.latestCalls)
		})
	}
}

func TestUpdateCheckCacheTracksUpstreamAndCurrentBuild(t *testing.T) {
	cache := &updateServiceCacheStub{}
	client := &updateServiceGitHubClientStub{release: &GitHubRelease{TagName: "v0.2.6"}}
	svc := NewUpdateService(cache, client, "0.2.5-mr.old", "source")
	info, err := svc.CheckUpdate(t.Context(), false)
	require.NoError(t, err)
	require.True(t, info.HasUpdate)
	// A deploy must recompute the comparison instead of reusing the old result.
	svc = NewUpdateService(cache, client, "0.2.6-mr.new", "source")
	info, err = svc.CheckUpdate(t.Context(), false)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.False(t, info.HasUpdate)
	require.Equal(t, "0.2.6-mr.new", info.CurrentVersion)
	require.Equal(t, 1, client.latestCalls)
	_, err = svc.CheckUpdate(t.Context(), true)
	require.NoError(t, err)
	require.Equal(t, 2, client.latestCalls)
}

func TestUpdateCheckRejectsOldForkOrExpiredCache(t *testing.T) {
	for _, tc := range []struct {
		repo string
		age  int64
	}{
		{"", 0}, {"MagicRealms/sub2api", 0}, {upstreamRepo, updateCacheTTL + 1},
	} {
		data, err := json.Marshal(map[string]any{
			"repo": tc.repo, "latest": "99.0.0", "timestamp": time.Now().Unix() - tc.age,
		})
		require.NoError(t, err)
		client := &updateServiceGitHubClientStub{release: &GitHubRelease{TagName: "v0.2.5"}}
		svc := NewUpdateService(&updateServiceCacheStub{data: string(data)}, client, "0.2.5-mr.1", "source")
		info, err := svc.CheckUpdate(t.Context(), false)
		require.NoError(t, err)
		require.False(t, info.HasUpdate)
		require.False(t, info.Cached)
		require.Equal(t, "0.2.5", info.LatestVersion)
		require.Equal(t, 1, client.latestCalls)
	}
}

func TestUpdateCheckFailureReportsWarningWithOrWithoutCache(t *testing.T) {
	cache := &updateServiceCacheStub{}
	client := &updateServiceGitHubClientStub{latestErr: errors.New("github unavailable")}
	svc := NewUpdateService(cache, client, "0.2.5-mr.1", "source")
	info, err := svc.CheckUpdate(t.Context(), true)
	require.NoError(t, err)
	require.Contains(t, info.Warning, "github unavailable")
	require.Equal(t, "source", info.BuildType)
	require.False(t, info.Cached)
	client.latestErr = nil
	client.release = &GitHubRelease{TagName: "v0.2.6"}
	_, err = svc.CheckUpdate(t.Context(), true)
	require.NoError(t, err)
	client.latestErr = errors.New("github unavailable")
	info, err = svc.CheckUpdate(t.Context(), true)
	require.NoError(t, err)
	require.Contains(t, info.Warning, "github unavailable")
	require.True(t, info.Cached)
	require.True(t, info.HasUpdate)
}

func TestReleaseDeploymentStillUsesForkInsteadOfNotificationAssets(t *testing.T) {
	client := &updateServiceGitHubClientStub{release: &GitHubRelease{TagName: "v0.2.5"}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.5", "release")
	_, err := svc.CheckUpdate(t.Context(), true)
	require.NoError(t, err)
	require.Equal(t, "Wei-Shaw/sub2api", client.latestRepo)
	require.ErrorIs(t, svc.PerformUpdate(t.Context()), ErrNoUpdateAvailable)
	require.Equal(t, "MagicRealms/sub2api", client.latestRepo)
}
