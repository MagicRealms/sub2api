package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const orderedRouteResponseLimit = 64 << 10

// Re-enter the complete gateway chain for each server-selected group. This is
// deliberate: group permission, subscription, RPM, channel policy and admission
// must all run again. No credential or group override is accepted from headers.
func orderedKeyGroupRouting(engine *gin.Engine, keys *service.APIKeyService, gateway *handler.GatewayHandler) gin.HandlerFunc {
	return orderedKeyGroupRoutingWithSupport(engine, keys, gateway.GroupRouteSupportsModel, gateway.HasGroupFinalFallback)
}

func orderedKeyGroupRoutingWithSupport(engine *gin.Engine, keys *service.APIKeyService, supportsModel func(context.Context, *service.Group, string, string) (bool, error), fallbackChecks ...func(context.Context, []int64) (bool, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if keys == nil || service.IsAPIKeyGroupRouteAttempt(c.Request.Context()) {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		isModels := c.Request.Method == http.MethodGet && (path == "/v1/models" || path == "/models")
		isModelRead := c.Request.Method == http.MethodGet && (strings.HasPrefix(path, "/v1/models/") || strings.HasPrefix(path, "/models/"))
		// Reject ineligible paths before auth/catalog lookups or body buffering.
		if (c.Request.Method == http.MethodPost && !orderedRouteCanReplay(path)) ||
			(c.Request.Method != http.MethodPost && !isModels && !isModelRead) {
			c.Next()
			return
		}
		credential := orderedRouteCredential(c.Request)
		if credential == "" || len(credential) > service.MaxAPIKeyCredentialBytes ||
			(strings.TrimSpace(c.Query("key")) != "" && !orderedRouteGooglePath(path)) || strings.TrimSpace(c.Query("api_key")) != "" ||
			middleware.APIKeyLookupPrecheckBlocked(c, keys) {
			c.Next()
			return
		}
		key, err := keys.GetByKey(c.Request.Context(), credential)
		if err != nil {
			if errors.Is(err, service.ErrAPIKeyNotFound) {
				c.Next()
			} else {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"type": "api_error", "message": "API key authentication is temporarily unavailable"}})
			}
			return
		}
		probeGroupID, pinnedProbe := service.InternalMonitorProbeGroup(c.Request.Context(), key.ID)
		if len(key.OrderedGroupIDs()) < 2 && !pinnedProbe {
			// Keep unconfigured single-group keys on their original path. Only
			// configured reserve accounts require buffering a final error response.
			configured := false
			if c.Request.Method == http.MethodPost && len(fallbackChecks) > 0 {
				configured, _ = fallbackChecks[0](c.Request.Context(), key.OrderedGroupIDs())
			}
			if !configured {
				c.Next()
				return
			}
		}
		// Let normal authentication report disabled keys/users before performing
		// any group or model queries on their behalf.
		if key.User == nil || !key.User.IsActive() || (!key.IsActive() && key.Status != service.StatusAPIKeyExpired && key.Status != service.StatusAPIKeyQuotaExhausted) {
			c.Next()
			return
		}
		groups, err := keys.OrderedRouteGroups(c.Request.Context(), key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"type": "api_error", "message": "Group route policy is temporarily unavailable"}})
			return
		}
		if pinnedProbe {
			if key.User == nil || !key.User.IsAdmin() {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"message": "Administrator-owned monitor key required"}})
				return
			}
			var selected []*service.Group
			for _, group := range groups {
				if group.ID == probeGroupID {
					selected = []*service.Group{group}
					break
				}
			}
			groups = selected
		}
		if len(groups) == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "group_not_allowed", "message": "No selected group is available to this key"}})
			return
		}
		var body []byte
		model := compositeGeminiModelFromParams(c)
		if c.Request.Method == http.MethodPost {
			body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
			if err != nil {
				if !middleware.RejectRequestBodyReadError(c, "ordered_group_routes", err) {
					middleware.AbortWithRequestBodyReadFailure(c, "ordered_group_routes", err)
				}
				c.Abort()
				return
			}
			c.Request.Body = pkghttputil.NewPrereadBody(body)
			models := requestmodel.FromBodyCandidates(path, c.Request.Header.Get("Content-Type"), body)
			for _, candidate := range models {
				if len(models) > 0 && candidate != models[0] {
					c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Ambiguous model fields are not allowed for ordered group routing"}})
					return
				}
			}
			if model == "" {
				model = requestmodel.FromBodyForRoute(path, c.Request.Header.Get("Content-Type"), body)
			}
		}
		if model == "" {
			model = c.Query("model")
		}
		// Apply the same image permission as the handler before discovering model
		// support or fixing the billing floor. Codex's passive image_gen namespace
		// does not make a text request eligible for this filter.
		if service.IsImageGenerationEndpoint(path) ||
			(strings.HasSuffix(path, "/responses") && service.IsExplicitImageGenerationIntent(path, model, body)) {
			groups = slices.DeleteFunc(groups, func(group *service.Group) bool {
				return !service.GroupAllowsImageGeneration(group)
			})
			if len(groups) == 0 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "permission_error", "message": service.ImageGenerationPermissionMessage()}})
				return
			}
		}
		c.Request = c.Request.WithContext(service.WithAPIKeyGroupRoutePricingAt(c.Request.Context(), time.Now()))
		replay := orderedRouteCanReplay(path)
		finalFallback := replay && !pinnedProbe && len(fallbackChecks) > 0 && accountFinalFallbackCanReplay(path, body)
		var floor *service.Group
		var pending *orderedRouteResponse
		fallbackState := service.NewAccountFinalFallbackState()
		fallbackEligible := false
		eligibleGroups := make([]*service.Group, 0, len(groups))
		for _, group := range groups {
			if c.Request.Context().Err() != nil {
				break
			}
			// Capability is loaded only as a group is reached. A successful first
			// route must not depend on every unused fallback's account catalog.
			if !isModels && len(key.OrderedGroupIDs()) > 1 {
				supports, catalogErr := supportsModel(c.Request.Context(), group, model, path)
				if catalogErr != nil {
					fallbackEligible = false
					pending = newOrderedRouteResponse(c.Writer)
					pending.Header().Set("Content-Type", "application/json")
					pending.WriteHeader(http.StatusServiceUnavailable)
					message := "Group model catalog is temporarily unavailable"
					if errors.Is(catalogErr, service.ErrModelPricingUnavailable) {
						message = "Model pricing is unavailable for the selected group route"
					}
					body, _ := json.Marshal(gin.H{"error": gin.H{"type": "api_error", "message": message}})
					_, _ = pending.Write(body)
					break
				}
				if !supports {
					continue
				}
			}
			if floor == nil {
				floor = group
			}
			ctx, release := pkghttputil.WithRequestBodyMemoryScope(c.Request.Context())
			if err := pkghttputil.ReserveRequestBodyMemory(ctx, int64(len(body))); err != nil {
				fallbackEligible = false
				release()
				pending = newOrderedRouteResponse(c.Writer)
				pending.Header().Set("Content-Type", "application/json")
				pending.WriteHeader(http.StatusServiceUnavailable)
				_, _ = pending.Write([]byte(`{"error":{"type":"api_error","message":"Request body memory budget is temporarily exhausted"}}`))
				break
			}
			ctx = service.WithAPIKeyGroupRoute(ctx, key.ID, group, floor, model)
			if finalFallback {
				ctx = service.WithAccountFinalFallbackAttempt(ctx, fallbackState, group, false)
			}
			if isModels {
				ctx = service.WithAPIKeyRouteDiscovery(ctx, groups)
			}
			req := c.Request.Clone(ctx)
			req.Body = pkghttputil.NewPrereadBody(bytes.Clone(body))
			req.ContentLength = int64(len(body))
			response := newOrderedRouteResponse(c.Writer)
			engine.ServeHTTP(response, req)
			release()
			pending = response
			if !replay || service.APIKeyGroupRouteBilled(ctx) || isModels || !response.canRetry() {
				fallbackEligible = false
				break
			}
			fallbackEligible = service.AccountFinalFallbackEligible(ctx)
			if fallbackEligible {
				eligibleGroups = append(eligibleGroups, group)
			}
		}
		// Every normal group keeps its existing account retries and switch limit.
		// Reserve attempts have a separate, shared, three-account request budget.
		if fallbackEligible && finalFallback && len(eligibleGroups) > 0 && c.Request.Context().Err() == nil {
			for _, group := range eligibleGroups {
				if c.Request.Context().Err() != nil || fallbackState.Attempts() >= service.MaxAccountFinalFallbackAttempts {
					break
				}
				hasReserve, lookupErr := fallbackChecks[0](c.Request.Context(), []int64{group.ID})
				if lookupErr != nil {
					break
				}
				if !hasReserve {
					continue
				}
				supports, supportErr := supportsModel(c.Request.Context(), group, model, path)
				if supportErr != nil {
					break
				}
				if !supports {
					continue
				}
				ctx, release := pkghttputil.WithRequestBodyMemoryScope(c.Request.Context())
				if pkghttputil.ReserveRequestBodyMemory(ctx, int64(len(body))) != nil {
					release()
					break
				}
				ctx = service.WithAPIKeyGroupRoute(ctx, key.ID, group, floor, model)
				ctx = service.WithAccountFinalFallbackAttempt(ctx, fallbackState, group, true)
				req := c.Request.Clone(ctx)
				req.Body = pkghttputil.NewPrereadBody(bytes.Clone(body))
				req.ContentLength = int64(len(body))
				response := newOrderedRouteResponse(c.Writer)
				before := fallbackState.Attempts()
				engine.ServeHTTP(response, req)
				release()
				if fallbackState.Attempts() > before || !response.canRetry() || service.APIKeyGroupRouteBilled(ctx) {
					pending = response
					slog.InfoContext(ctx, "gateway.account_final_fallback", "group_id", group.ID, "attempts", fallbackState.Attempts()-before, "status", response.status)
				}
				if service.APIKeyGroupRouteBilled(ctx) || !response.canRetry() {
					break
				}
			}
		}
		if pending != nil {
			pending.finish()
		} else if c.Request.Context().Err() == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "invalid_request_error", "code": "model_not_found", "message": "No selected group supports the requested model"}})
		}
		c.Abort()
	}
}

