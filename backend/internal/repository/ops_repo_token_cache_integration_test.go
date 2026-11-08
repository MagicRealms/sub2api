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

func TestOpsTokenCacheHitRateWeightedAndScoped(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID})
	account := mustCreateAccount(t, client, &service.Account{Name: "cache-rate", Platform: service.PlatformOpenAI})
	group := mustCreateGroup(t, client, &service.Group{Name: "cache-rate", Platform: service.PlatformOpenAI})
	other := mustCreateGroup(t, client, &service.Group{Name: "cache-rate-other", Platform: service.PlatformAnthropic})
	start := time.Date(2021, 6, 15, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE id = $1", key.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", account.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id IN ($1,$2)", group.ID, other.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})
	insert := func(model string, input, write, read int64, at time.Time, groupID int64) {
		t.Helper()
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs
 (user_id,api_key_id,account_id,group_id,request_id,model,input_tokens,cache_creation_tokens,cache_read_tokens,created_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, user.ID, key.ID, account.ID, groupID, fmt.Sprintf("cache-%d-%d", user.ID, time.Now().UnixNano()), model, input, write, read, at)
		require.NoError(t, err)
	}
	// Unequal prompt sizes distinguish a weighted ratio from averaging request rates.
	insert("weighted", 10, 0, 90, start, group.ID)
	insert("weighted", 800, 100, 100, start, group.ID)
	insert("empty", 0, 0, 0, start, group.ID)
	insert("cached", 0, 0, 200, start, group.ID)
	// Cast each column before adding, so a row's prompt may exceed int32.
	insert("large", 2_000_000_000, 2_000_000_000, 2_000_000_000, start, group.ID)
	insert("weighted", 0, 0, 999999, start.Add(-time.Second), group.ID)
	insert("weighted", 0, 0, 999999, end, group.ID)
	insert("weighted", 0, 0, 999999, start, other.ID)
	repo := NewOpsRepository(integrationDB).(*opsRepository)
	for _, topN := range []int{20, 0} {
		stats, err := repo.GetOpenAITokenStats(ctx, &service.OpsOpenAITokenStatsFilter{StartTime: start, EndTime: end, Platform: "openai", GroupID: &group.ID, TopN: topN, Page: 1, PageSize: 20})
		require.NoError(t, err)
		require.EqualValues(t, 4, stats.Total)
		require.Len(t, stats.Items, 4)
		byModel := map[string]*service.OpsOpenAITokenStatsItem{}
		for _, row := range stats.Items {
			byModel[row.Model] = row
		}
		require.EqualValues(t, 1100, byModel["weighted"].TotalPromptTokens)
		require.EqualValues(t, 190, byModel["weighted"].TotalCacheReadTokens)
		require.InDelta(t, 190.0/1100*100, byModel["weighted"].CacheHitRate, 1e-9)
		require.Zero(t, byModel["empty"].CacheHitRate)
		require.Equal(t, 100.0, byModel["cached"].CacheHitRate)
		require.EqualValues(t, 6_000_000_000, byModel["large"].TotalPromptTokens)
		require.InDelta(t, 100.0/3, byModel["large"].CacheHitRate, 1e-9)
	}
}
