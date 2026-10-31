package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelIQHandler exposes the authenticated, read-only intelligence cards.
// Customers never receive a mutation endpoint or the selected upstream key.
type ChannelIQHandler struct {
	service *service.ChannelIQService
}

func NewChannelIQHandler(channelIQService *service.ChannelIQService) *ChannelIQHandler {
	return &ChannelIQHandler{service: channelIQService}
}

// List GET /api/v1/channel-iq
func (h *ChannelIQHandler) List(c *gin.Context) {
	snapshot, err := h.service.GetPublicSnapshot(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}
