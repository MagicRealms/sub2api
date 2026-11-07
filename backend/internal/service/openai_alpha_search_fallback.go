package service

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const openAIAlphaSearchFallbackTTL = 10 * time.Minute

type openAIAlphaSearchFallbackState struct {
	Configuration [sha256.Size]byte
	ExpiresAt     time.Time
}

// Keep one short-lived capability entry per account. Credentials and routing
// changes invalidate it immediately; only a digest is retained, never a token.
func openAIAlphaSearchFallbackConfiguration(account *Account, token, proxyURL string) [sha256.Size]byte {
	configuration, _ := json.Marshal(struct {
		BaseURL string
		Token   string
		Proxy   string
		Headers map[string]string
	}{account.GetOpenAIBaseURL(), token, proxyURL, account.GetHeaderOverrides()})
	return sha256.Sum256(configuration)
}

func (s *OpenAIGatewayService) useOpenAIAlphaSearchResponsesFallback(account *Account, token, proxyURL string) bool {
	if s == nil || account == nil || !account.IsOpenAIApiKey() || account.ID <= 0 {
		return false
	}
	raw, ok := s.openaiAlphaSearchResponsesFallback.Load(account.ID)
	if !ok {
		return false
	}
	entry, valid := raw.(openAIAlphaSearchFallbackState)
	if !valid {
		s.openaiAlphaSearchResponsesFallback.Delete(account.ID)
		return false
	}
	if !time.Now().Before(entry.ExpiresAt) || entry.Configuration != openAIAlphaSearchFallbackConfiguration(account, token, proxyURL) {
		s.openaiAlphaSearchResponsesFallback.CompareAndDelete(account.ID, entry)
		return false
	}
	return true
}

func (s *OpenAIGatewayService) rememberOpenAIAlphaSearchResponsesFallback(account *Account, token, proxyURL string) {
	if s == nil || account == nil || !account.IsOpenAIApiKey() || account.ID <= 0 {
		return
	}
	now := time.Now()
	s.openaiAlphaSearchResponsesFallback.Store(account.ID, openAIAlphaSearchFallbackState{
		Configuration: openAIAlphaSearchFallbackConfiguration(account, token, proxyURL),
		ExpiresAt:     now.Add(openAIAlphaSearchFallbackTTL),
	})
	// Remove expired entries on writes, including accounts no longer receiving
	// traffic, so deleted-account capability hints cannot accumulate forever.
	s.openaiAlphaSearchResponsesFallback.Range(func(key, raw any) bool {
		if entry, ok := raw.(openAIAlphaSearchFallbackState); ok && !now.Before(entry.ExpiresAt) {
			s.openaiAlphaSearchResponsesFallback.CompareAndDelete(key, raw)
		}
		return true
	})
}

func shouldUseOpenAIAlphaSearchResponsesFallback(account *Account, statusCode int, body []byte) bool {
	if account == nil || !account.IsOpenAIApiKey() || !isOpenAIAlphaSearchEndpointUnsupported(account, statusCode) {
		return false
	}
	// A model-specific rejection is not evidence that the standalone route is
	// missing. Preserve its existing failover policy and never cache it as a
	// capability failure for every model on this account.
	for _, path := range []string{"error.code", "error.type", "code", "type"} {
		code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, path).String()))
		if strings.Contains(code, "model") || strings.Contains(code, "auth") ||
			strings.Contains(code, "permission") || strings.Contains(code, "policy") {
			return false
		}
	}
	message := strings.ToLower(extractUpstreamErrorMessage(body))
	if strings.Contains(message, "model") || strings.Contains(message, "api key") || strings.Contains(message, "api_key") {
		return false
	}
	return statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed
}
