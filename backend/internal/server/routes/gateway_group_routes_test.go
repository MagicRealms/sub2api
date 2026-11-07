//go:build unit

package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type orderedRouteKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r orderedRouteKeyRepo) GetByKeyForAuth(context.Context, string) (*service.APIKey, error) {
	return r.key, nil
}
func (r orderedRouteKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error { return nil }

type orderedRouteGroupRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r orderedRouteGroupRepo) GetByID(_ context.Context, id int64) (*service.Group, error) {
	return r.groups[id], nil
}

func orderedTestEngine(t *testing.T, groups []*service.Group, handler gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ids := make([]int64, 0, len(groups))
	byID := make(map[int64]*service.Group)
	for _, group := range groups {
		ids = append(ids, group.ID)
		byID[group.ID] = group
	}
	key := &service.APIKey{ID: 10, UserID: 9, GroupID: &groups[0].ID, GroupIDs: ids, Group: groups[0], Status: service.StatusActive, User: &service.User{ID: 9, Status: service.StatusActive, Balance: 100}}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	keys := service.NewAPIKeyService(orderedRouteKeyRepo{key: key}, nil, orderedRouteGroupRepo{groups: byID}, nil, nil, nil, cfg)
	engine := gin.New()
	engine.Use(orderedKeyGroupRoutingWithSupport(engine, keys, func(_ context.Context, g *service.Group, model, path string) (bool, error) {
		return g.ModelAllowlist.Allows(model), nil
	}))
	engine.POST("/v1/responses", gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg)), handler)
	return engine
}

func orderedTestGroups() []*service.Group {
	return []*service.Group{{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: .32}, {ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: .13}}
}
func orderedTestRequest(engine *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-6.1-sol","input":"original prompt"}`))
	req.Header.Set("Authorization", "Bearer sk-ordered-route-test-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Group-ID", "999")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	return response
}

func TestOrderedGroupRoutesNeverRetryPolicyOrQuotaErrors(t *testing.T) {
	for _, body := range []string{
		`{"error":{"type":"api_error","code":"content_policy_violation","message":"Blocked"}}`,
		"event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"safety_violation\"}}}\n\n",
		`{"error":{"type":"api_key_quota_exceeded","message":"Quota exhausted"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) {
				calls++
				if strings.HasPrefix(body, "event:") {
					c.Header("Content-Type", "text/event-stream")
					c.Writer.WriteHeader(http.StatusOK)
				} else {
					c.Writer.WriteHeader(http.StatusServiceUnavailable)
				}
				_, _ = c.Writer.Write([]byte(body))
			})
			orderedTestRequest(engine)
			require.Equal(t, 1, calls)
		})
	}
}

func TestOrderedGroupRoutesRejectAmbiguousModelFields(t *testing.T) {
	calls := 0
	engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) { calls++ })
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-6.1-sol","model":"gpt-image-2"}`))
	req.Header.Set("Authorization", "Bearer sk-ordered-route-test-key")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Zero(t, calls)
}

func TestOrderedGroupRoutesUnmatchedModelPreservesBodyForNormalValidation(t *testing.T) {
	groups := orderedTestGroups()
	for _, group := range groups {
		group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"another-model"}}
	}
	engine := orderedTestEngine(t, groups, func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "original prompt")
		c.Status(http.StatusNotFound)
	})
	require.Equal(t, http.StatusNotFound, orderedTestRequest(engine).Code)
}

