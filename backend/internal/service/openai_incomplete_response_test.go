package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIIncompleteResponseRecovery(t *testing.T) {
	for _, mode := range []string{"native_stream", "passthrough_stream", "native_sse_json", "passthrough_sse_json", "native_json", "passthrough_json"} {
		for _, tc := range []struct {
			name, reason   string
			partial, retry bool
		}{
			{name: "missing_reason", retry: true},
			{name: "unknown_reason", reason: "unknown", retry: true},
			{name: "partial_missing_reason", partial: true},
			{name: "partial_unknown_reason", reason: "unknown", partial: true},
			{name: "output_limit", reason: "max_output_tokens"},
			{name: "content_filter", reason: "content_filter"},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				c, rec := newNonStreamingFailoverContext(t)
				svc := &OpenAIGatewayService{cfg: &config.Config{}}
				account := newNonStreamingFailoverAccount()
				body := `{"id":"resp_incomplete","object":"response","status":"incomplete","output":[],"incomplete_details":null,"usage":{"input_tokens":5,"output_tokens":3}}`
				if tc.reason != "" {
					body = strings.Replace(body, `"incomplete_details":null`, `"incomplete_details":{"reason":"`+tc.reason+`"}`, 1)
				}
				if tc.partial {
					body = strings.Replace(body, `"output":[]`, `"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]`, 1)
				}
				resp := newNonStreamingSSEResponse()
				sse := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_incomplete\",\"status\":\"in_progress\"}}\n\n"
				if tc.partial {
					sse += "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"
				}
				sse += "event: response.incomplete\ndata: {\"type\":\"response.incomplete\",\"response\":" + body + "}\n\n"
				resp.Body = io.NopCloser(strings.NewReader(sse))
				var err error
				var usage *OpenAIUsage
				var nativeStream *openaiStreamingResult
				var passthroughStream *openaiStreamingResultPassthrough
				var nativeJSON *openaiNonStreamingResult
				var passthroughJSON *openaiNonStreamingResultPassthrough
				switch mode {
				case "native_stream":
					nativeStream, err = svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "model", "model")
				case "passthrough_stream":
					passthroughStream, err = svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "model", "model")
				case "native_sse_json":
					nativeJSON, err = svc.handleSSEToJSON(resp, c, account, []byte(sse), "model", "model")
				case "passthrough_sse_json":
					passthroughJSON, err = svc.handlePassthroughSSEToJSON(resp, c, account, []byte(sse), "model", "model")
				case "native_json", "passthrough_json":
					resp.Header.Set("Content-Type", "application/json")
					resp.Body = io.NopCloser(strings.NewReader(body))
					if mode == "native_json" {
						nativeJSON, err = svc.handleNonStreamingResponse(c.Request.Context(), resp, c, account, "model", "model")
					} else {
						passthroughJSON, err = svc.handleNonStreamingResponsePassthrough(c.Request.Context(), resp, c, account, "model", "model")
					}
				}
				if nativeStream != nil {
					usage = nativeStream.usage
				}
				if passthroughStream != nil {
					usage = passthroughStream.usage
				}
				if nativeJSON != nil {
					usage = nativeJSON.usage
				}
				if passthroughJSON != nil {
					usage = passthroughJSON.usage
				}
				if tc.retry {
					var failover *UpstreamFailoverError
					require.ErrorAs(t, err, &failover)
					require.False(t, failover.RetryableOnSameAccount)
					require.Equal(t, http.StatusBadGateway, failover.StatusCode)
					require.False(t, c.Writer.Written())
					require.Empty(t, rec.Body.String())
					require.Empty(t, GetOpsStreamErrors(c), "a recovered attempt must not mark the entire request failed")
					return
				}
				require.NoError(t, err)
				require.NotNil(t, usage, "preserve billing for an incomplete response with output")
				require.Equal(t, 5, usage.InputTokens)
				require.Equal(t, 3, usage.OutputTokens)
				require.Contains(t, rec.Body.String(), `"incomplete"`)
				if tc.reason != "" {
					require.Contains(t, rec.Body.String(), `"reason":"`+tc.reason+`"`)
				}
				if tc.partial {
					require.Contains(t, rec.Body.String(), "hello")
					require.NotContains(t, rec.Body.String(), "max_output_tokens")
					errors := GetOpsStreamErrors(c)
					require.Len(t, errors, 1)
					require.Equal(t, "upstream_response_incomplete", errors[0].Code)
					require.True(t, errors[0].CountTowardsSLA)
				} else {
					require.Empty(t, GetOpsStreamErrors(c))
				}
			})
		}
	}
}

