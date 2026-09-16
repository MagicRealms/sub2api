//go:build unit

package service

import (
	"testing"
	"time"
)

func TestClampOpsMonitoringWindow(t *testing.T) {
	baseline := time.Date(2026, 9, 16, 12, 30, 0, 0, time.UTC)
	before := baseline.Add(-time.Hour)
	after := baseline.Add(time.Hour)
	for _, tc := range []struct {
		name                                     string
		start, end, baseline, wantStart, wantEnd time.Time
	}{
		{"disabled preserves history", before, after, time.Time{}, before, after},
		{"spanning reset", before, after, baseline, baseline, after},
		{"new requests unchanged", baseline, after, baseline, baseline, after},
		{"later window unchanged", after, after.Add(time.Hour), baseline, after, after.Add(time.Hour)},
		{"entirely old window is empty", before.Add(-time.Hour), before, baseline, before, before},
		{"ending at reset is empty", before, baseline, baseline, baseline, baseline},
		{"offset timestamp", before, after, baseline.In(time.FixedZone("CST", 8*3600)), baseline, after},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, end := clampOpsMonitoringWindow(tc.start, tc.end, tc.baseline)
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) {
				t.Fatalf("got %s..%s, want %s..%s", start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}
