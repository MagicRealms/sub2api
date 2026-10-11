package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// Keep this optional capability separate from the general usage repository so
// consumers that only write usage logs do not need an admin reporting API.
type userUsageDetailsRepository interface {
	GetUserUsageDetails(context.Context, int64, time.Time, time.Time, string) (*usagestats.UserUsageDetails, error)
}

// GetAdminUserUsageDetails returns seven calendar days (including today), plus
// cumulative usage. The caller supplies a validated IANA timezone.
func (s *UsageService) GetAdminUserUsageDetails(ctx context.Context, userID int64, location *time.Location) (*User, *usagestats.UserUsageDetails, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	repo, ok := s.usageRepo.(userUsageDetailsRepository)
	if !ok {
		return nil, nil, fmt.Errorf("user usage details repository unavailable")
	}
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := today.AddDate(0, 0, -6)
	// Limit reporting time without imposing a timeout on other usage operations.
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	details, err := repo.GetUserUsageDetails(ctx, userID, start, now, location.String())
	if err != nil {
		return nil, nil, err
	}
	details.GeneratedAt = now
	details.StartDate = start.Format(time.DateOnly)
	details.EndDate = today.Format(time.DateOnly)
	details.Timezone = location.String()
	details.Daily = fillUserUsageDays(details.Daily, start)
	for i := range details.APIKeys {
		details.APIKeys[i].Daily = fillUserUsageDays(details.APIKeys[i].Daily, start)
	}
	return user, details, nil
}

func fillUserUsageDays(days []usagestats.UserUsageDay, start time.Time) []usagestats.UserUsageDay {
	byDate := make(map[string]usagestats.UserUsageMetrics, len(days))
	for _, day := range days {
		byDate[day.Date] = day.UserUsageMetrics
	}
	result := make([]usagestats.UserUsageDay, 7)
	for i := range result {
		date := start.AddDate(0, 0, i).Format(time.DateOnly)
		result[i] = usagestats.UserUsageDay{Date: date, UserUsageMetrics: byDate[date]}
	}
	return result
}
