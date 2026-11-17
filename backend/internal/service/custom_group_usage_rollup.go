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

// Group usage reporting follows the dashboard calendar, independent of upstream quota calendars.
func GroupUsageTimezoneName() string {
	return usagestats.DashboardTimezone
}

// GroupUsageTodayStart returns dashboard-calendar midnight as an absolute UTC instant.
func GroupUsageTodayStart(at time.Time) time.Time {
	year, month, day := at.In(usagestats.DashboardLocation()).Date()
	return groupUsageDayStart(year, month, day).UTC()
}

// GroupUsageYesterdayStart 返回指定时刻前一个北京时间日历日的 UTC 起点。
func GroupUsageYesterdayStart(at time.Time) time.Time {
	// 任一本地日至少 22 小时，当日起点前 12 小时必落在前一个日历日内。
	return GroupUsageTodayStart(GroupUsageTodayStart(at).Add(-12 * time.Hour))
}

// GroupUsageDate returns the Beijing reporting date.
func GroupUsageDate(at time.Time) string {
	return at.In(usagestats.DashboardLocation()).Format(groupUsageDateFormat)
}

// ParseGroupUsageDate 解析仪表盘日期并返回该日期的实际起点。
func ParseGroupUsageDate(value string) (time.Time, error) {
	date, err := time.Parse(groupUsageDateFormat, value)
	if err != nil {
		return time.Time{}, err
	}
	year, month, day := date.Date()
	return groupUsageDayStart(year, month, day), nil
}

// groupUsageDayStart 返回日期在仪表盘时区内的实际首个时刻。
// 夏令时在零点跳变的日期，00:00 可能不存在或出现两次，此时返回该日期实际开始的时刻。
func groupUsageDayStart(year int, month time.Month, day int) time.Time {
	loc := usagestats.DashboardLocation()
	target := groupUsageDateOrdinal(year, month, day)
	start := time.Date(year, month, day, 0, 0, 0, 0, loc)
	if localGroupUsageDateOrdinal(start) == target && localGroupUsageDateOrdinal(start.Add(-time.Nanosecond)) < target {
		return start
	}
	// 在 [前一日正午, 当日正午] 内二分出本地日期首次到达目标日期的时刻。
	low := time.Date(year, month, day-1, 12, 0, 0, 0, loc)
	high := time.Date(year, month, day, 12, 0, 0, 0, loc)
	for high.Sub(low) > time.Nanosecond {
		mid := low.Add(high.Sub(low) / 2)
		if localGroupUsageDateOrdinal(mid) >= target {
			high = mid
		} else {
			low = mid
		}
	}
	return high
}

func localGroupUsageDateOrdinal(at time.Time) int {
	year, month, day := at.In(usagestats.DashboardLocation()).Date()
	return groupUsageDateOrdinal(year, month, day)
}

func groupUsageDateOrdinal(year int, month time.Month, day int) int {
	return year*10000 + int(month)*100 + day
}
