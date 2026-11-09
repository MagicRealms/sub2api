//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStrictPriorityRetry_PrefersNextAccountOnlyForOrdinaryStrictSelection(t *testing.T) {
	err := &service.UpstreamFailoverError{StatusCode: 503, RetryableOnSameAccount: true, RequestScopedTransient: true}
	for _, layer := range []string{"strict_priority", "previous_response_id", "guardian_parent", "load_balance", "session_hash", ""} {
		t.Run(layer, func(t *testing.T) {
			d := service.OpenAIAccountScheduleDecision{Layer: layer}
			require.Equal(t, layer != "strict_priority", sameAccountRetryAllowedForSchedule(err, 0, 3, d))
		})
	}
	err.SameAccountRetryDeadline = time.Now().Add(time.Minute)
	require.False(t, sameAccountRetryAllowedForSchedule(err, 0, 0, service.OpenAIAccountScheduleDecision{Layer: "strict_priority"}), "deadline retries must not override strict ordering")
	require.True(t, sameAccountRetryAllowedForSchedule(err, 0, 0, service.OpenAIAccountScheduleDecision{Layer: "previous_response_id"}), "hard-owned continuation retains its retry window")
	err.RetryableOnSameAccount = false
	require.False(t, sameAccountRetryAllowedForSchedule(err, 0, 3, service.OpenAIAccountScheduleDecision{}))
}

type strictPriorityHandlerUpstream struct {
	service.HTTPUpstream
	mu   sync.Mutex
	hits []int64
}

func (u *strictPriorityHandlerUpstream) Do(req *http.Request, _ string, id int64, _ int) (*http.Response, error) {
	requestBody, _ := io.ReadAll(req.Body)
	u.mu.Lock()
	u.hits = append(u.hits, id)
	u.mu.Unlock()
	status := http.StatusOK
	body := `{"id":"resp_strict_success","object":"response","model":"gpt-5.6-sol","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`
	if strings.Contains(req.URL.Path, "/chat/completions") {
		body = `{"id":"chatcmpl_strict_success","object":"chat.completion","model":"gpt-5.6-sol","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	}
	if id != 803 {
		status = http.StatusServiceUnavailable
		body = `{"error":{"code":"server_is_overloaded","type":"server_error","message":"Our servers are currently overloaded. Please try again later."}}`
	}
	contentType := "application/json"
	if id == 803 && bytes.Contains(requestBody, []byte(`"stream":true`)) {
		body = "data: {\"type\":\"response.completed\",\"response\":" + body + "}\n\n"
		contentType = "text/event-stream"
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestStrictPriorityHTTPHandlers_OverloadSwitchesOncePerAccountInOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ path, body string }{
		{"/responses", `{"model":"gpt-5.6-sol","input":"hello","stream":false}`},
		{"/chat/completions", `{"model":"gpt-5.6-sol","messages":[{"role":"user","content":"hello"}],"stream":false}`},
		{"/messages", `{"model":"gpt-5.6-sol","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			groupID := int64(901)
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Gateway.MaxAccountSwitches = 3
			cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs = []int64{groupID}
			repo := &grokCredentialHandlerRepo{}
			for i := int64(0); i < 3; i++ {
				repo.accounts = append(repo.accounts, service.Account{ID: 801 + i, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 200, Priority: int(i + 1), GroupIDs: []int64{groupID}, Credentials: map[string]any{"api_key": "test-key", "base_url": "https://upstream.example/v1", "pool_mode": true, "pool_mode_retry_count": 3}})
			}
			upstream := &strictPriorityHandlerUpstream{}
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			defer billing.Stop()
			gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
			cache := &concurrencyCacheMock{acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil }, acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil }}
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(cache), billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
			key := &service.APIKey{ID: 902, GroupID: &groupID, User: &service.User{ID: 903, Status: service.StatusActive, Balance: 100}, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, AllowMessagesDispatch: true}}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.User.ID, Concurrency: 20})
				c.Next()
			})
			router.POST("/responses", h.Responses)
			router.POST("/chat/completions", h.ChatCompletions)
			router.POST("/messages", h.Messages)
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, []int64{801, 802, 803}, upstream.hits, "overload must advance once per tier without pool-mode same-account repeats")
		})
	}
}
