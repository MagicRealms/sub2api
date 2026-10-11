package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// GetUserUsageDetails aggregates just this user's history in one query. The
// cumulative pass groups by key only, avoiding a sort of the entire history.
// Only seven days are expanded; both passes share the same database snapshot.
func (r *usageLogRepository) GetUserUsageDetails(ctx context.Context, userID int64, start, end time.Time, timezone string) (*usagestats.UserUsageDetails, error) {
	query := `WITH cumulative_usage AS (
		SELECT ul.api_key_id, '' AS day,
			COUNT(*) AS requests,
			COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(ul.actual_cost), 0) AS actual_cost,
			MAX(ul.created_at) AS last_used_at
		FROM usage_logs ul
		WHERE ul.user_id = $1 AND ul.created_at < $3 AND ` + usageLogSuccessFilterUL + `
		GROUP BY ul.api_key_id
	), daily_usage AS (
		SELECT ul.api_key_id,
			TO_CHAR(ul.created_at AT TIME ZONE $4, 'YYYY-MM-DD') AS day,
			COUNT(*) AS requests,
			COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(ul.actual_cost), 0) AS actual_cost,
			MAX(ul.created_at) AS last_used_at
		FROM usage_logs ul
		WHERE ul.user_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3 AND ` + usageLogSuccessFilterUL + `
		GROUP BY ul.api_key_id, day
	), usage_by_key_day AS (
		SELECT * FROM cumulative_usage UNION ALL SELECT * FROM daily_usage
	), user_keys AS (
		SELECT id, name,
			CASE WHEN LENGTH(key) > 16 THEN LEFT(key, 8) || '…' || RIGHT(key, 4) ELSE '••••' END AS key_preview,
			status, deleted_at, expires_at, last_used_at, group_id
		FROM api_keys WHERE user_id = $1
	)
	SELECT COALESCE(u.api_key_id, k.id), COALESCE(k.name, ''), COALESCE(k.key_preview, ''),
		COALESCE(k.status, 'deleted'), (k.id IS NULL OR k.deleted_at IS NOT NULL), COALESCE(g.name, ''),
		k.expires_at, GREATEST(k.last_used_at, u.last_used_at), COALESCE(u.day, ''),
		COALESCE(u.requests, 0), COALESCE(u.input_tokens, 0), COALESCE(u.output_tokens, 0),
		COALESCE(u.cache_creation_tokens, 0), COALESCE(u.cache_read_tokens, 0), COALESCE(u.actual_cost, 0)
	FROM usage_by_key_day u FULL OUTER JOIN user_keys k ON k.id = u.api_key_id
	LEFT JOIN groups g ON g.id = k.group_id
	WHERE k.deleted_at IS NULL OR u.api_key_id IS NOT NULL`
	rows, err := r.sql.QueryContext(ctx, query, userID, start, end, timezone)
	if err != nil {
		return nil, fmt.Errorf("query user usage details: %w", err)
	}
	defer rows.Close()
	details := &usagestats.UserUsageDetails{Daily: []usagestats.UserUsageDay{}, APIKeys: []usagestats.UserAPIKeyUsage{}}
	keys := make(map[int64]*usagestats.UserAPIKeyUsage)
	daily := make(map[string]usagestats.UserUsageMetrics)
	for rows.Next() {
		var key usagestats.UserAPIKeyUsage
		var expires, lastUsed sql.NullTime
		var day string
		var metrics usagestats.UserUsageMetrics
		if err := rows.Scan(&key.ID, &key.Name, &key.KeyPreview, &key.Status, &key.Deleted, &key.GroupName,
			&expires, &lastUsed, &day, &metrics.Requests, &metrics.InputTokens, &metrics.OutputTokens,
			&metrics.CacheCreationTokens, &metrics.CacheReadTokens, &metrics.ActualCost); err != nil {
			return nil, fmt.Errorf("scan user usage details: %w", err)
		}
		metrics.TotalTokens = metrics.InputTokens + metrics.OutputTokens + metrics.CacheCreationTokens + metrics.CacheReadTokens
		stored, exists := keys[key.ID]
		if !exists {
			key.Daily = []usagestats.UserUsageDay{}
			if expires.Valid {
				key.ExpiresAt = &expires.Time
			}
			stored = &key
			keys[key.ID] = stored
		}
		if lastUsed.Valid && (stored.LastUsedAt == nil || lastUsed.Time.After(*stored.LastUsedAt)) {
			value := lastUsed.Time
			stored.LastUsedAt = &value
		}
		if day == "" {
			stored.Total.Add(metrics)
			details.Total.Add(metrics)
		} else {
			stored.Daily = append(stored.Daily, usagestats.UserUsageDay{Date: day, UserUsageMetrics: metrics})
			dayMetrics := daily[day]
			dayMetrics.Add(metrics)
			daily[day] = dayMetrics
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read user usage details: %w", err)
	}
	for _, key := range keys {
		details.APIKeys = append(details.APIKeys, *key)
	}
	sort.Slice(details.APIKeys, func(i, j int) bool {
		a, b := details.APIKeys[i], details.APIKeys[j]
		if a.Total.ActualCost != b.Total.ActualCost {
			return a.Total.ActualCost > b.Total.ActualCost
		}
		return a.ID < b.ID
	})
	for date, metrics := range daily {
		details.Daily = append(details.Daily, usagestats.UserUsageDay{Date: date, UserUsageMetrics: metrics})
	}
	return details, nil
}
