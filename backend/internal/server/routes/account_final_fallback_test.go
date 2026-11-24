//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFinalAccountFallbackRunsAfterAllNormalGroups(t *testing.T) {
	groups := orderedTestGroups()
	var calls []int64
	engine := orderedTestEngineWithFallback(t, groups, func(c *gin.Context) {
		key, ok := middleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		ctx := c.Request.Context()
		if service.IsAccountFinalFallbackAttempt(ctx) {
			calls = append(calls, -*key.GroupID)
			require.Equal(t, groups[0].ID, key.RoutingBillingFloor.ID)
			service.RecordAccountFallbackAttempt(ctx, 90, service.PlatformOpenAI)
			service.MarkAPIKeyGroupRouteBilled(ctx)
			c.JSON(http.StatusOK, gin.H{"result": "reserve"})
			return
		}
		calls = append(calls, *key.GroupID)
		service.MarkAccountFinalFallbackUpstreamFailure(ctx, &service.UpstreamFailoverError{StatusCode: 503})
		c.Header("Retry-After", "10")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"type": "upstream_error"}})
	}, func(_ context.Context, ids []int64) (bool, error) {
		return slices.Contains(ids, groups[1].ID), nil
	})
	res := orderedTestRequest(engine)
	require.Equal(t, []int64{1, 2, -2}, calls)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.JSONEq(t, `{"result":"reserve"}`, res.Body.String())
	require.Empty(t, res.Header().Get("Retry-After"))
}

func TestFinalAccountFallbackSkipsUnsafeReplays(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		fail func(*gin.Context)
		code int
	}{
		{"ordinary success", `{"model":"gpt-6.1-sol"}`, func(c *gin.Context) { c.JSON(200, gin.H{"result": "normal"}) }, 200},
		{"failure class unknown", `{"model":"gpt-6.1-sol"}`, func(c *gin.Context) { c.JSON(503, gin.H{"error": "internal"}) }, 503},
		{"request scoped", `{"model":"gpt-6.1-sol"}`, func(c *gin.Context) {
			service.MarkAccountFinalFallbackUpstreamFailure(c.Request.Context(), &service.UpstreamFailoverError{StatusCode: 503, Scope: service.GatewayFailureScopeRequest})
			c.JSON(503, gin.H{"error": "request failed"})
		}, 503},
		{"prior response", `{"model":"gpt-6.1-sol","previous_response_id":"resp_previous"}`, func(c *gin.Context) { c.JSON(503, gin.H{"error": "unavailable"}) }, 503},
		{"already billed", `{"model":"gpt-6.1-sol"}`, func(c *gin.Context) {
			service.MarkAPIKeyGroupRouteBilled(c.Request.Context())
			c.JSON(503, gin.H{"error": "upstream after usage"})
		}, 503},
		{"customer quota", `{"model":"gpt-6.1-sol"}`, func(c *gin.Context) {
			c.JSON(429, gin.H{"error": gin.H{"code": "api_key_quota_exhausted"}})
		}, 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groups := orderedTestGroups()[:1]
			calls := 0
			engine := orderedTestEngineWithFallback(t, groups, func(c *gin.Context) {
				require.False(t, service.IsAccountFinalFallbackAttempt(c.Request.Context()))
				calls++
				if tc.name != "failure class unknown" && tc.name != "request scoped" {
					service.MarkAccountFinalFallbackEligible(c.Request.Context())
				}
				tc.fail(c)
			}, func(context.Context, []int64) (bool, error) { return true, nil })
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer sk-final-fallback-test-key")
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			engine.ServeHTTP(res, req)
			require.Equal(t, tc.code, res.Code, res.Body.String())
			require.Equal(t, 1, calls)
		})
	}
}
