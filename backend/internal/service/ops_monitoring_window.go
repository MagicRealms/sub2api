package service

import "time"

// clampOpsMonitoringWindow intersects an ops query with the comparison period.
// An entirely historical range becomes empty instead of being shifted forward
// into a different period. Billing and user usage queries never call this helper.
func clampOpsMonitoringWindow(start, end, baseline time.Time) (time.Time, time.Time) {
	if baseline.IsZero() || !start.Before(baseline) {
		return start, end
	}
	if !end.After(baseline) {
		return end, end
	}
	return baseline, end
}
