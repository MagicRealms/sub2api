// Package errorprivacy owns the customer-facing projection of diagnostic errors.
// Never apply it before retry/policy matching or before recording admin diagnostics.
package errorprivacy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

const (
	Site          = "太阳新世界（codex.magicrealms.top）"
	MaxErrorBytes = 64 << 10
)

var identity = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://|(?:[a-z0-9-]+\.)+[a-z]{2,63}\b|\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b|\[[0-9a-f:]+\]|\b[0-9a-f]{0,4}:[0-9a-f:]*:[0-9a-f:]+\b|\bsk-[a-z0-9_-]+|bearer\s+|sharellm|openrouter)`)
var localFieldPath = regexp.MustCompile(`(?i)^(?:input|output|response|responses|request|reasoning|thinking|messages|content|tool|tools|session|model|metadata)(?:\.[a-zA-Z_][a-zA-Z0-9_]*|\.[0-9]+|\[[0-9]+\])+$`)
var quotedLocalFieldPath = regexp.MustCompile(`"[a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*|\[[0-9]+\])+"`)
var protocolWords = strings.NewReplacer("response.create", "response_create", "response.cancel", "response_cancel", "response.failed", "response_failed", "response.id", "response_id")
var publicShareLLMIdentity = regexp.MustCompile(`(?i)(?:(?:[a-z0-9-]+\.)*sharellm\.net\.?|share[\s_-]*llm)`)

const PublicRouteAlias = "路由中转"
const PublicRouteID = "route_relay"
const PublicHost = "codex.magicrealms.top"

// ScrubText is the narrow customer-facing identity filter. It intentionally
// targets only the hidden ShareLLM identity; other provider URLs remain valid
// user content and are left untouched.
func ScrubText(message string) string {
	if !hasPrivateIdentityPrefix(message) {
		return message
	}
	return publicShareLLMIdentity.ReplaceAllStringFunc(message, func(value string) string {
		if strings.HasSuffix(strings.ToLower(strings.TrimSuffix(value, ".")), "sharellm.net") {
			return PublicHost
		}
		return PublicRouteAlias
	})
}

func ScrubBytes(body []byte) []byte {
	if !hasPrivateIdentityPrefix(body) || !publicShareLLMIdentity.Match(body) {
		return body
	}
	return []byte(ScrubText(string(body)))
}

// ScrubJSON recursively projects ordinary customer JSON. It is separate from
// JSON because successful model content needs the same narrow identity filter.
func ScrubJSON(body []byte) ([]byte, bool) {
	if !hasPrivateIdentityPrefix(body) && !bytes.Contains(body, []byte(`\u`)) {
		return body, false
	}
	if !json.Valid(body) {
		projected := ScrubBytes(body)
		return projected, !bytes.Equal(projected, body)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return body, false
	}
	changed := scrubJSONValue(&value)
	if !changed {
		return body, false
	}
	projected, err := json.Marshal(value)
	if err != nil {
		return ScrubBytes(body), true
	}
	return projected, true
}

// An allocation-free ASCII prefilter keeps normal token streams off the regexp
// and full JSON decoding paths. JSON Unicode escapes are checked separately.
func hasPrivateIdentityPrefix[T ~string | ~[]byte](value T) bool {
	for i := 0; i+5 <= len(value); i++ {
		if value[i]|0x20 == 's' && value[i+1]|0x20 == 'h' && value[i+2]|0x20 == 'a' && value[i+3]|0x20 == 'r' && value[i+4]|0x20 == 'e' {
			return true
		}
	}
	return false
}

func scrubJSONValue(value *any) bool {
	switch current := (*value).(type) {
	case string:
		updated := ScrubText(current)
		if updated == current {
			return false
		}
		*value = updated
		return true
	case []any:
		changed := false
		for i := range current {
			item := any(current[i])
			if scrubJSONValue(&item) {
				current[i] = item
				changed = true
			}
		}
		return changed
	case map[string]any:
		changed := false
		// Build a separate map: inserting renamed keys while ranging a map can
		// process those keys twice and makes collisions nondeterministic.
		projected := make(map[string]any, len(current))
		for key, item := range current {
			publicKey := ScrubText(key)
			if strings.EqualFold(key, "sharellm") {
				publicKey = PublicRouteID
			}
			if publicKey != key {
				changed = true
				// An existing public field wins over a renamed private field.
				if _, exists := current[publicKey]; exists {
					continue
				}
			}
			if key == "platform" || key == "target_platform" {
				if platform, ok := item.(string); ok {
					if public := PublicPlatform(platform); public != platform {
						item = public
						changed = true
					}
				}
			}
			if scrubJSONValue(&item) {
				changed = true
			}
			projected[publicKey] = item
		}
		if changed {
			*value = projected
		}
		return changed
	default:
		return false
	}
}

