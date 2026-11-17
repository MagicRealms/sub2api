//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type iqVisibilityPlanRepo struct {
	ScheduledTestPlanRepository
	plan *ScheduledTestPlan
}

func (r *iqVisibilityPlanRepo) ListByKind(context.Context, string) ([]*ScheduledTestPlan, error) {
	return []*ScheduledTestPlan{r.plan}, nil
}

type iqVisibilityResultRepo struct {
	ScheduledTestResultRepository
	results []*ScheduledTestResult
}

func (r *iqVisibilityResultRepo) ListByPlanID(context.Context, int64, int) ([]*ScheduledTestResult, error) {
	return r.results, nil
}

func TestChannelIQVisibilityHidesPublicResultsWithoutChangingPlan(t *testing.T) {
	ctx := context.Background()
	repo := newRuntimeSettingRepoStub()
	settings := NewSettingService(repo, &config.Config{})
	invalidations := 0
	settings.SetOnUpdateCallback(func() { invalidations++ })
	nextRun := time.Now().Add(time.Hour)
	plan := &ScheduledTestPlan{ID: 5, AccountID: 12, ModelID: "gpt-6.1-sol", Enabled: true, NextRunAt: &nextRun}
	results := &iqVisibilityResultRepo{results: []*ScheduledTestResult{{ID: 4, Status: "failed", ErrorMessage: "private upstream failure"}}}
	svc := NewChannelIQService(&iqVisibilityPlanRepo{plan: plan}, results, nil, nil, nil, settings)

	visible, err := svc.GetPublicSnapshot(ctx)
	require.NoError(t, err)
	require.True(t, visible.Visible)
	require.True(t, visible.Enabled)
	require.Len(t, visible.Results, 1)

	require.NoError(t, svc.SetVisibility(ctx, false))
	hidden, err := svc.GetPublicSnapshot(ctx)
	require.NoError(t, err)
	require.False(t, hidden.Visible)
	require.False(t, hidden.Configured)
	require.Empty(t, hidden.ModelID)
	require.Empty(t, hidden.Results)
	admin, err := svc.GetAdminSnapshot(ctx)
	require.NoError(t, err)
	require.False(t, admin.Visible)
	require.True(t, admin.Config.Enabled)
	require.Equal(t, &nextRun, plan.NextRunAt)
	require.Equal(t, int64(12), plan.AccountID)
	require.Len(t, admin.Results, 1)
	require.Equal(t, "private upstream failure", admin.Results[0].ErrorMessage)

	require.NoError(t, svc.SetVisibility(ctx, true))
	visible, err = svc.GetPublicSnapshot(ctx)
	require.NoError(t, err)
	require.True(t, visible.Visible)
	require.Len(t, visible.Results, 1)
	require.Equal(t, 2, invalidations)
}

func TestChannelIQHiddenSkipsPlanAndResultReads(t *testing.T) {
	repo := newRuntimeSettingRepoStub()
	repo.values[SettingKeyChannelIQEnabled] = "false"
	// Nil plan/result repositories ensure no history is queried when hidden.
	svc := &ChannelIQService{settings: NewSettingService(repo, &config.Config{})}
	view, err := svc.GetPublicSnapshot(context.Background())
	require.NoError(t, err)
	require.False(t, view.Visible)
	require.NotNil(t, view.Results)
	require.Empty(t, view.Results)
}

func TestChannelIQVisibilityStorageFailureDoesNotPublish(t *testing.T) {
	readErr := errors.New("database unavailable")
	settings := NewSettingService(&settingPublicRepoStub{err: readErr}, &config.Config{})
	svc := &ChannelIQService{settings: settings}
	view, err := svc.GetPublicSnapshot(context.Background())
	require.ErrorIs(t, err, readErr)
	require.Nil(t, view)
}

func TestChannelIQVisibilityWriteFailureDoesNotInvalidate(t *testing.T) {
	repo := newRuntimeSettingRepoStub()
	repo.values[SettingKeyChannelIQEnabled] = "true"
	repo.setFn = func(string, string) error { return errors.New("write failed") }
	settings := NewSettingService(repo, &config.Config{})
	settings.SetOnUpdateCallback(func() { t.Fatal("failed write must not invalidate settings") })
	require.Error(t, settings.SetChannelIQVisibility(context.Background(), false))
	require.Equal(t, "true", repo.values[SettingKeyChannelIQEnabled])
}

func TestChannelIQVisibilityIsIncludedInPublicSettingsAndHTML(t *testing.T) {
	for _, raw := range []string{"", "true", "false"} {
		t.Run(raw, func(t *testing.T) {
			repo := &settingPublicRepoStub{values: map[string]string{SettingKeyChannelIQEnabled: raw}}
			settings := NewSettingService(repo, &config.Config{})
			public, err := settings.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, raw != "false", public.ChannelIQEnabled)
			injected, err := settings.GetPublicSettingsForInjection(context.Background())
			require.NoError(t, err)
			require.Equal(t, public.ChannelIQEnabled, injected.(*PublicSettingsInjectionPayload).ChannelIQEnabled)
		})
	}
}
