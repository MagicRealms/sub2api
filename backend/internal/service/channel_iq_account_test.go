//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChannelIQBackgroundProbeUsesSelectedAccountAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, platform, accountType, endpoint, token string
		chat                                         bool
	}{
		{"api key responses", PlatformOpenAI, AccountTypeAPIKey, "https://relay.example/v1/responses", "selected-key", false},
		{"api key chat", PlatformOpenAI, AccountTypeAPIKey, "https://relay.example/v1/chat/completions", "selected-key", true},
		{"sharellm responses", PlatformShareLLM, AccountTypeAPIKey, "https://sharellm.net/v1/responses", "selected-key", false},
		{"oauth", PlatformOpenAI, AccountTypeOAuth, chatgptCodexAPIURL, "oauth-token", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{ID: 31, Platform: tc.platform, Type: tc.accountType, Status: StatusActive,
				Credentials: map[string]any{"base_url": "https://relay.example/v1", "api_key": "selected-key", "access_token": "oauth-token"},
			}
			if tc.chat {
				account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: false}
			}
			repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{31: account}}}
			body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"selected result\"}\n\ndata: {\"type\":\"response.completed\"}\n\n"
			if tc.chat {
				body = "data: {\"choices\":[{\"delta\":{\"content\":\"selected result\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
			}
			upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK, body)}}
			svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
			result, err := svc.RunPromptTestBackgroundWithOptions(context.Background(), 31, "gpt-6.1-sol", "selected prompt", "high")
			require.NoError(t, err)
			require.Equal(t, "success", result.Status, result.ErrorMessage)
			require.Equal(t, "selected result", result.ResponseText)
			require.Len(t, upstream.requests, 1)
			req := upstream.requests[0]
			require.Equal(t, tc.endpoint, req.URL.String())
			require.Equal(t, "Bearer "+tc.token, req.Header.Get("Authorization"))
			payload, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Contains(t, string(payload), "selected prompt")
			if tc.chat {
				require.Equal(t, "high", gjson.GetBytes(payload, "reasoning_effort").String())
			} else {
				require.Equal(t, "high", gjson.GetBytes(payload, "reasoning.effort").String())
			}
		})
	}
}

func TestChannelIQAPIKeyUpstreamRevocationDoesNotDisableRelay(t *testing.T) {
	for _, tc := range []struct {
		name, platform, accountType, code string
		chat, markError                   bool
	}{
		{"relay responses", PlatformOpenAI, AccountTypeAPIKey, "token_revoked", false, false},
		{"relay chat", PlatformOpenAI, AccountTypeAPIKey, "token_revoked", true, false},
		{"sharellm", PlatformShareLLM, AccountTypeAPIKey, "token_revoked", false, false},
		{"invalid API key", PlatformOpenAI, AccountTypeAPIKey, "invalid_api_key", false, true},
		{"revoked local OAuth", PlatformOpenAI, AccountTypeOAuth, "token_revoked", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{ID: 31, Platform: tc.platform, Type: tc.accountType, Status: StatusActive,
				Credentials: map[string]any{"base_url": "https://relay.example/v1", "api_key": "selected-key", "access_token": "oauth-token"},
			}
			if tc.chat {
				account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: false}
			}
			repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{31: account}}}
			upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusUnauthorized, `{"error":{"code":"`+tc.code+`","message":"authentication failed"}}`)}}
			svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
			result, err := svc.RunPromptTestBackgroundWithOptions(context.Background(), 31, "gpt-6.1-sol", "selected prompt", "high")
			require.NoError(t, err)
			require.Equal(t, "failed", result.Status)
			if tc.markError {
				require.Equal(t, account.ID, repo.setErrorID)
			} else {
				require.Zero(t, repo.setErrorID, "upstream OAuth revocation must not disable our relay API key")
				require.Contains(t, result.ErrorMessage, accountTestUpstreamOAuthHint)
			}
		})
	}
}
