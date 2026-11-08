package usagestats

import (
	"testing"
	"time"
)

func TestDashboardDayUsesBeijingMidnight(t *testing.T) {
	for _, tc := range []struct{ instant, day string }{
		{"2026-11-08T15:59:59Z", "2026-11-08"},
		{"2026-11-08T16:00:00Z", "2026-11-09"},
		{"2026-03-08T16:00:00Z", "2026-03-09"},
	} {
		at, _ := time.Parse(time.RFC3339, tc.instant)
		day := DashboardDay(at)
		if day.Format("2006-01-02") != tc.day || day.Hour() != 0 || day.Location().String() != DashboardTimezone {
			t.Fatalf("wrong reporting day: %v", day)
		}
		if day.UTC().Hour() != 16 {
			t.Fatalf("wrong UTC boundary: %v", day)
		}
	}
}
