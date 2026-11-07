//go:build unit

package routes

import (
	"context"
	"encoding/json"
	"fmt"
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
	err    error
}

func (r orderedRouteGroupRepo) GetByID(_ context.Context, id int64) (*service.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.groups[id], nil
}

func orderedTestEngine(t *testing.T, groups []*service.Group, handler gin.HandlerFunc, support ...func(context.Context, *service.Group, string, string) (bool, error)) *gin.Engine {
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
	supports := func(_ context.Context, g *service.Group, model, path string) (bool, error) {
		return g.ModelAllowlist.Allows(model), nil
	}
	if len(support) > 0 {
		supports = support[0]
	}
	engine.Use(orderedKeyGroupRoutingWithSupport(engine, keys, supports))
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

func TestOrderedGroupRoutesUnmatchedModelCannotEscapeSelectedCatalog(t *testing.T) {
	groups := orderedTestGroups()
	for _, group := range groups {
		group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"another-model"}}
	}
	engine := orderedTestEngine(t, groups, func(c *gin.Context) {
		t.Fatal("unsupported models must not fall through to legacy primary routing")
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

func TestOrderedGroupResponsePreservesAlreadyProducedContent(t *testing.T) {
	for name, event := range map[string]string{
		"responses_message":   `{"type":"response.output_item.added","item":{"type":"message","content":[{"type":"output_text","text":"answer"}]}}`,
		"responses_reasoning": `{"type":"response.output_item.added","item":{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}]}}`,
		"content_part":        `{"type":"response.content_part.added","part":{"type":"output_text","text":"answer"}}`,
		"chat_reasoning":      `{"choices":[{"delta":{"reasoning_content":"thinking"}}]}`,
		"anthropic_thinking":  `{"type":"content_block_start","content_block":{"type":"thinking","thinking":"answer"}}`,
		"prefilled_message":   `{"type":"message_start","message":{"content":[{"type":"text","text":"answer"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			target := httptest.NewRecorder()
			w := newOrderedRouteResponse(target)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: " + event + "\n\n"))
			_, _ = w.Write([]byte("data: {\"type\":\"response.failed\"}\n\n"))
			require.False(t, w.canRetry(), "never discard generated text or reasoning")
			w.finish()
			require.Contains(t, target.Body.String(), event)
		})
	}
}

func TestOrderedGroupResponseModelNotFoundMayFailOver(t *testing.T) {
	w := newOrderedRouteResponse(httptest.NewRecorder())
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","code":"model_not_found","message":"The requested model does not exist"}}`))
	require.True(t, w.canRetry())
}

func TestOrderedGroupCredentialMatchesProtocolPrecedence(t *testing.T) {
	for _, tc := range []struct{ path, auth, google, want string }{
		{"/v1beta/models/gemini:generateContent", "Bearer other-key", "google-key", "google-key"},
		{"/antigravity/v1beta/models/gemini:generateContent?key=query-key", "", "", "query-key"},
		{"/v1/responses", "Bearer ", "", "fallback-key"},
		{"/v1/responses", "Bearer bearer-key", "google-key", "bearer-key"},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		req.Header.Set("Authorization", tc.auth)
		req.Header.Set("x-goog-api-key", tc.google)
		if tc.want != "query-key" {
			req.Header.Set("x-api-key", "fallback-key")
		}
		require.Equal(t, tc.want, orderedRouteCredential(req), tc.path)
	}
}

func TestOrderedGroupRoutesLoadOnlyReachedCatalogs(t *testing.T) {
	var checked []int64
	engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}, func(_ context.Context, group *service.Group, _, _ string) (bool, error) {
		checked = append(checked, group.ID)
		if group.ID != 1 {
			return false, fmt.Errorf("unused fallback unavailable")
		}
		return true, nil
	})
	require.Equal(t, http.StatusOK, orderedTestRequest(engine).Code)
	require.Equal(t, []int64{1}, checked)
}

func TestOrderedGroupResponseBoundsSingleChunkBeforeBuffering(t *testing.T) {
	w := newOrderedRouteResponse(httptest.NewRecorder())
	w.Header().Set("Content-Type", "text/event-stream")
	chunk := []byte(strings.Repeat(":", orderedRouteResponseLimit*2))
	_, err := w.Write(chunk)
	require.NoError(t, err)
	require.Zero(t, w.buffer.Cap(), "oversized chunks must go directly to the client")
	require.False(t, w.canRetry())
}

func TestOrderedGroupResponseMultilineFramesAndEmptyChatPrelude(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		retry         bool
	}{
		{"multiline_text", "data: {\n" + "data: \"type\":\"response.output_text.delta\",\n" + "data: \"delta\":\"answer\"}\n\n" + "data: {\"type\":\"response.failed\"}\n\n", false},
		{"multiline_policy", "event: error\ndata: {\n" + "data: \"error\":{\"code\":\"content_policy_violation\"}}\n\n", false},
		{"empty_chat_prelude", "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":null}]}\n\n" + "data: {\"error\":{\"code\":\"server_overloaded\"}}\n\n", true},
		{"recoverable_named_error", "event: error\ndata: {\"error\":{\"code\":\"server_overloaded\"}}\r\n\r\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newOrderedRouteResponse(httptest.NewRecorder())
			w.Header().Set("Content-Type", "text/event-stream")
			// Exercise all possible transport splits without quadratic rescanning.
			for _, b := range []byte(tc.payload) {
				_, err := w.Write([]byte{b})
				require.NoError(t, err)
			}
			require.Equal(t, tc.retry, w.canRetry())
		})
	}
}

