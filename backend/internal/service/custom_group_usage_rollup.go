package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const groupUsageDateFormat = "2006-01-02"

// GroupUsageRollupRepository 是分组日汇总的可选持久化能力。
type GroupUsageRollupRepository interface {
	SyncGroupUsageRollups(ctx context.Context, todayStart time.Time) error
}

// Group usage is display reporting, independent of upstream quota calendars.
func GroupUsageTimezoneName() string {
	return usagestats.DashboardTimezone
}

// GroupUsageTodayStart returns Beijing midnight as an absolute UTC instant.
func GroupUsageTodayStart(at time.Time) time.Time {
	return usagestats.DashboardDay(at).UTC()
}

// GroupUsageYesterdayStart 返回指定时刻前一个北京时间日历日的 UTC 起点。
func GroupUsageYesterdayStart(at time.Time) time.Time {
	return usagestats.DashboardDay(at).AddDate(0, 0, -1).UTC()
}

// GroupUsageDate returns the Beijing reporting date.
func GroupUsageDate(at time.Time) string {
	return at.In(usagestats.DashboardLocation()).Format(groupUsageDateFormat)
}

// ParseGroupUsageDate parses a Beijing reporting date.
func ParseGroupUsageDate(value string) (time.Time, error) {
	return time.ParseInLocation(groupUsageDateFormat, value, usagestats.DashboardLocation())
}
