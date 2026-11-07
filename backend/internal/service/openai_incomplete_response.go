package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const openAIIncompleteResponseMessage = "Upstream returned response.incomplete without a reason"

// handleOpenAIIncompleteResponse recovers an unexplained incomplete response
// only before any output (including tool progress) has been delivered. Once a
// turn has output, preserve its terminal and billing but record the failure so
// HTTP 200 cannot hide it. Never invent max_output_tokens or bypass a refusal.
// payload can be either an SSE event or a buffered Responses object.
func (s *OpenAIGatewayService) handleOpenAIIncompleteResponse(
	c *gin.Context, account *Account, resp *http.Response, passthrough bool,
	eventType string, payload []byte, outputStarted bool, model string,
) *UpstreamFailoverError {
	if c == nil || account == nil || account.Platform != PlatformOpenAI || !gjson.ValidBytes(payload) {
		return nil
	}
	response := gjson.ParseBytes(payload)
	if nested := response.Get("response"); nested.IsObject() {
		response = nested
	}
	if (eventType != "" && eventType != "response.incomplete") ||
		(eventType == "" && response.Get("status").String() != "incomplete") {
		return nil
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return nil
	}
	reason := strings.TrimSpace(response.Get("incomplete_details.reason").String())
	if reason != "" && !strings.EqualFold(reason, "unknown") {
		return nil
	}
	// Even buffered output may contain completed tools. Keep this conservative:
	// an output item of any type prevents replay, including encrypted reasoning.
	output := response.Get("output")
	hasOutput := len(output.Array()) > 0 || response.Get("output_text").String() != "" ||
		(output.Exists() && output.Type != gjson.Null && !output.IsArray())
	message := openAIIncompleteResponseMessage
	if upstreamMessage := extractOpenAISSEErrorMessage(payload); upstreamMessage != "" &&
		(response.Get("error").IsObject() || gjson.GetBytes(payload, "error").IsObject()) {
		message += ": " + upstreamMessage
	}
	canRetry := !outputStarted && !hasOutput && !IsResponseCommitted(c) &&
		openAIStreamFailedEventShouldFailover(payload, message)
	if c.Request == nil || c.Request.Context().Err() != nil {
		canRetry = false
	}
	upstreamRequestID := ""
	var headers http.Header
	if resp != nil {
		headers = resp.Header
		upstreamRequestID = strings.TrimSpace(headers.Get("x-request-id"))
	}
	// Record diagnostic metadata only, never prompt text, generated content,
	// tool arguments, or opaque reasoning from a terminal response.
	diagnostic, _ := json.Marshal(gin.H{
		"type": "response.incomplete",
		"response": gin.H{
			"id": response.Get("id").String(), "status": "incomplete",
			"incomplete_details": gin.H{"reason": reason},
			"input_tokens":       response.Get("usage.input_tokens").Int(),
			"output_tokens":      response.Get("usage.output_tokens").Int(),
			"output_items":       len(response.Get("output").Array()),
		},
		"error": gin.H{"code": "upstream_response_incomplete", "type": "server_error", "message": message},
	})
	logContext := context.Background()
	if c.Request != nil {
		logContext = c.Request.Context()
	}
	logger.FromContext(logContext).Warn("gateway.openai_response_incomplete",
		zap.Int64("account_id", account.ID), zap.String("model", model),
		zap.String("upstream_request_id", upstreamRequestID),
		zap.String("response_id", response.Get("id").String()),
		zap.Bool("output_started", outputStarted), zap.Bool("has_output", hasOutput),
		zap.Bool("failover", canRetry),
	)
	if canRetry {
		err := s.newOpenAIStreamFailoverErrorWithModel(c, account, passthrough,
			upstreamRequestID, diagnostic, message, model, headers)
		// An unexplained terminal is not a known transient capacity signal. Try
		// another eligible account within the existing request switch budget.
		err.RetryableOnSameAccount = false
		return err
	}
	s.recordOpenAIStreamUpstreamError(c, account, passthrough, upstreamRequestID,
		"stream_incomplete", diagnostic, message)
	MarkOpsStreamFailure(c, "upstream_error", "upstream_response_incomplete", message, http.StatusBadGateway)
	return nil
}

func openAIIncompleteSSEHasOutput(body string) bool {
	hasOutput := false
	forEachOpenAISSEFrame(body, func(eventType string, payload []byte) {
		if !hasOutput && !openAIStreamEventTypeIsTerminal(eventType) &&
			openAIStreamDataStartsClientOutput(string(payload), eventType) {
			hasOutput = true
		}
	})
	return hasOutput
}
