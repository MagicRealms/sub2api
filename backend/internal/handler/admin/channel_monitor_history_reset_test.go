//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type historyResetHandlerRepo struct {
	service.ChannelMonitorRepository
	calls int
	id    int64
}

func (r *historyResetHandlerRepo) GetByID(context.Context, int64) (*service.ChannelMonitor, error) {
	return &service.ChannelMonitor{ID: 42}, nil
}

func (r *historyResetHandlerRepo) ResetHistory(_ context.Context, id int64) (*service.ChannelMonitorHistoryResetResult, error) {
	r.calls++
	r.id = id
	return &service.ChannelMonitorHistoryResetResult{MonitorsReset: 1, HistoryDeleted: 4}, nil
}

func TestChannelMonitorHistoryResetReplaysWithoutClearingNewRecords(t *testing.T) {
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(newMemoryIdempotencyRepoStub(), service.DefaultIdempotencyConfig()))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 77})
		c.Next()
	})
	repo := &historyResetHandlerRepo{}
	h := NewChannelMonitorHandler(service.NewChannelMonitorService(repo, nil))
	router.POST("/api/v1/admin/channel-monitors/:id/reset-history", h.ResetHistory)
	router.POST("/api/v1/admin/channel-monitors/reset-history", h.ResetAllHistory)
	for _, path := range []string{"/api/v1/admin/channel-monitors/42/reset-history", "/api/v1/admin/channel-monitors/reset-history"} {
		before := repo.calls
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.Header.Set("Idempotency-Key", "reset-"+strings.ReplaceAll(path, "/", "-"))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), `"history_deleted":4`)
			if i == 1 {
				require.Equal(t, "true", w.Header().Get("X-Idempotency-Replayed"))
			}
		}
		require.Equal(t, before+1, repo.calls)
	}
	require.Zero(t, repo.id)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/channel-monitors/0/reset-history", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}
