//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTesterEndpointsRejectNonTesterRolesBeforeStorage(t *testing.T) {
	for _, endpoint := range []string{"monitor", "reset"} {
		for _, role := range []string{"", service.RoleUser, service.RoleAdmin, service.RoleTester} {
			t.Run(endpoint+"/"+role, func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/tester/"+endpoint+"?user_id=999", strings.NewReader(`{"user_id":999,"role":"admin"}`))
				if role != "" {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
					c.Set(string(middleware.ContextKeyUserRole), role)
				}
				h := NewUsageHandler(nil, nil, nil, nil)
				if endpoint == "monitor" {
					h.TesterMonitoring(c)
				} else {
					h.ResetTesterHistory(c)
				}
				want := http.StatusForbidden
				if role == "" {
					want = http.StatusUnauthorized
				}
				if role == service.RoleTester {
					want = http.StatusServiceUnavailable
				}
				require.Equal(t, want, rec.Code)
			})
		}
	}
}

func TestUsageHistoryIgnoresTesterMonitoringReset(t *testing.T) {
	// Usage history must not consult tester_usage_history, even for old clients
	// that still send test_session=true. A nil service fails if it is consulted.
	for _, role := range []string{service.RoleUser, service.RoleTester} {
		for _, requireRange := range []bool{false, true} {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/usage?test_session=true&start_time=2026-11-01T00:00:00Z&end_time=2026-11-02T00:00:00Z", nil)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			c.Set(string(middleware.ContextKeyUserRole), role)
			h := NewUsageHandler(nil, nil, nil, nil)
			parsed, ok := h.parseUserUsageFilters(c, requireRange)
			require.True(t, ok)
			require.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), *parsed.Filters.StartTime)
			require.Equal(t, int64(7), parsed.Filters.UserID)
		}
	}
}
