//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserReportingDateFiltersAlwaysUseBeijing(t *testing.T) {
	for _, zone := range []string{"UTC", "America/Los_Angeles", "Asia/Shanghai", "invalid"} {
		t.Run(zone, func(t *testing.T) {
			repo := &userUsageRepoCapture{}
			router := newUserUsageRequestTypeTestRouter(repo)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage?start_date=2026-11-09&end_date=2026-11-09&timezone="+zone, nil))
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, "2026-11-08T16:00:00Z", repo.listFilters.StartTime.UTC().Format(time.RFC3339))
			require.Equal(t, "2026-11-09T16:00:00Z", repo.listFilters.EndTime.UTC().Format(time.RFC3339))
		})
	}
}
func TestGatewayReportingDatesUseBeijing(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage?start_date=2026-11-09&end_date=2026-11-09&timezone=UTC", nil)
	start, end := (&GatewayHandler{}).parseUsageDateRange(c)
	require.Equal(t, "2026-11-08T16:00:00Z", start.UTC().Format(time.RFC3339))
	require.Equal(t, "2026-11-09T16:00:00Z", end.UTC().Format(time.RFC3339))
}
func TestAPIKeyDailyUsageRangeUsesBeijing(t *testing.T) {
	for _, zone := range []string{"UTC", "America/Los_Angeles", "Asia/Shanghai"} {
		start, end := apiKeyDailyUsageRange(7, zone)
		require.Equal(t, 7*24*time.Hour, end.Sub(start))
		require.Equal(t, 0, start.Hour())
		_, offset := start.Zone()
		require.Equal(t, 8*60*60, offset)
	}
}
