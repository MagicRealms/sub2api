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

func TestDashboardMonthUsesBeijingMonthStart(t *testing.T) {
	at := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.FixedZone("PDT", -8*60*60))
	month := DashboardMonth(at)
	if month.Format("2006-01-02 15:04:05") != "2026-11-01 00:00:00" || month.Location().String() != DashboardTimezone {
		t.Fatalf("wrong reporting month: %v", month)
	}
	if month.UTC().Format(time.RFC3339) != "2026-10-31T16:00:00Z" {
		t.Fatalf("wrong UTC boundary: %v", month)
	}
}
