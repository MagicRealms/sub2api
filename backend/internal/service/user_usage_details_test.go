//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestFillUserUsageDaysAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, 10, 29, 0, 0, 0, 0, loc)
	days := fillUserUsageDays([]usagestats.UserUsageDay{{Date: "2026-11-01", UserUsageMetrics: usagestats.UserUsageMetrics{Requests: 3}}}, start)
	require.Len(t, days, 7)
	for i, expected := range []string{"2026-10-29", "2026-10-30", "2026-10-31", "2026-11-01", "2026-11-02", "2026-11-03", "2026-11-04"} {
		require.Equal(t, expected, days[i].Date)
	}
	require.Equal(t, int64(3), days[3].Requests)
	require.Zero(t, days[4].Requests)
}

type detailsUserRepo struct {
	UserRepository
	user *User
	err  error
}

func (r detailsUserRepo) GetByID(context.Context, int64) (*User, error) { return r.user, r.err }

type detailsUsageRepo struct {
	UsageLogRepository
	called     bool
	id         int64
	start, end time.Time
	timezone   string
}

func (r *detailsUsageRepo) GetUserUsageDetails(_ context.Context, id int64, start, end time.Time, timezone string) (*usagestats.UserUsageDetails, error) {
	r.called, r.id, r.start, r.end, r.timezone = true, id, start, end, timezone
	return &usagestats.UserUsageDetails{
		APIKeys: []usagestats.UserAPIKeyUsage{{ID: 11}},
		Total:   usagestats.UserUsageMetrics{Requests: 25},
	}, nil
}

func TestAdminUserUsageDetailsUsesCalendarWindow(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	repo := &detailsUsageRepo{}
	svc := NewUsageService(repo, detailsUserRepo{user: &User{ID: 42}}, nil, nil)
	user, details, err := svc.GetAdminUserUsageDetails(context.Background(), 42, loc)
	require.NoError(t, err)
	require.Equal(t, int64(42), user.ID)
	require.Equal(t, int64(42), repo.id)
	require.Equal(t, "Asia/Shanghai", repo.timezone)
	require.Equal(t, 0, repo.start.Hour())
	require.Equal(t, repo.end.In(loc).Format(time.DateOnly), repo.start.AddDate(0, 0, 6).Format(time.DateOnly))
	require.Equal(t, int64(25), details.Total.Requests)
	require.Len(t, details.Daily, 7)
	require.Len(t, details.APIKeys[0].Daily, 7)
	require.Equal(t, details.EndDate, details.Daily[6].Date)
	require.Zero(t, details.Daily[6].Requests)
}

func TestAdminUserUsageDetailsMissingUser(t *testing.T) {
	repo := &detailsUsageRepo{}
	svc := NewUsageService(repo, detailsUserRepo{err: ErrUserNotFound}, nil, nil)
	_, _, err := svc.GetAdminUserUsageDetails(context.Background(), 42, time.UTC)
	require.ErrorIs(t, err, ErrUserNotFound)
	require.False(t, repo.called)
}
