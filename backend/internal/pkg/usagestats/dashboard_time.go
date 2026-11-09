package usagestats

import "time"

// Dashboard reporting has its own calendar, independent of upstream quota resets.
const DashboardTimezone = "Asia/Shanghai"

var dashboardLocation = time.FixedZone(DashboardTimezone, 8*60*60)

// DashboardLocation is for display/reporting calendars, never quota resets.
func DashboardLocation() *time.Location { return dashboardLocation }

func DashboardDay(at time.Time) time.Time {
	local := at.In(dashboardLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, dashboardLocation)
}
