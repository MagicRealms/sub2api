package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// TesterMetric contains customer-visible measurements only. Account identities,
// supplier costs, credentials, raw prompts and other users are never selected.
type TesterMetric struct {
	IsTotal              bool     `json:"is_total"`
	Model                string   `json:"model"`
	GroupID              int64    `json:"group_id"`
	GroupName            string   `json:"group_name"`
	Requests             int64    `json:"requests"`
	InputTokens          int64    `json:"input_tokens"`
	OutputTokens         int64    `json:"output_tokens"`
	CacheReadTokens      int64    `json:"cache_read_tokens"`
	CacheCreationTokens  int64    `json:"cache_creation_tokens"`
	TotalCost            float64  `json:"total_cost"`
	ActualCost           float64  `json:"actual_cost"`
	CacheHitRate         float64  `json:"cache_hit_rate"`
	FirstTokenSamples    int64    `json:"first_token_samples"`
	OutputSamples        int64    `json:"output_samples"`
	FirstTokenP50        *float64 `json:"first_token_p50_ms"`
	FirstTokenP95        *float64 `json:"first_token_p95_ms"`
	DurationP50          *float64 `json:"duration_p50_ms"`
	DurationP95          *float64 `json:"duration_p95_ms"`
	OutputTPS            *float64 `json:"output_tokens_per_second"`
	CostPerMillionTokens *float64 `json:"cost_per_million_tokens"`
}

type TesterSnapshot struct {
	GeneratedAt     time.Time      `json:"generated_at"`
	StartTime       time.Time      `json:"start_time"`
	EndTime         time.Time      `json:"end_time"`
	VisibleAfter    *time.Time     `json:"visible_after"`
	Summary         TesterMetric   `json:"summary"`
	Models          []TesterMetric `json:"models"`
	ModelsTruncated bool           `json:"models_truncated"`
	ErrorRecords    int64          `json:"error_records"`
}