func orderedRouteCredential(req *http.Request) string {
	google := orderedRouteGooglePath(req.URL.Path)
	if google {
		if key := strings.TrimSpace(req.Header.Get("x-goog-api-key")); key != "" {
			return key
		}
	}
	if header := req.Header.Get("Authorization"); header != "" {
		if google {
			header = strings.TrimSpace(header)
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if key := strings.TrimSpace(parts[1]); key != "" {
				return key
			}
		}
	}
	key := req.Header.Get("x-api-key")
	if google {
		key = strings.TrimSpace(key)
	}
	if key != "" {
		return key
	}
	if google {
		return strings.TrimSpace(req.URL.Query().Get("key"))
	}
	return req.Header.Get("x-goog-api-key")
}

func orderedRouteGooglePath(path string) bool {
	return strings.HasPrefix(path, "/v1beta") || strings.HasPrefix(path, "/antigravity/v1beta")
}

func orderedRouteCanReplay(path string) bool {
	for _, suffix := range []string{"/responses", "/responses/compact", "/responses/input_tokens", "/messages", "/messages/count_tokens", "/chat/completions", "/alpha/search", "/embeddings", "/images/generations", "/images/edits", ":generateContent", ":streamGenerateContent", ":countTokens"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func accountFinalFallbackCanReplay(path string, body []byte) bool {
	// A successful retry must be able to carry the whole request to a different
	// upstream account. Avoid replaying requests tied to remote state.
	if strings.HasSuffix(path, "/responses/input_tokens") || strings.HasSuffix(path, "/count_tokens") || strings.HasSuffix(path, ":countTokens") {
		return false
	}
	for _, field := range []string{"previous_response_id", "conversation", "cachedContent", "cached_content"} {
		if value := gjson.GetBytes(body, field); value.Exists() && value.Type != gjson.Null && value.String() != "" {
			return false
		}
	}
	for _, field := range []string{"input", "messages"} {
		bound := false
		gjson.GetBytes(body, field).ForEach(func(_, item gjson.Result) bool {
			switch item.Get("type").String() {
			case "function_call_output", "custom_tool_call_output", "computer_call_output", "mcp_approval_response":
				bound = true
				return false
			}
			return true
		})
		if bound {
			return false
		}
	}
	return true
}

type orderedRouteResponse struct {
	target         http.ResponseWriter
	header         http.Header
	status         int
	buffer         bytes.Buffer
	semantic       bool
	streamError    bool
	heartbeatSent  bool
	scanOffset     int
	eventData      []byte
	eventName      string
	permanentError bool
}

func newOrderedRouteResponse(target http.ResponseWriter) *orderedRouteResponse {
	return &orderedRouteResponse{target: target, header: make(http.Header)}
}

func (w *orderedRouteResponse) Header() http.Header         { return w.header }
func (w *orderedRouteResponse) Unwrap() http.ResponseWriter { return w.target }

func (w *orderedRouteResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *orderedRouteResponse) isSSE() bool {
	return strings.HasPrefix(w.header.Get("Content-Type"), "text/event-stream")
}

func (w *orderedRouteResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.semantic {
		return w.target.Write(data)
	}
	if w.status >= 400 {
		if w.buffer.Len()+len(data) > orderedRouteResponseLimit {
			w.commit()
			return w.target.Write(data)
		}
		return w.buffer.Write(data)
	}
	if !w.isSSE() {
		w.commit()
		return w.target.Write(data)
	}
	// Bound the allocation before copying a potentially huge image/tool frame.
	if w.buffer.Len()+len(data) > orderedRouteResponseLimit {
		w.commit()
		return w.target.Write(data)
	}
	n, err := w.buffer.Write(data)
	w.inspectSSE()
	if w.semantic || w.buffer.Len() > orderedRouteResponseLimit {
		w.commit()
	}
	return n, err
}

func (w *orderedRouteResponse) inspectSSE() {
	for !w.semantic && w.scanOffset < w.buffer.Len() {
		remaining := w.buffer.Bytes()[w.scanOffset:]
		end := bytes.IndexByte(remaining, '\n')
		if end < 0 {
			return
		}
		line := bytes.TrimSuffix(remaining[:end], []byte("\r"))
		w.scanOffset += end + 1
		if len(line) == 0 {
			if len(w.eventData) > 0 {
				w.inspectSSEEvent(w.eventData)
			}
			w.eventData = w.eventData[:0]
			w.eventName = ""
		} else if bytes.HasPrefix(line, []byte("event:")) {
			w.eventName = strings.TrimSpace(string(line[6:]))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			if len(w.eventData) > 0 {
				w.eventData = append(w.eventData, '\n')
			}
			w.eventData = append(w.eventData, bytes.TrimPrefix(line[5:], []byte(" "))...)
		}
	}
}

func (w *orderedRouteResponse) inspectSSEEvent(data []byte) {
	if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
		if !w.streamError {
			w.semantic = true
		}
		return
	}
	if !gjson.ValidBytes(data) {
		// Unknown delivered data is not safe to replay, even if it is not JSON.
		w.semantic = true
		return
	}
	if len(gjson.GetBytes(data, "response.output").Array()) > 0 || len(gjson.GetBytes(data, "message.content").Array()) > 0 {
		w.semantic = true
		return
	}
	if w.eventName == "error" || gjson.GetBytes(data, "error").IsObject() {
		w.streamError = true
		w.permanentError = w.permanentError || orderedRoutePermanentError(data)
		return
	}
	typeName := gjson.GetBytes(data, "type").String()
	switch typeName {
	case "error", "response.failed", "response.incomplete":
		w.streamError = true
	case "response.created", "response.in_progress", "message_start", "ping":
	case "response.content_part.added":
		part := gjson.GetBytes(data, "part")
		w.semantic = part.Get("text").String() != "" || part.Get("refusal").String() != ""
	case "response.output_item.added":
		item := gjson.GetBytes(data, "item")
		w.semantic = item.Get("type").String() != "message" && item.Get("type").String() != "reasoning" ||
			len(item.Get("content").Array()) > 0 || len(item.Get("summary").Array()) > 0
	case "content_block_start":
		block := gjson.GetBytes(data, "content_block")
		if (block.Get("type").String() != "" && block.Get("type").String() != "text") || block.Get("text").String() != "" {
			w.semantic = true
		}
	default:
		// Empty output deltas are only a prelude. Tool calls, reasoning, text,
		// images and completed responses irrevocably commit the request.
		if strings.HasSuffix(typeName, ".delta") && gjson.GetBytes(data, "delta").Type == gjson.String && gjson.GetBytes(data, "delta").String() == "" {
			return
		}
		if typeName == "" {
			delta := gjson.GetBytes(data, "choices.0.delta")
			if delta.Exists() && delta.Get("content").String() == "" && delta.Get("reasoning_content").String() == "" && delta.Get("reasoning").String() == "" && delta.Get("refusal").String() == "" && !delta.Get("tool_calls").Exists() && !delta.Get("function_call").Exists() && gjson.GetBytes(data, "choices.0.finish_reason").String() == "" {
				return
			}
		}
		w.semantic = true
	}
	if w.streamError {
		w.permanentError = w.permanentError || orderedRoutePermanentError(data)
	}
}

func (w *orderedRouteResponse) copyHeaders() {
	for key, values := range w.header {
		w.target.Header()[key] = append([]string(nil), values...)
	}
}

func (w *orderedRouteResponse) commit() {
	w.semantic = true
	w.copyHeaders()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.target.WriteHeader(w.status)
	_, _ = io.Copy(w.target, &w.buffer)
	w.buffer = bytes.Buffer{}
	w.eventData = nil
}

func (w *orderedRouteResponse) Flush() {
	if w.semantic {
		if f, ok := w.target.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	if w.isSSE() && w.status < 400 {
		// Heartbeats can cross group boundaries; semantic events cannot.
		w.copyHeaders()
		w.target.WriteHeader(http.StatusOK)
		_, _ = w.target.Write([]byte(": keepalive\n\n"))
		if f, ok := w.target.(http.Flusher); ok {
			f.Flush()
		}
		w.heartbeatSent = true
	}
}

func (w *orderedRouteResponse) canRetry() bool {
	if w.semantic {
		return false
	}
	if w.permanentError || orderedRoutePermanentError(w.buffer.Bytes()) {
		return false
	}
	if w.streamError {
		return true
	}
	switch w.status {
	case 404, 429, 500, 502, 503, 504:
		return true
	}
	return false
}

func (w *orderedRouteResponse) finish() {
	if w.semantic {
		return
	}
	// A prior attempt may already have sent SSE keepalives. Preserve the wire
	// protocol when the final attempt ends with a normal JSON HTTP error.
	if w.status >= 400 && strings.HasPrefix(w.target.Header().Get("Content-Type"), "text/event-stream") {
		body := w.buffer.Bytes()
		if !json.Valid(body) {
			body = []byte(`{"error":{"type":"api_error","message":"All selected group routes failed"}}`)
		}
		_, _ = w.target.Write(append(append([]byte("event: error\ndata: "), body...), []byte("\n\n")...))
		return
	}
	w.commit()
}

// Ensure the auth middleware and every other route guard ran before discovery.
func orderedDiscoveryModels(c *gin.Context, gateway *handler.GatewayHandler) bool {
	groups := service.APIKeyRouteDiscovery(c.Request.Context())
	if len(groups) == 0 {
		return false
	}
	if _, ok := middleware.GetAPIKeyFromContext(c); !ok {
		return false
	}
	gateway.OrderedGroupModels(c, groups)
	return true
}

func orderedRoutePermanentError(body []byte) bool {
	modelMissing := gjson.GetBytes(body, "error.code").String() == "model_not_found" || gjson.GetBytes(body, "code").String() == "model_not_found"
	fields := []string{"error.code", "error.type", "error.message", "response.error.code", "response.error.type", "response.error.message", "code", "type", "message"}
	for _, field := range fields {
		value := strings.ToLower(gjson.GetBytes(body, field).String())
		for _, blocked := range []string{"cyber", "policy", "content_filter", "safety", "moderation", "invalid_request", "invalid_api_key", "api_key_quota", "api_key_rate", "insufficient_balance", "permission", "access_denied", "group_not_allowed"} {
			if modelMissing && (field == "error.type" || field == "type") && blocked == "invalid_request" {
				continue
			}
			if strings.Contains(value, blocked) {
				return true
			}
		}
	}
	// SSE errors are data lines rather than one JSON document.
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("data:")) {
			data := bytes.TrimSpace(line[5:])
			if gjson.ValidBytes(data) && orderedRoutePermanentError(data) {
				return true
			}
		}
	}
	return false
}
