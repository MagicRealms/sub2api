package admin

import (
	"testing"
	"time"
)

func TestParseUsageListTimeRangeExactRFC3339(t *testing.T) {
	start, end, err := parseUsageListTimeRange("", "", "2026-11-08T09:00:00Z", "2026-11-08T10:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := start.UTC().Format(time.RFC3339), "2026-11-08T09:00:00Z"; got != want {
		t.Fatalf("start = %s, want %s", got, want)
	}
	if got, want := end.UTC().Format(time.RFC3339), "2026-11-08T10:00:00Z"; got != want {
		t.Fatalf("end = %s, want %s", got, want)
	}
}

func TestParseUsageListTimeRangeRejectsInvalidExactRange(t *testing.T) {
	cases := []struct {
		name, startDate, endDate, startTime, endTime string
	}{
		{name: "missing end", startTime: "2026-11-08T09:00:00Z"},
		{name: "invalid start", startTime: "bad", endTime: "2026-11-08T10:00:00Z"},
		{name: "end before start", startTime: "2026-11-08T10:00:00Z", endTime: "2026-11-08T09:00:00Z"},
		{name: "mixed modes", startDate: "2026-11-08", startTime: "2026-11-08T09:00:00Z", endTime: "2026-11-08T10:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseUsageListTimeRange(tc.startDate, tc.endDate, tc.startTime, tc.endTime, "")
			if err == nil {
				t.Fatal("expected invalid range error")
			}
		})
	}
}

func TestParseUsageListDateRangeAlwaysUsesBeijing(t *testing.T) {
	for _, zone := range []string{"", "UTC", "America/Los_Angeles", "Asia/Shanghai", "invalid-zone"} {
		t.Run(zone, func(t *testing.T) {
			start, end, err := parseUsageListTimeRange("2026-11-09", "2026-11-09", "", "", zone)
			if err != nil {
				t.Fatal(err)
			}
			if got := start.UTC().Format(time.RFC3339); got != "2026-11-08T16:00:00Z" {
				t.Fatalf("start=%s", got)
			}
			if got := end.UTC().Format(time.RFC3339); got != "2026-11-09T16:00:00Z" {
				t.Fatalf("end=%s", got)
			}
		})
	}
}
