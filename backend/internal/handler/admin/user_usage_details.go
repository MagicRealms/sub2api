package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/gin-gonic/gin"
)

// GetUserUsageDetails handles GET /api/v1/admin/users/:id/usage-details.
func (h *UserHandler) GetUserUsageDetails(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	location := usagestats.DashboardLocation()
	user, details, err := h.usageService.GetAdminUserUsageDetails(c.Request.Context(), userID, location)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, struct {
		User *dto.AdminUser `json:"user"`
		*usagestats.UserUsageDetails
	}{User: dto.UserFromServiceAdmin(user), UserUsageDetails: details})
}
