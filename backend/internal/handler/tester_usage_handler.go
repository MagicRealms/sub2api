package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func isTesterContext(c *gin.Context) bool {
	role, ok := middleware.GetUserRoleFromContext(c)
	return ok && role == service.RoleTester
}
func requireTesterSubject(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	if !isTesterContext(c) {
		response.Forbidden(c, "Tester role is required")
		return 0, false
	}
	return subject.UserID, true
}

// No caller-supplied user ID is accepted; the current DB-backed JWT subject
// is always used. Neither endpoint grants access to administrator ops APIs.
func (h *UsageHandler) TesterMonitoring(c *gin.Context) {
	userID, ok := requireTesterSubject(c)
	if !ok {
		return
	}
	filters, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	snapshot, err := h.usageService.TesterMonitoring(ctx, userID, filters.StartTime, filters.EndTime, filters.Filters.APIKeyID, filters.Filters.GroupID, filters.Filters.Model)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}

func (h *UsageHandler) ResetTesterHistory(c *gin.Context) {
	userID, ok := requireTesterSubject(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	since, err := h.usageService.ResetTesterHistory(ctx, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"visible_after":           since,
		"billing_preserved":       true,
		"usage_records_preserved": true,
	})
}
