package service

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// Preserve HTTP retry guidance through failover. Cloudflare can send an RFC
// problem document with retry_after instead of the HTTP header. Accept only
// its explicit, matching 5xx error envelope and a bounded integer delay; never
// search arbitrary echoed request content or change failover eligibility.
func upstreamFailoverResponseHeaders(status int, headers http.Header, body []byte) http.Header {
	result := headers.Clone()
	if result.Get("Retry-After") != "" || status < 500 || status > 599 ||
		!strings.EqualFold(headers.Get("Server"), "cloudflare") || !gjson.ValidBytes(body) {
		return result
	}
	root := gjson.ParseBytes(body)
	if root.Get("cloudflare_error").Type != gjson.True || root.Get("retryable").Type != gjson.True ||
		root.Get("status").Type != gjson.Number || root.Get("status").Int() != int64(status) {
		return result
	}
	value := root.Get("retry_after")
	seconds := value.Int()
	if value.Type != gjson.Number || seconds < 1 || seconds > 300 || value.Float() != float64(seconds) {
		return result
	}
	if result == nil {
		result = make(http.Header)
	}
	result.Set("Retry-After", strconv.FormatInt(seconds, 10))
	return result
}