// PublicJSON applies error projection first, then the narrow identity filter
// to ordinary successful JSON as well.
func PublicJSON(body []byte, upstream bool) ([]byte, bool) {
	if projected, changed := JSON(body, upstream); changed {
		scrubbed, _ := ScrubJSON(projected)
		return scrubbed, true
	}
	return ScrubJSON(body)
}

func PublicPlatform(platform string) string {
	if strings.EqualFold(strings.TrimSpace(platform), "sharellm") {
		return PublicRouteID
	}
	return ScrubText(platform)
}

// Text preserves local actionable errors, but never returns diagnostic addresses.
func Text(message string) string {
	if len(message) > MaxErrorBytes || publicShareLLMIdentity.MatchString(message) {
		return UpstreamMessage(message)
	}
	trimmed := strings.TrimSpace(message)
	if localFieldPath.MatchString(trimmed) {
		return message
	}
	identityInput := protocolWords.Replace(message)
	identityInput = quotedLocalFieldPath.ReplaceAllStringFunc(identityInput, func(value string) string {
		if localFieldPath.MatchString(value[1 : len(value)-1]) {
			return ""
		}
		return value
	})
	if identity.MatchString(identityInput) {
		return UpstreamMessage(message)
	}
	return message
}

// UpstreamMessage does not echo arbitrary supplier prose. Canonical context and
// continuation phrases are retained because clients use them for recovery.
func UpstreamMessage(message string) string {
	if len(message) > MaxErrorBytes {
		message = message[:MaxErrorBytes]
	}
	lower := strings.ToLower(message)
	switch {
	case containsAny(lower, "context_length", "context_too_large", "context length", "context window", "prompt is too long", "maximum prompt length", "input is too long", "input too long", "too many tokens", "maximum number of tokens"):
		return Site + "：context_length_exceeded: prompt is too long; please shorten the input or compact the conversation."
	case strings.Contains(lower, "previous_response") && containsAny(lower, "not found", "not_found", "invalid", "does not exist"):
		return Site + "：previous_response_id not found; please start a new conversation."
	case containsAny(lower, "timeout", "timed out", "deadline exceeded", "超时"):
		return Site + "请求超时，请稍后重试。"
	case containsAny(lower, "rate_limit", "rate limit", "too many requests", "overloaded", "capacity", "限流"):
		return Site + "当前请求较多，请稍后重试。"
	case containsAny(lower, "content_policy", "content policy", "cyber_policy", "safety policy"):
		return Site + "：request rejected by content policy."
	case containsAny(lower, "model_not_found", "model not found", "model is not supported", "unsupported model"):
		return Site + "：the requested model is not available."
	default:
		return Site + "请求暂时无法完成，请稍后重试。"
	}
}

func containsAny(s string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(s, value) {
			return true
		}
	}
	return false
}

// JSON only changes protocol error envelopes. Successful content, citations,
// image URLs, usage and normal stream deltas remain byte-for-byte unchanged.
// Supplier metadata is removed with an allowlist; type/code/param/status and
// correlation IDs remain available to clients.
func JSON(body []byte, upstream bool) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return body, false
	}
	typ := gjson.GetBytes(body, "type").String()
	// Standard delta/content events cannot carry protocol failures. Avoid full
	// decoding on this hot path (and never inspect their normal text/URLs).
	if strings.HasPrefix(typ, "response.output_") || strings.HasPrefix(typ, "response.reasoning_") || strings.HasPrefix(typ, "content_block_") || typ == "message_delta" || typ == "message_start" || typ == "message_stop" || typ == "ping" {
		return body, false
	}
	hasError := typ == "error" || typ == "response.failed"
	if !hasError {
		for _, path := range []string{"error", "response.error"} {
			value := gjson.GetBytes(body, path)
			if value.Exists() && value.Type != gjson.Null {
				hasError = true
				break
			}
		}
	}
	if !hasError {
		return body, false
	}
	if len(body) > MaxErrorBytes {
		// Existing transport limits bound the frame; inspect only its type here,
		// without decoding/copying its potentially large diagnostic payload.
		return Fallback(), true
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(body, &root) != nil || root == nil {
		return body, false
	}
	var response map[string]json.RawMessage
	_ = json.Unmarshal(root["response"], &response)
	out := pick(root, "type", "code", "param", "status", "status_code", "id", "object", "sequence_number", "request_id", "event_id", "created_at", "completed_at", "expires_at", "task_id", "http_status")
	if stringValue(root["object"]) == "image.generation.task" {
		// These are generated content, not supplier diagnostics.
		for _, key := range []string{"image_url", "result"} {
			if value, ok := root[key]; ok {
				out[key] = value
			}
		}
	}
	if nonNull(root["error"]) {
		out["error"] = projectError(root["error"], upstream)
	}
	if nonNull(root["message"]) {
		out["message"] = messageJSON(stringValue(root["message"]), upstream)
	}
	if response != nil {
		projected := pick(response, "id", "object", "status", "created_at")
		if nonNull(response["error"]) {
			projected["error"] = projectError(response["error"], upstream)
		}
		out["response"], _ = json.Marshal(projected)
	}
	if len(out) == 0 {
		return Fallback(), true
	}
	result, err := json.Marshal(out)
	if err != nil {
		return Fallback(), true
	}
	return result, true
}

