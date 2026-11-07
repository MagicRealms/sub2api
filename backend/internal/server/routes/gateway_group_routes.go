package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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
	return orderedKeyGroupRoutingWithSupport(engine, keys, gateway.GroupRouteSupportsModel)
}

func orderedKeyGroupRoutingWithSupport(engine *gin.Engine, keys *service.APIKeyService, supportsModel func(context.Context, *service.Group, string, string) (bool, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if keys == nil || service.IsAPIKeyGroupRouteAttempt(c.Request.Context()) {
			c.Next()
			return
		}
		credential := orderedRouteCredential(c.Request)
		if credential == "" || len(credential) > service.MaxAPIKeyCredentialBytes || c.Query("key") != "" || c.Query("api_key") != "" {
			c.Next()
			return
		}
		key, err := keys.GetByKey(c.Request.Context(), credential)
		if err != nil || len(key.OrderedGroupIDs()) < 2 {
			c.Next()
			return
		}
		groups, err := keys.OrderedRouteGroups(c.Request.Context(), key)
		if err != nil || len(groups) == 0 {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		// Protocol upgrades bind one account for their lifetime. The existing WS
		// handler retains ownership; frames must never be replayed by HTTP retry.
		if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			c.Next()
			return
		}
		// Async jobs and other stateful APIs keep their established primary-group
		// ownership for both creation and polling. Only completion-style requests
		// and model discovery participate in ordered cross-group routing.
		if c.Request.Method == http.MethodPost && !orderedRouteCanReplay(path) {
			c.Next()
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
		isModels := c.Request.Method == http.MethodGet && (path == "/v1/models" || path == "/models")
		isModelRead := c.Request.Method == http.MethodGet && (strings.HasPrefix(path, "/v1/models/") || strings.HasPrefix(path, "/models/"))
		if c.Request.Method != http.MethodPost && !isModels && !isModelRead {
			c.Next()
			return
		}
		candidates := make([]*service.Group, 0, len(groups))
		for _, group := range groups {
			supports, catalogErr := supportsModel(c.Request.Context(), group, model, path)
			if catalogErr != nil {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"type": "api_error", "message": "Group model catalog is temporarily unavailable"}})
				return
			}
			if isModels || supports {
				candidates = append(candidates, group)
			}
		}
		if len(candidates) == 0 {
			c.Next()
			return
		}
		c.Request = c.Request.WithContext(service.WithAPIKeyGroupRoutePricingAt(c.Request.Context(), time.Now()))
		replay := orderedRouteCanReplay(path)
		for index, group := range candidates {
			if c.Request.Context().Err() != nil {
				break
			}
			ctx, release := pkghttputil.WithRequestBodyMemoryScope(c.Request.Context())
			if err := pkghttputil.ReserveRequestBodyMemory(ctx, int64(len(body))); err != nil {
				release()
				middleware.RejectRequestBodyReadError(c, "ordered_group_routes", err)
				break
			}
			ctx = service.WithAPIKeyGroupRoute(ctx, key.ID, group, candidates[0], model)
			if isModels {
				ctx = service.WithAPIKeyRouteDiscovery(ctx, candidates)
			}
			req := c.Request.Clone(ctx)
			req.Body = pkghttputil.NewPrereadBody(bytes.Clone(body))
			req.ContentLength = int64(len(body))
			response := newOrderedRouteResponse(c.Writer)
			engine.ServeHTTP(response, req)
			release()
			if !replay || service.APIKeyGroupRouteBilled(ctx) || isModels || index == len(candidates)-1 || !response.canRetry() {
				response.finish()
				break
			}
		}
		c.Abort()
	}
}

func orderedRouteCredential(req *http.Request) string {
	if header := req.Header.Get("Authorization"); header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	if key := req.Header.Get("x-api-key"); key != "" {
		return key
	}
	return req.Header.Get("x-goog-api-key")
}

func orderedRouteCanReplay(path string) bool {
	for _, suffix := range []string{"/responses", "/responses/compact", "/responses/input_tokens", "/messages", "/messages/count_tokens", "/chat/completions", "/alpha/search", "/embeddings", "/images/generations", "/images/edits", ":generateContent", ":streamGenerateContent", ":countTokens"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

type orderedRouteResponse struct {
	target        http.ResponseWriter
	header        http.Header
	status        int
	buffer        bytes.Buffer
	semantic      bool
	streamError   bool
	heartbeatSent bool
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
	n, err := w.buffer.Write(data)
	w.inspectSSE()
	if w.semantic || w.buffer.Len() > orderedRouteResponseLimit {
		w.commit()
	}
	return n, err
}

func (w *orderedRouteResponse) inspectSSE() {
	for _, line := range bytes.Split(w.buffer.Bytes(), []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(line[len("data:"):])
		if bytes.Equal(data, []byte("[DONE]")) {
			if !w.streamError {
				w.semantic = true
			}
			continue
		}
		if !gjson.ValidBytes(data) {
			continue
		} // split frames remain buffered
		typeName := gjson.GetBytes(data, "type").String()
		switch typeName {
		case "error", "response.failed", "response.incomplete":
			w.streamError = true
		case "response.created", "response.in_progress", "message_start", "ping", "response.content_part.added":
		case "response.output_item.added":
			if kind := gjson.GetBytes(data, "item.type").String(); kind != "message" && kind != "reasoning" {
				w.semantic = true
			}
		case "content_block_start":
			block := gjson.GetBytes(data, "content_block")
			if block.Get("type").String() == "tool_use" || block.Get("text").String() != "" {
				w.semantic = true
			}
		default:
			// Empty output deltas are only a prelude. Tool calls, reasoning, text,
			// images and completed responses irrevocably commit the request.
			if strings.HasSuffix(typeName, ".delta") && gjson.GetBytes(data, "delta").Type == gjson.String && gjson.GetBytes(data, "delta").String() == "" {
				continue
			}
			if typeName == "" {
				delta := gjson.GetBytes(data, "choices.0.delta")
				if delta.Exists() && delta.Get("content").String() == "" && !delta.Get("tool_calls").Exists() && !gjson.GetBytes(data, "choices.0.finish_reason").Exists() {
					continue
				}
			}
			w.semantic = true
		}
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
	if orderedRoutePermanentError(w.buffer.Bytes()) {
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
	fields := []string{"error.code", "error.type", "error.message", "response.error.code", "response.error.type", "response.error.message"}
	for _, field := range fields {
		value := strings.ToLower(gjson.GetBytes(body, field).String())
		for _, blocked := range []string{"cyber", "policy", "content_filter", "safety", "moderation", "invalid_request", "invalid_api_key", "api_key_quota", "api_key_rate", "insufficient_balance", "permission", "access_denied", "group_not_allowed"} {
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
