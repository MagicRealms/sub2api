package usagestats

import "time"

// UserUsageMetrics uses the same billable-request and actual-cost definitions as
// the admin dashboard. Totals cover the usage history retained in usage_logs.
type UserUsageMetrics struct {
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	ActualCost          float64 `json:"actual_cost"`
}

func (m *UserUsageMetrics) Add(other UserUsageMetrics) {
	m.Requests += other.Requests
	m.InputTokens += other.InputTokens
	m.OutputTokens += other.OutputTokens
	m.CacheCreationTokens += other.CacheCreationTokens
	m.CacheReadTokens += other.CacheReadTokens
	m.TotalTokens += other.TotalTokens
	m.ActualCost += other.ActualCost
}

type UserUsageDay struct {
	Date string `json:"date"`
	UserUsageMetrics
}

type UserAPIKeyUsage struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	KeyPreview string           `json:"key_preview"`
	Status     string           `json:"status"`
	Deleted    bool             `json:"deleted"`
	GroupName  string           `json:"group_name"`
	ExpiresAt  *time.Time       `json:"expires_at"`
	LastUsedAt *time.Time       `json:"last_used_at"`
	Total      UserUsageMetrics `json:"total"`
	Daily      []UserUsageDay   `json:"daily"`
}

type UserUsageDetails struct {
	GeneratedAt time.Time         `json:"generated_at"`
	StartDate   string            `json:"start_date"`
	EndDate     string            `json:"end_date"`
	Timezone    string            `json:"timezone"`
	Total       UserUsageMetrics  `json:"total"`
	Daily       []UserUsageDay    `json:"daily"`
	APIKeys     []UserAPIKeyUsage `json:"api_keys"`
}
