package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelIQHandler manages the single-key intelligence probe configuration.
type ChannelIQHandler struct {
	service *service.ChannelIQService
}

func NewChannelIQHandler(channelIQService *service.ChannelIQService) *ChannelIQHandler {
	return &ChannelIQHandler{service: channelIQService}
}

// Get GET /admin/channel-iq
func (h *ChannelIQHandler) Get(c *gin.Context) {
	snapshot, err := h.service.GetAdminSnapshot(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}

// Save PUT /admin/channel-iq
func (h *ChannelIQHandler) Save(c *gin.Context) {
	var input service.ChannelIQSaveInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	snapshot, err := h.service.SaveConfig(c.Request.Context(), input)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, snapshot)
}

// Run POST /admin/channel-iq/run. This is intentionally admin-only; public
// users only see the persisted cards returned by the user handler.
func (h *ChannelIQHandler) Run(c *gin.Context) {
	result, err := h.service.RunNow(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