func TestOrderedGroupRoutesPricingFailureNeverFallsThroughToFreeRoute(t *testing.T) {
	calls := 0
	engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) { calls++; c.Status(http.StatusOK) },
		func(context.Context, *service.Group, string, string) (bool, error) {
			return true, service.ErrModelPricingUnavailable
		})
	response := orderedTestRequest(engine)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Contains(t, response.Body.String(), "pricing")
	require.Zero(t, calls)
}

func BenchmarkOrderedRoutePrelude(b *testing.B) {
	payload := []byte("data: {\"type\":\"ping\"}\n\n")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := newOrderedRouteResponse(httptest.NewRecorder())
		w.Header().Set("Content-Type", "text/event-stream")
		for j := 0; j < 100; j++ {
			_, _ = w.Write(payload)
		}
	}
}

func TestOrderedGroupRoutePolicyReadFailureFailsClosed(t *testing.T) {
	groups := orderedTestGroups()
	key := &service.APIKey{ID: 10, GroupID: &groups[0].ID, Group: groups[0], GroupIDs: []int64{1, 2}, Status: service.StatusActive, User: &service.User{ID: 9, Status: service.StatusActive}}
	keys := service.NewAPIKeyService(orderedRouteKeyRepo{key: key}, nil, orderedRouteGroupRepo{err: fmt.Errorf("database unavailable")}, nil, nil, nil, &config.Config{})
	engine := gin.New()
	engine.Use(orderedKeyGroupRoutingWithSupport(engine, keys, func(context.Context, *service.Group, string, string) (bool, error) {
		t.Fatal("catalog should not be reached")
		return false, nil
	}))
	engine.POST("/v1/responses", func(c *gin.Context) { t.Fatal("policy read failure must never enter legacy routing") })
	require.Equal(t, http.StatusServiceUnavailable, orderedTestRequest(engine).Code)
}

func TestOrderedGroupNamedPolicyErrorCannotBeRetried(t *testing.T) {
	w := newOrderedRouteResponse(httptest.NewRecorder())
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write([]byte("event: error\ndata: {\"code\":\"content_policy_violation\",\"message\":\"Blocked\"}\n\n"))
	require.False(t, w.canRetry())
}

func TestOrderedGroupRoutingCannotBeDisabledByCosmeticHeadersOrBlankQuery(t *testing.T) {
	for _, tc := range []struct{ name, path, upgrade string }{
		{"blank_query", "/v1/responses?key=%20&api_key=%20", ""},
		{"fake_upgrade", "/v1/responses", "websocket"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []int64
			engine := orderedTestEngine(t, orderedTestGroups(), func(c *gin.Context) {
				key, _ := middleware.GetAPIKeyFromContext(c)
				calls = append(calls, key.Group.ID)
				if key.Group.ID == 1 {
					c.JSON(503, gin.H{"error": "overloaded"})
					return
				}
				c.JSON(200, gin.H{"ok": true})
			})
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"model":"gpt-6.1-sol","input":"test"}`))
			req.Header.Set("Authorization", "Bearer sk-ordered-route-test-key")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Upgrade", tc.upgrade)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			require.Equal(t, 200, response.Code)
			require.Equal(t, []int64{1, 2}, calls)
		})
	}
}

func TestOrderedGroupPrelookupHonorsAuthAbuseAndHeaderLimits(t *testing.T) {
	for _, kind := range []string{"abuse", "oversized_alternate_header"} {
		t.Run(kind, func(t *testing.T) {
			cfg := &config.Config{RunMode: config.RunModeStandard, APIKeyAuth: config.APIKeyAuthCacheConfig{InvalidAbuse: config.InvalidAuthAbuseConfig{Enabled: true, Threshold: 1, WindowSeconds: 60, BlockSeconds: 10, Capacity: 16}}}
			// A nil repository would panic if routing attempted a forbidden lookup.
			keys := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg)
			if kind == "abuse" {
				keys.RecordInvalidAuthFailure("203.0.113.10")
			}
			engine := gin.New()
			engine.Use(orderedKeyGroupRoutingWithSupport(engine, keys, func(context.Context, *service.Group, string, string) (bool, error) {
				t.Fatal("unexpected catalog lookup")
				return false, nil
			}))
			engine.POST("/v1/responses", gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg)), func(c *gin.Context) { t.Fatal("unexpected upstream") })
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-6.1-sol"}`))
			req.RemoteAddr = "203.0.113.10:1234"
			req.Header.Set("Authorization", "Bearer key-that-must-not-be-looked-up")
			want := http.StatusTooManyRequests
			if kind == "oversized_alternate_header" {
				req.Header.Set("x-goog-api-key", strings.Repeat("x", service.MaxAPIKeyCredentialBytes+1))
				want = http.StatusUnauthorized
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			require.Equal(t, want, response.Code)
		})
	}
}
