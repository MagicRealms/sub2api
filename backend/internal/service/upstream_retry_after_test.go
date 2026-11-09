package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCloudflareProblemRetryAfterBoundedAndExplicit(t *testing.T) {
	for _, tc := range []struct {
		name, body, existing, want string
		status                     int
	}{
		{"524", `{"cloudflare_error":true,"retryable":true,"status":524,"retry_after":120}`, "", "120", 524},
		{"502", `{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":60}`, "", "60", 502},
		{"header_wins", `{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":60}`, "17", "17", 502},
		{"unbounded", `{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":301}`, "", "", 502},
		{"fraction", `{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":1.5}`, "", "", 502},
		{"negative", `{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":-3}`, "", "", 502},
		{"string", `{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":"60"}`, "", "", 502},
		{"wrong_status", `{"cloudflare_error":true,"retryable":true,"status":524,"retry_after":60}`, "", "", 502},
		{"not_retryable", `{"cloudflare_error":true,"retryable":false,"status":502,"retry_after":60}`, "", "", 502},
		{"echo_only", `{"error":{"message":"bad","input":{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":60}}}`, "", "", 502},
		{"policy", `{"cloudflare_error":true,"retryable":true,"status":400,"retry_after":60}`, "", "", 400},
		{"html", `error code: 524`, "", "", 524},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{"Server": []string{"cloudflare"}}
			if tc.existing != "" {
				h.Set("Retry-After", tc.existing)
			}
			err := newOpenAIUpstreamFailoverError(tc.status, h, []byte(tc.body), "", false)
			require.Equal(t, tc.want, err.ResponseHeaders.Get("Retry-After"))
			require.Equal(t, tc.existing, h.Get("Retry-After"), "do not mutate the received response")
			require.Equal(t, tc.status, err.StatusCode)
			require.Equal(t, []byte(tc.body), err.ResponseBody)
			require.False(t, err.RetryableOnSameAccount)
		})
	}
	require.Nil(t, upstreamFailoverResponseHeaders(502, nil, []byte(`{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":60}`)))
}

func TestAnthropicForwardPathsPreserveProblemRetryAfter(t *testing.T) {
	for _, endpoint := range []string{"messages", "responses", "chat/completions"} {
		t.Run(endpoint, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}],"stream":true}`)
			if endpoint == "responses" {
				body = []byte(`{"model":"claude-sonnet-4-6","input":"hi","stream":true}`)
			}
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, strings.NewReader(string(body)))
			upstreamBody := `{"cloudflare_error":true,"retryable":true,"status":502,"retry_after":60}`
			resp := &http.Response{StatusCode: 502, Header: http.Header{"Server": []string{"cloudflare"}, "Content-Type": []string{"application/problem+json"}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}
			svc := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: &anthropicHTTPUpstreamRecorder{resp: resp}, rateLimitService: &RateLimitService{accountRepo: &errorPolicyRepoStub{}, cfg: &config.Config{}}}
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "custom_error_codes_enabled": true, "custom_error_codes": []any{502}}}
			var result *ForwardResult
			var err error
			switch endpoint {
			case "responses":
				result, err = svc.ForwardAsResponses(context.Background(), c, account, body, nil)
			case "chat/completions":
				result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
			default:
				var parsed *ParsedRequest
				parsed, err = ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
				require.NoError(t, err)
				result, err = svc.Forward(context.Background(), c, account, parsed)
			}
			var failure *UpstreamFailoverError
			require.ErrorAs(t, err, &failure, fmt.Sprintf("%s: %v", endpoint, err))
			require.Nil(t, result)
			require.Equal(t, "60", failure.ResponseHeaders.Get("Retry-After"))
			require.False(t, c.Writer.Written())
		})
	}
}