func projectError(raw json.RawMessage, upstream bool) json.RawMessage {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return messageJSON(stringValue(raw), upstream)
	}
	out := pick(fields, "type", "code", "param", "status", "status_code", "request_id", "event_id")
	message := stringValue(fields["message"])
	if upstream {
		// Also use machine-readable semantics when an upstream omits its message.
		message += " " + stringValue(fields["code"]) + " " + stringValue(fields["type"])
	}
	out["message"] = messageJSON(message, upstream)
	result, _ := json.Marshal(out)
	return result
}

func messageJSON(message string, upstream bool) json.RawMessage {
	if upstream {
		message = UpstreamMessage(message)
	} else {
		message = Text(message)
	}
	result, _ := json.Marshal(message)
	return result
}

func pick(fields map[string]json.RawMessage, keys ...string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage)
	for _, key := range keys {
		if value, ok := fields[key]; ok {
			// Protocol scalars cannot be used as a secondary metadata tunnel.
			if bytes.HasPrefix(value, []byte("{")) || bytes.HasPrefix(value, []byte("[")) {
				continue
			}
			text := stringValue(value)
			protocolName := (key == "type" && (text == "response.failed" || text == "response.incomplete" || text == "response.cancelled")) ||
				(key == "object" && (text == "image.generation.task" || text == "response" || text == "chat.completion"))
			parameterPath := key == "param" && !strings.ContainsAny(text, ":/@ ")
			if len(text) > 256 || (!protocolName && !parameterPath && identity.MatchString(text)) {
				if key == "type" || key == "code" {
					out[key] = json.RawMessage(`"upstream_error"`)
				} else {
					out[key] = json.RawMessage(`null`)
				}
				continue
			}
			out[key] = value
		}
	}
	return out
}

func nonNull(value json.RawMessage) bool     { return len(value) > 0 && string(value) != "null" }
func stringValue(raw json.RawMessage) string { var s string; _ = json.Unmarshal(raw, &s); return s }

func Fallback() []byte {
	result, _ := json.Marshal(map[string]any{"error": map[string]any{"type": "upstream_error", "message": UpstreamMessage("")}})
	return result
}

func Body(body string, upstream bool) string {
	if body == "" {
		return ""
	}
	if result, changed := JSON([]byte(body), upstream); changed {
		projected, _ := ScrubJSON(result)
		return string(projected)
	}
	// A non-envelope diagnostic (HTML, nested vendor metadata, or malformed JSON)
	// has no public schema. Do not reflect it or return it merely truncated.
	return string(Fallback())
}

// Headers removes diagnostic/redirect headers before the public response is
// committed. Retry-After, request IDs and rate-limit headers are preserved.
func Headers(headers http.Header, errorResponse bool) {
	for key := range headers {
		lower := strings.ToLower(key)
		identityValue := false
		for _, value := range headers[key] {
			if publicShareLLMIdentity.MatchString(value) {
				identityValue = true
				break
			}
		}
		if identityValue || publicShareLLMIdentity.MatchString(key) {
			headers.Del(key)
			continue
		}
		if lower == "server" || lower == "via" || lower == "x-powered-by" ||
			strings.HasPrefix(lower, "x-upstream") || strings.HasPrefix(lower, "x-provider") ||
			strings.HasPrefix(lower, "x-envoy") || strings.HasPrefix(lower, "cf-") ||
			(errorResponse && (lower == "location" || lower == "www-authenticate")) {
			headers.Del(key)
			continue
		}
		if errorResponse || strings.HasPrefix(lower, "x-") {
			for _, value := range headers[key] {
				if lower == "retry-after" {
					if _, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64); err == nil {
						continue
					}
					if _, err := http.ParseTime(value); err == nil {
						continue
					}
				}
				if len(value) > 8192 || identity.MatchString(value) {
					headers.Del(key)
					break
				}
			}
		}
	}
}

// CloseReason keeps WebSocket reasons valid UTF-8 and within the control-frame
// budget. Raw causes remain on the internal error, never on the close frame.
func CloseReason(reason string) string {
	reason = Text(strings.TrimSpace(reason))
	if len(reason) <= 120 {
		return reason
	}
	reason = reason[:120]
	for !utf8.ValidString(reason) {
		reason = reason[:len(reason)-1]
	}
	return reason
}
