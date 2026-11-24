package errorprivacy

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestErrorProjectionPreservesProtocolAndHidesSupplier(t *testing.T) {
	cases := []string{
		`{"error":{"type":"invalid_request_error","code":"context_length_exceeded","param":"input","message":"ShareLLM context window at https://sharellm.net/v1/responses exceeded","provider":"private-vendor","details":{"host":"203.0.113.1"}},"provider":"private-vendor","request_id":"req_123"}`,
		`{"type":"response.failed","sequence_number":3,"response":{"id":"resp_1","status":"failed","instructions":"secret","error":{"code":"context_length_exceeded","message":"ShareLLM context length exceeded"}}}`,
		`{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"private-vendor rate limit","details":[{"metadata":{"service":"sharellm.net"}}]}}`,
		`{"type":"error","error":"Adaptive Responses endpoint request failed: Post https://sharellm.net/v1/responses: timeout"}`,
	}
	for _, input := range cases {
		output, isError := JSON([]byte(input), true)
		if !isError || !json.Valid(output) {
			t.Fatalf("invalid projection: %s", output)
		}
		for _, secret := range []string{"sharellm", "ShareLLM", "private-vendor", "203.0.113.1", "instructions", "details"} {
			if strings.Contains(string(output), secret) {
				t.Fatalf("leaked %s: %s", secret, output)
			}
		}
		if !strings.Contains(string(output), "codex.magicrealms.top") {
			t.Fatalf("missing site: %s", output)
		}
		if strings.Contains(input, `"type":"response.failed"`) && !strings.Contains(string(output), `"type":"response.failed"`) {
			t.Fatal("response.failed type changed")
		}
	}
	input := []byte(cases[0])
	output, _ := JSON(input, true)
	var root struct {
		Error     struct{ Type, Code, Param, Message string }
		RequestID string `json:"request_id"`
	}
	_ = json.Unmarshal(output, &root)
	if root.Error.Type != "invalid_request_error" || root.Error.Code != "context_length_exceeded" || root.Error.Param != "input" || root.RequestID != "req_123" || !strings.Contains(root.Error.Message, "prompt is too long") {
		t.Fatalf("protocol lost: %s", output)
	}
	if !strings.Contains(string(input), "sharellm.net") {
		t.Fatal("mutated diagnostic input")
	}
	parameterOutput, _ := JSON([]byte(`{"error":{"type":"invalid_request_error","code":"invalid_value","param":"input.0.content.0.image_url","message":"invalid input"}}`), true)
	if !strings.Contains(string(parameterOutput), `"param":"input.0.content.0.image_url"`) {
		t.Fatal("parameter path changed")
	}
}

func TestSuccessfulOutputAndLocalValidationRemainIntact(t *testing.T) {
	output, changed := PublicJSON([]byte(`{"id":"resp_ok","error":null,"output":[{"text":"Visit https://sharellm.net/ or https://example.org"}]}`), true)
	if !changed || strings.Contains(string(output), "sharellm") || !strings.Contains(string(output), PublicHost) || !strings.Contains(string(output), "https://example.org") {
		t.Fatalf("hidden identity or normal URL was lost: %s", output)
	}
	for _, input := range []string{
		`{"type":"response.output_text.delta","delta":"https://example.com is a normal citation"}`,
		`{"data":[{"url":"https://image.example.com/result.png"}]}`,
	} {
		output, changed := PublicJSON([]byte(input), true)
		if changed || string(output) != input {
			t.Fatalf("normal output changed: %s", output)
		}
	}
	output, _ = JSON([]byte(`{"error":{"type":"invalid_request_error","code":"invalid_value","param":"temperature","message":"temperature must be between 0 and 2"}}`), false)
	if !strings.Contains(string(output), "temperature must be between 0 and 2") {
		t.Fatalf("validation lost: %s", output)
	}
}

func TestPublicJSONScrubsOnlyShareLLMIdentity(t *testing.T) {
	input := []byte(`{"platform":"sharellm","owned_by":"ShareLLM","url":"https://sharellm.net/v1","other":"https://api.deepseek.com"}`)
	output, changed := PublicJSON(input, true)
	if !changed || strings.Contains(strings.ToLower(string(output)), "sharellm") || !strings.Contains(string(output), "api.deepseek.com") || !strings.Contains(string(output), PublicRouteAlias) {
		t.Fatalf("unexpected targeted projection: %s", output)
	}
}

func TestPublicJSONEscapesKeysAndLargeIntegers(t *testing.T) {
	input := []byte(`{"id":9007199254740993123,"platform":"\u0073harellm","sharellm":{"nested":["https:\/\/sharellm.net\/v1","https://api.deepseek.com"]},"error":null}`)
	output, changed := PublicJSON(input, false)
	if !changed || strings.Contains(strings.ToLower(string(output)), "sharellm") || !strings.Contains(string(output), `"id":9007199254740993123`) || !strings.Contains(string(output), `"platform":"route_relay"`) || !strings.Contains(string(output), `"route_relay":`) || !strings.Contains(string(output), "https://api.deepseek.com") {
		t.Fatalf("unexpected projection: %s", output)
	}
	if !strings.Contains(string(input), "sharellm") {
		t.Fatal("input was mutated")
	}
}