func (s *UsageService) TesterHistorySince(ctx context.Context, userID int64) (*time.Time, error) {
	if s == nil || s.entClient == nil || userID <= 0 {
		return nil, infraerrors.ServiceUnavailable("TESTER_UNAVAILABLE", "Tester monitoring is unavailable")
	}
	rows, err := s.entClient.QueryContext(ctx, "SELECT visible_after FROM tester_usage_history WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var since time.Time
	if err := rows.Scan(&since); err != nil {
		return nil, err
	}
	return &since, rows.Err()
}

func (s *UsageService) ResetTesterHistory(ctx context.Context, userID int64) (*time.Time, error) {
	if s == nil || s.entClient == nil || userID <= 0 {
		return nil, infraerrors.ServiceUnavailable("TESTER_UNAVAILABLE", "Tester monitoring is unavailable")
	}
	// Recheck the live role in the atomic write as well as the JWT middleware.
	// Concurrent clears monotonically advance one row and cannot reset money.
	rows, err := s.entClient.QueryContext(ctx, `INSERT INTO tester_usage_history (user_id, visible_after)
SELECT id, clock_timestamp() FROM users
WHERE id = $1 AND role = 'tester' AND status = 'active' AND deleted_at IS NULL
ON CONFLICT (user_id) DO UPDATE SET visible_after = GREATEST(tester_usage_history.visible_after, EXCLUDED.visible_after)
RETURNING visible_after`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, infraerrors.Forbidden("TESTER_REQUIRED", "An active tester account is required")
	}
	var since time.Time
	if err := rows.Scan(&since); err != nil {
		return nil, err
	}
	return &since, rows.Err()
}

const testerMetricsSQL = `WITH scoped AS (
 SELECT COALESCE(NULLIF(ul.model, ''), 'unknown') AS model,
        COALESCE(ul.group_id, 0) AS group_id, COALESCE(g.name, '') AS group_name,
        ul.input_tokens::bigint, ul.output_tokens::bigint,
        ul.cache_read_tokens::bigint, ul.cache_creation_tokens::bigint,
        ul.total_cost, ul.actual_cost, ul.first_token_ms, ul.duration_ms, ul.stream
 FROM usage_logs ul LEFT JOIN groups g ON g.id = ul.group_id
 WHERE ul.user_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3
   AND ($4::bigint = 0 OR ul.api_key_id = $4)
   AND ($5::bigint = 0 OR ul.group_id = $5)
   AND ($6::text = '' OR ul.model = $6)
), metrics AS (
 SELECT GROUPING(model) = 1 AS is_total, COALESCE(model, '') AS model,
        COALESCE(group_id, 0) AS group_id, COALESCE(group_name, '') AS group_name,
        COUNT(*) AS requests, COALESCE(SUM(input_tokens),0) AS input_tokens,
        COALESCE(SUM(output_tokens),0) AS output_tokens,
        COALESCE(SUM(cache_read_tokens),0) AS cache_read_tokens,
        COALESCE(SUM(cache_creation_tokens),0) AS cache_creation_tokens,
        COALESCE(SUM(total_cost),0)::float8 AS total_cost,
        COALESCE(SUM(actual_cost),0)::float8 AS actual_cost,
        COALESCE(100.0 * SUM(cache_read_tokens) / NULLIF(SUM(input_tokens + cache_read_tokens + cache_creation_tokens),0),0)::float8 AS cache_hit_rate,
        COUNT(*) FILTER (WHERE first_token_ms > 0) AS first_token_samples,
        COUNT(*) FILTER (WHERE stream AND first_token_ms > 0 AND duration_ms > first_token_ms AND output_tokens > 0) AS output_samples,
        percentile_cont(0.50) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE first_token_ms > 0) AS first_token_p50_ms,
        percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE first_token_ms > 0) AS first_token_p95_ms,
        percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms > 0) AS duration_p50_ms,
        percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms > 0) AS duration_p95_ms,
        (1000.0 * SUM(output_tokens) FILTER (WHERE stream AND first_token_ms > 0 AND duration_ms > first_token_ms AND output_tokens > 0)
         / NULLIF(SUM(duration_ms::bigint - first_token_ms) FILTER (WHERE stream AND first_token_ms > 0 AND duration_ms > first_token_ms AND output_tokens > 0),0))::float8 AS output_tokens_per_second,
        (1000000.0 * SUM(actual_cost) / NULLIF(SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens),0))::float8 AS cost_per_million_tokens
 FROM scoped GROUP BY GROUPING SETS ((), (model, group_id, group_name))
) SELECT COALESCE(json_agg(t), '[]'::json) FROM
(SELECT * FROM metrics ORDER BY is_total DESC, requests DESC, model, group_id LIMIT 202) t`

func (s *UsageService) TesterMonitoring(ctx context.Context, userID int64, start, end time.Time, keyID, groupID int64, model string) (*TesterSnapshot, error) {
	if userID <= 0 || keyID < 0 || groupID < 0 || start.IsZero() || end.IsZero() || !end.After(start) || end.Sub(start) > 31*24*time.Hour {
		return nil, infraerrors.BadRequest("INVALID_TESTER_RANGE", "Choose a time range of at most 31 days")
	}
	since, err := s.TesterHistorySince(ctx, userID)
	if err != nil {
		return nil, err
	}
	if since != nil && since.After(start) {
		start = *since
	}
	result := &TesterSnapshot{GeneratedAt: time.Now().UTC(), StartTime: start, EndTime: end, VisibleAfter: since, Models: []TesterMetric{}}
	if !end.After(start) {
		return result, nil
	}
	args := []any{userID, start, end, keyID, groupID, model}
	rows, err := s.entClient.QueryContext(ctx, testerMetricsSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("load tester model metrics: %w", err)
	}
	var payload []byte
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err == nil {
			err = fmt.Errorf("missing tester metrics result")
		}
		return nil, err
	}
	err = rows.Scan(&payload)
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	var metrics []TesterMetric
	if err := json.Unmarshal(payload, &metrics); err != nil {
		return nil, err
	}
	for _, metric := range metrics {
		if metric.IsTotal {
			result.Summary = metric
			continue
		}
		if len(result.Models) >= 200 {
			result.ModelsTruncated = true
			continue
		}
		result.Models = append(result.Models, metric)
	}
	// Count only final customer-visible failures. Recovered upstream attempts
	// remain diagnostic events and are not represented as failed conversations.
	rows, err = s.entClient.QueryContext(ctx, `SELECT COUNT(*) FROM ops_error_logs e
WHERE e.user_id = $1 AND e.created_at >= $2 AND e.created_at < $3
 AND ($4::bigint = 0 OR e.api_key_id = $4) AND ($5::bigint = 0 OR e.group_id = $5)
 AND ($6::text = '' OR COALESCE(NULLIF(e.requested_model,''),e.model) = $6)
 AND e.status_code >= 400 AND COALESCE(e.is_count_tokens,false) = false`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("missing tester error count")
	}
	if err := rows.Scan(&result.ErrorRecords); err != nil {
		return nil, err
	}
	return result, rows.Err()
}
