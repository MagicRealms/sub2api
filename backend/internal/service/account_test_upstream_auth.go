package service

import (
	"net/http"

	"github.com/tidwall/gjson"
)

// A relay may authenticate our API key successfully but return an OAuth error
// from its backing account. That does not invalidate the relay key itself.
func accountTestUpstreamOAuthRevoked(account *Account, status int, body []byte) bool {
	return account != nil && account.Type == AccountTypeAPIKey &&
		status == http.StatusUnauthorized && gjson.GetBytes(body, "error.code").String() == "token_revoked"
}

const accountTestUpstreamOAuthHint = "上游返回 OAuth token_revoked，但本次使用的是本站配置的 API Key；请让上游检查其 OAuth 账号授权。"