func TestPublicJSONRejectsIdentityInErrorParameter(t *testing.T) {
	output, _ := PublicJSON([]byte(`{"error":{"type":"invalid_request_error","param":"sharellm.net","message":"bad input"}}`), false)
	if strings.Contains(strings.ToLower(string(output)), "sharellm") {
		t.Fatal(string(output))
	}
}

func TestLocalProtocolFieldErrorsRemainActionable(t *testing.T) {
	for _, field := range []string{"reasoning.effort", "thinking.type", "session.model", "input[0].role"} {
		message := "Request body declares field \"" + field + "\" more than once"
		if Text(message) != message {
			t.Fatalf("local validation hidden: %q", field)
		}
	}
	for _, message := range []string{"sharellm.net", `Request failed at "sharellm.net"`, `input.sharellm.net`} {
		if strings.Contains(strings.ToLower(Text(message)), "sharellm") {
			t.Fatal(message)
		}
	}
}

func TestTextAndCloseReason(t *testing.T) {
	if reason := "overlapping response.create is not supported"; CloseReason(reason) != reason {
		t.Fatal("local protocol instruction changed")
	}
	for _, raw := range []string{
		`Post "https://user:secret@sharellm.net/v1/responses?key=secret": timeout`,
		"dial tcp 203.0.113.9:443: timeout", "dial tcp [2001:db8::1]:443: timeout", "supplier.internal failed", "Authorization: Bearer secret", "private-vendor failed at https://proxy.example.org",
	} {
		if got := Text(raw); got == raw || !strings.Contains(got, "codex.magicrealms.top") {
			t.Fatalf("not hidden: %s", got)
		}
	}
	reason := CloseReason(strings.Repeat("中", 100))
	if len(reason) > 120 || !utf8.ValidString(reason) {
		t.Fatalf("invalid close reason: %q", reason)
	}
	if got := Body(`<html>sharellm.net at 203.0.113.9</html>`, true); !json.Valid([]byte(got)) || strings.Contains(got, "sharellm.net") {
		t.Fatal(got)
	}
}

func TestSSEMultilineAndNormalContent(t *testing.T) {
	input := "event: response.failed\r\ndata: {\"type\":\"response.failed\",\r\ndata: \"response\":{\"error\":{\"code\":\"context_length_exceeded\",\"message\":\"sharellm.net context window exceeded\"}}}\r\n\r\n"
	output := string(SSE([]byte(input)))
	if strings.Contains(output, "sharellm") || !strings.Contains(output, "context_length_exceeded") || !strings.HasSuffix(output, "\n\r\n") {
		t.Fatal(output)
	}
	for _, normal := range []string{": heartbeat\n\n", "data: [DONE]\n\n", "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"https://api.deepseek.com\"}\n\n"} {
		if got := string(SSE([]byte(normal))); got != normal {
			t.Fatalf("normal SSE changed: %q", got)
		}
	}
	normalWithHiddenIdentity := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"sharellm.net\"}\n\n"
	if got := string(SSE([]byte(normalWithHiddenIdentity))); strings.Contains(got, "sharellm") || !strings.Contains(got, PublicHost) {
		t.Fatalf("normal SSE identity was not scrubbed: %q", got)
	}
}

func TestTaskErrorPreservesPublicPollingFields(t *testing.T) {
	input := []byte(`{"id":"task_1","task_id":"task_1","object":"image.generation.task","status":"failed","http_status":502,"created_at":1,"completed_at":2,"expires_at":3,"error":{"code":"timeout","message":"sharellm.net timed out"}}`)
	projected, _ := JSON(input, true)
	var fields map[string]any
	_ = json.Unmarshal(projected, &fields)
	if fields["object"] != "image.generation.task" || fields["task_id"] != "task_1" || fields["http_status"] != float64(502) || fields["expires_at"] != float64(3) || strings.Contains(string(projected), "sharellm") {
		t.Fatal(string(projected))
	}
}

func TestHeaderPrivacyPreservesBothRetryAfterFormats(t *testing.T) {
	for _, value := range []string{"7", "Mon, 23 Nov 2026 08:45:00 GMT"} {
		headers := http.Header{"Retry-After": []string{value}, "X-Request-Id": []string{"req_123"}, "X-Private-Debug": []string{"https://sharellm.net/v1/responses"}}
		Headers(headers, true)
		if headers.Get("Retry-After") != value || headers.Get("X-Request-ID") != "req_123" || headers.Get("X-Private-Debug") != "" {
			t.Fatalf("%v", headers)
		}
	}
}

func TestSSEErrorCommentsAndMalformedPayloadNeverReflectDiagnostic(t *testing.T) {
	for _, input := range []string{
		"event: error\n: sharellm.net diagnostic\ndata: {\"type\":\"error\",\"message\":\"timeout\"}\n\n",
		"event: error\ndata: {\"message\":\"sharellm.net truncated\n\n",
	} {
		if output := string(SSE([]byte(input))); strings.Contains(output, "sharellm") || !strings.Contains(output, "codex.magicrealms.top") {
			t.Fatal(output)
		}
	}
}
