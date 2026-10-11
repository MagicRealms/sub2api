//go:build integration

package repository

import (
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *UsageLogRepoSuite) TestUserUsageDetailsIsolationAndCalendarDays() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	s.Require().NoError(err)
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, loc)
	today := start.AddDate(0, 0, 6)
	end := today.Add(3 * time.Hour)
	user := mustCreateUser(s.T(), s.client, &service.User{})
	other := mustCreateUser(s.T(), s.client, &service.User{})
	group := mustCreateGroup(s.T(), s.client, &service.Group{Name: "details-group"})
	secret := "sk-details-private-value-never-returned"
	key := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: secret, Name: "used", GroupID: &group.ID})
	unused := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: "tiny", Name: "unused"})
	deleted := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: "sk-deleted-details-private", Name: "deleted"})
	deletedUnused := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: "sk-deleted-unused-details", Name: "deleted-unused"})
	otherKey := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: other.ID, Key: "sk-other-details-private", Name: "other-user"})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "details-account"})
	s.createUsageLog(user, key, account, 10, 1, 1, start.Add(-time.Second))
	s.createUsageLog(user, key, account, 20, 2, 2, start)
	s.createUsageLog(user, key, account, 30, 3, 3, today.Add(-time.Second))
	withCache := s.createUsageLog(user, key, account, 40, 4, 4, today)
	err = s.client.UsageLog.UpdateOneID(withCache.ID).SetCacheCreationTokens(5).SetCacheReadTokens(6).Exec(s.ctx)
	s.Require().NoError(err)
	s.createUsageLog(user, deleted, account, 50, 5, 5, today.Add(time.Hour))
	s.createUsageLog(user, key, account, 100, 100, 100, end) // Exclusive snapshot cutoff.
	s.createUsageLog(other, otherKey, account, 200, 200, 200, today)
	s.createUsageLog(user, key, account, 0, 0, 0, today) // Failed-request placeholder.
	s.Require().NoError(s.client.APIKey.UpdateOneID(deleted.ID).SetDeletedAt(today).Exec(s.ctx))
	s.Require().NoError(s.client.APIKey.UpdateOneID(deletedUnused.ID).SetDeletedAt(today).Exec(s.ctx))

	details, err := s.repo.GetUserUsageDetails(s.ctx, user.ID, start, end, loc.String())
	s.Require().NoError(err)
	s.Equal(int64(5), details.Total.Requests)
	s.Equal(int64(176), details.Total.TotalTokens)
	s.InDelta(15, details.Total.ActualCost, 0.000001)
	s.Len(details.Daily, 3)
	days := map[string]usagestats.UserUsageMetrics{}
	for _, day := range details.Daily {
		days[day.Date] = day.UserUsageMetrics
	}
	s.Equal(int64(1), days["2026-10-05"].Requests)
	s.Equal(int64(1), days["2026-10-10"].Requests)
	s.Equal(int64(2), days["2026-10-11"].Requests)
	s.Equal(int64(110), days["2026-10-11"].TotalTokens)
	s.InDelta(9, days["2026-10-11"].ActualCost, 0.000001)
	keys := map[int64]usagestats.UserAPIKeyUsage{}
	for _, item := range details.APIKeys {
		keys[item.ID] = item
	}
	s.Len(keys, 3)
	s.Equal("details-group", keys[key.ID].GroupName)
	s.Equal("sk-detai…rned", keys[key.ID].KeyPreview)
	s.Equal(int64(4), keys[key.ID].Total.Requests)
	s.Equal(int64(0), keys[unused.ID].Total.Requests)
	s.Equal("••••", keys[unused.ID].KeyPreview)
	s.True(keys[deleted.ID].Deleted)
	s.Require().NotNil(keys[deleted.ID].LastUsedAt)
	s.True(keys[deleted.ID].LastUsedAt.Equal(today.Add(time.Hour)))
	encoded, err := json.Marshal(details)
	s.Require().NoError(err)
	s.NotContains(string(encoded), secret)
	s.NotContains(string(encoded), "other-user")
}

func (s *UsageLogRepoSuite) TestUserUsageDetailsEmptyHistory() {
	user := mustCreateUser(s.T(), s.client, &service.User{})
	end := time.Now().UTC()
	details, err := s.repo.GetUserUsageDetails(s.ctx, user.ID, end.AddDate(0, 0, -6), end, "UTC")
	s.Require().NoError(err)
	s.Zero(details.Total.Requests)
	s.NotNil(details.APIKeys)
	s.Empty(details.APIKeys)
	s.NotNil(details.Daily)
}
