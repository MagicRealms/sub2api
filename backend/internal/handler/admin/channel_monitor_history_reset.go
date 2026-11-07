package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *ChannelMonitorHandler) ResetHistory(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if ok {
		h.resetHistory(c, id)
	}
}

func (h *ChannelMonitorHandler) ResetAllHistory(c *gin.Context) {
	h.resetHistory(c, 0)
}

func (h *ChannelMonitorHandler) resetHistory(c *gin.Context, id int64) {
	result, err := executeAdminIdempotent(c, "admin.channel_monitors.reset_history", struct {
		MonitorID int64 `json:"monitor_id"`
	}{id}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.monitorService.ResetHistory(ctx, id)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	response.Success(c, result.Data)
}
