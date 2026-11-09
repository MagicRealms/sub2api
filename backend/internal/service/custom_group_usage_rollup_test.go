//go:build unit

package service

import (
	"testing"
	"time"

	appTimezone "github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func useGroupUsageTestTimezone(t *testing.T, name string) {
	t.Helper()

	previousName := appTimezone.Name()
	require.NoError(t, appTimezone.Init(name))
	t.Cleanup(func() { require.NoError(t, appTimezone.Init(previousName)) })
}

func TestGroupUsageDateUsesBeijingBoundaryRegardlessOfServerTimezone(t *testing.T) {
	useGroupUsageTestTimezone(t, "America/New_York")

	beforeMidnight := time.Date(2026, 3, 8, 15, 59, 59, 0, time.UTC)
	atMidnight := time.Date(2026, 3, 8, 16, 0, 0, 0, time.UTC)

	require.Equal(t, "2026-03-08", GroupUsageDate(beforeMidnight))
	require.Equal(t, "2026-03-09", GroupUsageDate(atMidnight))
	require.Equal(t, atMidnight, GroupUsageTodayStart(atMidnight))
}

func TestGroupUsageParseDateUsesBeijingTimezone(t *testing.T) {
	useGroupUsageTestTimezone(t, "America/New_York")

	parsed, err := ParseGroupUsageDate("2026-03-08")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 3, 7, 16, 0, 0, 0, time.UTC), parsed.UTC())
	require.Equal(t, "Asia/Shanghai", parsed.Location().String())
	require.Equal(t, "America/New_York", appTimezone.Name())
}

func TestGroupUsageYesterdayStartHandlesDST(t *testing.T) {
	useGroupUsageTestTimezone(t, "America/New_York")

	todayStart := time.Date(2026, 3, 8, 16, 0, 0, 0, time.UTC)
	yesterdayStart := GroupUsageYesterdayStart(todayStart)

	require.Equal(t, time.Date(2026, 3, 7, 16, 0, 0, 0, time.UTC), yesterdayStart)
	require.Equal(t, 24*time.Hour, todayStart.Sub(yesterdayStart))
	require.Equal(t, "Asia/Shanghai", GroupUsageTimezoneName())
}