func TestOrderedGroupRoutesSelectCrossPlatformModelWithoutUnrelatedFloor(t *testing.T) {
	groups := orderedTestGroups()
	groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-*"}}
	groups[1].Platform = service.PlatformAnthropic
	groups[1].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"claude-*"}}
	engine := orderedTestEngine(t, groups, func(c *gin.Context) {
		key, ok := middleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, service.PlatformAnthropic, key.Group.Platform)
		require.Equal(t, int64(2), key.Group.ID)
		require.Equal(t, int64(2), key.RoutingBillingFloor.ID)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"claude-sonnet-4-6","input":"test"}`))
	req.Header.Set("Authorization", "Bearer sk-ordered-route-test-key")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	require.Equal(t, http.StatusOK, response.Code)
}

func TestOrderedGroupRoutesRetryInSelectionOrderWithFreshAuth(t *testing.T) {
	var calls []int64
	engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) {
		key, _ := middleware.GetAPIKeyFromContext(c)
		calls = append(calls, key.Group.ID)
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "original prompt")
		if key.Group.ID == 1 {
			c.JSON(503, gin.H{"error": gin.H{"type": "api_error", "message": "upstream overloaded"}})
			return
		}
		require.NotNil(t, key.RoutingBillingFloor)
		require.Equal(t, int64(1), key.RoutingBillingFloor.ID)
		c.JSON(200, gin.H{"group_id": key.Group.ID})
	})
	response := orderedTestRequest(engine)
	require.Equal(t, 200, response.Code)
	require.Equal(t, []int64{1, 2}, calls)
	var result map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, float64(2), result["group_id"])
}

func TestOrderedGroupRoutesSkipUnsupportedModelAndSetIndependentFloor(t *testing.T) {
	groups := orderedTestGroups()
	groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"claude-*"}}
	var calls []int64
	engine := orderedTestEngine(t, groups, func(c *gin.Context) {
		key, _ := middleware.GetAPIKeyFromContext(c)
		calls = append(calls, key.Group.ID)
		require.Equal(t, int64(2), key.RoutingBillingFloor.ID)
		c.JSON(200, gin.H{"ok": true})
	})
	response := orderedTestRequest(engine)
	require.Equal(t, 200, response.Code)
	require.Equal(t, []int64{2}, calls)
}

func TestOrderedGroupRoutesDoNotEscalateGroupPermission(t *testing.T) {
	groups := orderedTestGroups()
	groups[1].IsExclusive = true
	var calls []int64
	engine := orderedTestEngine(t, groups, func(c *gin.Context) {
		key, _ := middleware.GetAPIKeyFromContext(c)
		calls = append(calls, key.Group.ID)
		c.JSON(503, gin.H{"error": "unavailable"})
	})
	response := orderedTestRequest(engine)
	require.Equal(t, 503, response.Code)
	require.Equal(t, []int64{1}, calls)
}

func TestOrderedGroupRoutesNeverReplaySemanticOutputOrBillableAttempt(t *testing.T) {
	for _, kind := range []string{"text", "tool", "billed"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) {
				calls++
				if kind == "billed" {
					service.MarkAPIKeyGroupRouteBilled(c.Request.Context())
					c.JSON(503, gin.H{"error": "already billed"})
					return
				}
				c.Header("Content-Type", "text/event-stream")
				payload := `{"type":"response.output_text.delta","delta":"hello"}`
				if kind == "tool" {
					payload = `{"type":"response.output_item.added","item":{"type":"function_call","name":"edit_file"}}`
				}
				_, _ = c.Writer.Write([]byte("data: " + payload + "\n\n"))
				c.Writer.Flush()
				_, _ = c.Writer.Write([]byte("data: {\"type\":\"response.failed\"}\n\n"))
			})
			orderedTestRequest(engine)
			require.Equal(t, 1, calls)
		})
	}
}

func TestOrderedGroupRoutesReplaceFailedSSEPreludeWithoutRepeatingOutput(t *testing.T) {
	calls := 0
	engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) {
		calls++
		c.Header("Content-Type", "text/event-stream")
		if calls == 1 {
			_, _ = c.Writer.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"failed-attempt\"}}\n\n"))
			c.Writer.Flush()
			_, _ = c.Writer.Write([]byte("data: {\"type\":\"response.failed\"}\n\ndata: [DONE]\n\n"))
			return
		}
		_, _ = c.Writer.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"success\"}\n\n"))
	})
	response := orderedTestRequest(engine)
	require.Equal(t, 2, calls)
	require.Contains(t, response.Body.String(), "success")
	require.NotContains(t, response.Body.String(), "failed-attempt")
	require.NotContains(t, response.Body.String(), "response.failed")
}