func TestOpenAIIncompleteResponseReplayGuards(t *testing.T) {
	for _, tc := range []struct {
		name, payload           string
		outputStarted, canceled bool
	}{
		{name: "already_delivered", payload: `{"status":"incomplete","output":[]}`, outputStarted: true},
		{name: "tool_call", payload: `{"status":"incomplete","output":[{"type":"function_call","name":"charge_card","arguments":"{}"}]}`},
		{name: "encrypted_reasoning", payload: `{"status":"incomplete","output":[{"type":"reasoning","encrypted_content":"opaque"}]}`},
		{name: "malformed_output", payload: `{"status":"incomplete","output":{"type":"function_call"}}`},
		{name: "policy_error", payload: `{"status":"incomplete","error":{"code":"content_policy_violation","type":"invalid_request_error","message":"blocked by safety policy"}}`},
		{name: "client_canceled", payload: `{"status":"incomplete","output":[]}`, canceled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newNonStreamingFailoverContext(t)
			if tc.canceled {
				ctx, cancel := context.WithCancel(c.Request.Context())
				cancel()
				c.Request = c.Request.WithContext(ctx)
			}
			svc := newNonStreamingFailoverService()
			err := svc.handleOpenAIIncompleteResponse(c, newNonStreamingFailoverAccount(), newNonStreamingSSEResponse(), false, "", []byte(tc.payload), tc.outputStarted, "model")
			require.Nil(t, err)
			if tc.canceled {
				require.Empty(t, GetOpsStreamErrors(c))
			}
		})
	}
}

func TestOpenAIIncompleteSSEDetectsBufferedToolProgress(t *testing.T) {
	body := "data: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{}\"}\n\n" +
		"data: {\"type\":\"response.incomplete\",\"response\":{\"status\":\"incomplete\",\"output\":[]}}\n\n"
	require.True(t, openAIIncompleteSSEHasOutput(body))
	require.False(t, openAIIncompleteSSEHasOutput("data: {\"type\":\"response.created\"}\n\n"))
}

func TestOpenAIIncompleteSSEWithBufferedToolOutputDoesNotReplay(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		c, rec := newNonStreamingFailoverContext(t)
		svc := newNonStreamingFailoverService()
		svc.cfg.Gateway.LogUpstreamErrorBody = true
		body := []byte("data: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"private-tool-argument\"}\n\n" +
			"data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_buffered\",\"status\":\"incomplete\",\"output\":[],\"usage\":{\"input_tokens\":3,\"output_tokens\":2}}}\n\n")
		var err error
		if passthrough {
			_, err = svc.handlePassthroughSSEToJSON(newNonStreamingSSEResponse(), c, newNonStreamingFailoverAccount(), body, "model", "model")
		} else {
			_, err = svc.handleSSEToJSON(newNonStreamingSSEResponse(), c, newNonStreamingFailoverAccount(), body, "model", "model")
		}
		require.NoError(t, err)
		require.Contains(t, rec.Body.String(), `"incomplete"`)
		require.Len(t, GetOpsStreamErrors(c), 1)
		detail, _ := c.Get(OpsUpstreamErrorDetailKey)
		require.NotContains(t, detail, "private-tool-argument")
	}
}
