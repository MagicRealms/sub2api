package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func alphaAPIKeyFallbackAccount() *Account {
	return &Account{ID: 1400, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-fixture", "base_url": "https://relay.example/v1",
			"model_mapping": map[string]any{"client-model": "upstream-model"}}}
}

func alphaAPIKeyFallbackResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(body))}
}

func alphaAPIKeyFallbackContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, []byte) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	body := []byte(`{"id":"client-session","model":"client-model","commands":{"search_query":[{"q":"OpenAI documentation"}]}}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(body))
	return c, rec, body
}

func TestAlphaSearchAPIKeyMissingEndpointUsesResponsesAndCachesCapability(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				alphaAPIKeyFallbackResponse(status, `{"error":{"message":"Invalid URL (POST /v1/alpha/search)"}}`),
				alphaAPIKeyFallbackResponse(http.StatusOK, alphaSearchResponsesSSE("fresh search results")),
				alphaAPIKeyFallbackResponse(http.StatusOK, alphaSearchResponsesSSE("second search results")),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := alphaAPIKeyFallbackAccount()
			for i := 0; i < 2; i++ {
				c, rec, body := alphaAPIKeyFallbackContext(t)
				result, err := svc.ForwardAlphaSearch(context.Background(), c, account, body)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Contains(t, rec.Body.String(), "search results")
				require.Equal(t, "https://example.com/news", gjson.GetBytes(rec.Body.Bytes(), "results.0.url").String())
				require.Equal(t, "client-model", result.Model)
				require.Equal(t, "upstream-model", result.UpstreamModel)
				require.Equal(t, "/v1/responses", result.UpstreamEndpoint)
				require.Equal(t, 1, result.WebSearchCalls)
				require.Empty(t, GetOpsStreamErrors(c))
			}
			require.Len(t, upstream.requests, 3, "only the first request should probe the missing endpoint")
			require.Equal(t, "https://relay.example/v1/alpha/search", upstream.requests[0].URL.String())
			for i := 1; i < 3; i++ {
				request := upstream.requests[i]
				require.Equal(t, "https://relay.example/v1/responses", request.URL.String())
				require.NotEqual(t, "chatgpt.com", request.Host)
				require.Equal(t, "Bearer sk-fixture", request.Header.Get("Authorization"))
				require.Empty(t, request.Header.Get("Chatgpt-Account-Id"))
				require.Empty(t, request.Header.Get("OpenAI-Beta"))
				require.Empty(t, request.Header.Get("Session_ID"))
				require.Empty(t, request.Header.Get("Conversation_ID"))
				require.Equal(t, "upstream-model", gjson.GetBytes(upstream.bodies[i], "model").String())
				require.Equal(t, "web_search", gjson.GetBytes(upstream.bodies[i], "tools.0.type").String())
			}
		})
	}
}

func TestAlphaSearchAPIKeyNativeEndpointStillUsesOriginalProtocol(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: alphaAPIKeyFallbackResponse(http.StatusOK, `{"output":"native result","results":[]}`)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, rec, body := alphaAPIKeyFallbackContext(t)
	result, err := svc.ForwardAlphaSearch(context.Background(), c, alphaAPIKeyFallbackAccount(), body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "/v1/alpha/search", upstream.lastReq.URL.Path)
	require.JSONEq(t, `{"output":"native result","results":[]}`, rec.Body.String())
}

func TestAlphaSearchAPIKeyFallbackBothEndpointsMissingRemainsBounded(t *testing.T) {
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		alphaAPIKeyFallbackResponse(http.StatusNotFound, `{"error":{"message":"Not Found"}}`),
		alphaAPIKeyFallbackResponse(http.StatusNotFound, `{"error":{"message":"Invalid URL (POST /v1/responses)"}}`),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, rec, body := alphaAPIKeyFallbackContext(t)
	result, err := svc.ForwardAlphaSearch(context.Background(), c, alphaAPIKeyFallbackAccount(), body)
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusNotFound, failover.StatusCode)
	require.False(t, failover.RetryableOnSameAccount)
	require.Len(t, upstream.requests, 2)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestAlphaSearchAPIKeyFallbackClassifierPreservesModelAndAuthErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		fallback bool
	}{
		{"route404", 404, `{"error":{"message":"Invalid URL (POST /v1/alpha/search)"}}`, true},
		{"route405", 405, `{"error":{"message":"Method Not Allowed"}}`, true},
		{"generic404", 404, `Not Found`, true},
		{"model_code", 404, `{"error":{"code":"model_not_found","message":"Unavailable"}}`, false},
		{"model_message", 404, `{"error":{"message":"The requested model does not exist"}}`, false},
		{"authentication_code", 404, `{"error":{"code":"authentication_error"}}`, false},
		{"permission_code", 404, `{"error":{"code":"permission_denied"}}`, false},
		{"invalid_api_key", 404, `{"error":{"message":"Invalid API key"}}`, false},
		{"unauthorized", 401, `{}`, false},
		{"forbidden", 403, `{}`, false},
		{"rate_limit", 429, `{}`, false},
		{"unavailable", 503, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.fallback, shouldUseOpenAIAlphaSearchResponsesFallback(alphaAPIKeyFallbackAccount(), tc.status, []byte(tc.body)))
		})
	}
	require.False(t, shouldUseOpenAIAlphaSearchResponsesFallback(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, 404, nil))
	require.False(t, shouldUseOpenAIAlphaSearchResponsesFallback(nil, 404, nil))
}

func TestAlphaSearchResponsesFallbackCapabilityExpiryAndConfiguration(t *testing.T) {
	for _, change := range []string{"token", "base_url", "proxy", "headers", "expired", "invalid_entry"} {
		t.Run(change, func(t *testing.T) {
			svc := &OpenAIGatewayService{}
			account := alphaAPIKeyFallbackAccount()
			svc.rememberOpenAIAlphaSearchResponsesFallback(account, "token", "")
			require.True(t, svc.useOpenAIAlphaSearchResponsesFallback(account, "token", ""))
			token, proxy := "token", ""
			switch change {
			case "token":
				token = "rotated-token"
			case "base_url":
				account.Credentials["base_url"] = "https://new-relay.example/v1"
			case "proxy":
				proxy = "http://new-proxy.example:8080"
			case "headers":
				account.Credentials[credKeyHeaderOverrideEnabled] = true
				account.Credentials[credKeyHeaderOverrides] = map[string]any{"X-Custom-Route": "new-route"}
			case "expired":
				svc.openaiAlphaSearchResponsesFallback.Store(account.ID, openAIAlphaSearchFallbackState{Configuration: openAIAlphaSearchFallbackConfiguration(account, token, proxy), ExpiresAt: time.Now().Add(-time.Second)})
			case "invalid_entry":
				svc.openaiAlphaSearchResponsesFallback.Store(account.ID, []string{"invalid"})
			}
			require.False(t, svc.useOpenAIAlphaSearchResponsesFallback(account, token, proxy))
		})
	}
}

func TestAlphaSearchFallbackJSONRequiresCompletionAndPreservesCitations(t *testing.T) {
	body := []byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Search answer","annotations":[{"type":"url_citation","url":"https://example.com/source","title":"Source"}]}]}]}`)
	response, err := openAIAlphaSearchResponseFromResponsesSSE(body)
	require.NoError(t, err)
	require.Equal(t, "Search answer", gjson.GetBytes(response, "output").String())
	require.Equal(t, "https://example.com/source", gjson.GetBytes(response, "results.0.url").String())
	for _, body := range []string{`{"status":"incomplete","output":[]}`, `{"status":"failed","output":[]}`, `{"output":[]}`, `{"status":"completed","output":[]}`} {
		_, err := openAIAlphaSearchResponseFromResponsesSSE([]byte(body))
		require.Error(t, err)
	}
}
