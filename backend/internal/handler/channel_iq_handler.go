package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelIQHandler exposes the authenticated, read-only channel intelligence
// view. Configuration remains an admin channel setting; this handler only
// returns the small operational status card payload.
type ChannelIQHandler struct {
	service *service.ChannelIQService
}

func NewChannelIQHandler(channelIQService *service.ChannelIQService) *ChannelIQHandler {
	return &ChannelIQHandler{service: channelIQService}
}

// List GET /api/v1/channel-iq
func (h *ChannelIQHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// Run POST /api/v1/channel-iq/:id/run
func (h *ChannelIQHandler) Run(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid channel ID")
		return
	}
	item, err := h.service.Run(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
