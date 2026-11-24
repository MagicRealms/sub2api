package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/util/errorprivacy"
	"github.com/gin-gonic/gin"
)

type privacyRawCapture struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *privacyRawCapture) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func TestPublicErrorPrivacyRetainsRawDiagnosticsAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(PublicErrorPrivacy())
	var raw string
	engine.Use(func(c *gin.Context) {
		original := c.Writer
		capture := &privacyRawCapture{ResponseWriter: original}
		c.Writer = capture
		c.Next()
		raw = capture.body.String()
		c.Writer = original
	})
	engine.GET("/", func(c *gin.Context) {
		errorprivacy.MarkUpstream(c.Request.Context())
		c.Set("ops_upstream_status_code", 400)
		c.Header("Retry-After", "7")
		c.Header("X-Request-ID", "req_private")
		c.Header("Location", "https://sharellm.net/login")
		c.Header("WWW-Authenticate", `Bearer realm="https://sharellm.net"`)
		c.Header("X-Upstream-Host", "sharellm.net")
		c.JSON(400, gin.H{"error": gin.H{"type": "invalid_request_error", "code": "invalid_value", "param": "input", "message": "private-vendor says invalid input", "provider": "sharellm.net"}})
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != 400 || response.Header().Get("Retry-After") != "7" || response.Header().Get("X-Request-ID") != "req_private" {
		t.Fatalf("status/headers lost: %v", response)
	}
	if !strings.Contains(raw, "sharellm.net") || !strings.Contains(raw, "private-vendor") {
		t.Fatalf("raw diagnostics altered: %s", raw)
	}
	if strings.Contains(response.Body.String(), "private-vendor") || strings.Contains(response.Body.String(), "sharellm") {
		t.Fatal(response.Body.String())
	}
	for _, name := range []string{"Location", "WWW-Authenticate", "X-Upstream-Host"} {
		if response.Header().Get(name) != "" {
			t.Fatalf("header leaked: %s", name)
		}
	}
	if !strings.Contains(response.Body.String(), `"param":"input"`) || !strings.Contains(response.Body.String(), `"code":"invalid_value"`) {
		t.Fatal(response.Body.String())
	}
}

func TestPublicErrorPrivacyFragmentsBoundsAndNormalSSE(t *testing.T) {
	normal := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"" + strings.Repeat("a", errorprivacy.MaxErrorBytes*2) + " https://example.org/\"}\n\n"
	errorEvent := "event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"server_error\",\"message\":\"https://sharellm.net timed out\"}}\n\n"
	oversized := "event: error\ndata: {\"type\":\"error\",\"error\":{\"message\":\"" + strings.Repeat("sharellm.net ", errorprivacy.MaxErrorBytes/2) + "\"}}\n\n"
	for _, input := range []string{normal + errorEvent, oversized + normal + errorEvent, ": ping\r\n\r\n" + errorEvent} {
		for _, chunk := range []int{1, 257, len(input)} {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Header("Content-Type", "text/event-stream")
			writer := &publicErrorWriter{ResponseWriter: c.Writer, status: 200, size: -1, state: &publicErrorPrivacyState{}}
			for start := 0; start < len(input); start += chunk {
				end := start + chunk
				if end > len(input) {
					end = len(input)
				}
				if _, err := writer.WriteString(input[start:end]); err != nil {
					t.Fatal(err)
				}
				if len(writer.pending) > errorprivacy.MaxErrorBytes || cap(writer.pending) > errorprivacy.MaxErrorBytes {
					t.Fatal("unbounded event")
				}
			}
			writer.finish()
			got := recorder.Body.String()
			if strings.Contains(input, normal) && !strings.Contains(got, normal) {
				t.Fatal("large normal delta modified")
			}
			got = strings.ReplaceAll(got, normal, "")
			if strings.Contains(got, "sharellm") || !strings.Contains(got, "codex.magicrealms.top") {
				t.Fatalf("error leak/missing projection (chunk=%d): %.300s", chunk, got)
			}
		}
	}
}

func TestPublicErrorPrivacyScrubsNormalSSEShareLLMOnly(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Header("Content-Type", "text/event-stream")
	writer := &publicErrorWriter{ResponseWriter: c.Writer, status: 200, size: -1, state: &publicErrorPrivacyState{}}
	_, _ = writer.WriteString("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"sharellm.net https://api.deepseek.com\"}\n\n")
	writer.finish()
	if strings.Contains(recorder.Body.String(), "sharellm") || !strings.Contains(recorder.Body.String(), "api.deepseek.com") || !strings.Contains(recorder.Body.String(), errorprivacy.PublicHost) {
		t.Fatal(recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Body.String(), "event: response.output_text.delta\n") || strings.Contains(recorder.Body.String(), "event: error") {
		t.Fatal("normal delta became an error: " + recorder.Body.String())
	}
}

func TestPublicIdentityProjectsNestedUserViewsWithoutChangingRouting(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicIdentity())
	raw := gin.H{"platform": "sharellm", "name": "ShareLLM group", "models": []string{"gpt-5", "https://api.deepseek.com"}, "id": int64(9007199254740993123)}
	engine.GET("/", func(c *gin.Context) { c.JSON(200, gin.H{"data": raw}) })
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(strings.ToLower(response.Body.String()), "sharellm") || !strings.Contains(response.Body.String(), `"platform":"route_relay"`) || !strings.Contains(response.Body.String(), "9007199254740993123") || raw["platform"] != "sharellm" {
		t.Fatal(response.Body.String())
	}
}

func TestPublicIdentityPlatformFiltersUseOriginalIDsAfterAliasSelection(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicIdentity())
	engine.GET("/", func(c *gin.Context) {
		if c.Query("platform") != "openai,sharellm" || c.Query("model") != "route_relay" {
			t.Errorf("unexpected query mapping: %s", c.Request.URL.RawQuery)
		}
		c.JSON(200, gin.H{"platform": "sharellm"})
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest("GET", "/?platform=openai,route_relay&model=route_relay", nil))
	if !strings.Contains(response.Body.String(), `"platform":"route_relay"`) {
		t.Fatal(response.Body.String())
	}
}

func TestPublicErrorPrivacyProjectsSplitSSEDeltasAndKeepsEventMetadata(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Header("Content-Type", "text/event-stream")
	writer := &publicErrorWriter{ResponseWriter: c.Writer, status: 200, size: -1, state: &publicErrorPrivacyState{}}
	for _, delta := range []string{"sha", "rellm.net"} {
		_, err := writer.WriteString("event: response.output_text.delta\r\nid: evt-1\r\n: keep\r\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"" + delta + "\"}\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
	}
	writer.finish()
	body := recorder.Body.String()
	if strings.Count(body, "event: response.output_text.delta\r\n") != 2 || strings.Count(body, "id: evt-1\r\n: keep\r\n") != 2 || !strings.Contains(body, errorprivacy.PublicRouteAlias) || strings.Contains(body, "event: error") {
		t.Fatal(body)
	}
}

func TestPublicErrorPrivacyDoesNotModifyBinaryDownloads(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicErrorPrivacy())
	input := []byte("\x00\xffsharellm.net\x80")
	engine.GET("/", func(c *gin.Context) { c.Data(200, "application/octet-stream", input) })
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	if !bytes.Equal(response.Body.Bytes(), input) {
		t.Fatal("binary download was modified")
	}
}

func TestPublicErrorPrivacyNonJSONGatewayErrorKeepsGenericFallback(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicErrorPrivacy())
	engine.GET("/", func(c *gin.Context) {
		c.Data(502, "text/html", []byte("<html>sharellm.net diagnostic at 203.0.113.9</html>"))
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	if !bytes.Equal(response.Body.Bytes(), errorprivacy.Fallback()) || strings.Contains(response.Body.String(), "203.0.113.9") {
		t.Fatal(response.Body.String())
	}
}

func TestPublicErrorPrivacyPreservesErrorSSEStatus(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicErrorPrivacy())
	engine.GET("/", func(c *gin.Context) {
		errorprivacy.MarkUpstream(c.Request.Context())
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteHeader(429)
		_, _ = c.Writer.WriteString("event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"rate_limit_exceeded\",\"message\":\"sharellm.net rate limit\"}}\n\n")
		c.Writer.Flush()
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != 429 || response.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(response.Body.String(), "event: error") || strings.Contains(response.Body.String(), "sharellm") {
		t.Fatalf("%v %s", response.Code, response.Body.String())
	}
}

func TestPublicErrorPrivacyStreamsUnknownSupplierWithoutHidingLocalPolicy(t *testing.T) {
	for _, upstream := range []bool{false, true} {
		engine := gin.New()
		engine.Use(PublicErrorPrivacy())
		engine.GET("/", func(c *gin.Context) {
			if upstream {
				errorprivacy.MarkUpstream(c.Request.Context())
			}
			c.Header("Content-Type", "text/event-stream")
			_, _ = c.Writer.WriteString("event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"invalid_value\",\"message\":\"PrivateVendor rejected field\"}}\n\n")
			c.Writer.Flush()
		})
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if strings.Contains(response.Body.String(), "PrivateVendor") == upstream {
			t.Fatal(response.Body.String())
		}
	}
}

func TestPublicIdentityPreservesLocalPlainTextErrors(t *testing.T) {
	engine := gin.New()
	engine.Use(PublicIdentity())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusBadRequest, "ShareLLM is not a valid public group")
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "ShareLLM") || !strings.Contains(response.Body.String(), errorprivacy.PublicRouteAlias) {
		t.Fatalf("local error was not projected: %d %q", response.Code, response.Body.String())
	}
}

func TestPublicErrorPrivacyScrubsSplitLargeNormalSSEIdentity(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Header("Content-Type", "text/event-stream")
	writer := &publicErrorWriter{ResponseWriter: c.Writer, status: 200, size: -1, state: &publicErrorPrivacyState{}}
	prefix := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"" + strings.Repeat("a", errorprivacy.MaxErrorBytes) + " share"
	for _, part := range []string{prefix, "llm.net\"}\n\n"} {
		if _, err := writer.WriteString(part); err != nil {
			t.Fatal(err)
		}
	}
	writer.finish()
	if strings.Contains(strings.ToLower(recorder.Body.String()), "sharellm") || !strings.Contains(recorder.Body.String(), errorprivacy.PublicHost) {
		t.Fatalf("split identity leaked: %q", recorder.Body.String()[max(0, len(recorder.Body.String())-160):])
	}
}

func BenchmarkPublicErrorPrivacySSE(b *testing.B) {
	gin.SetMode(gin.TestMode)
	for _, size := range []int{1024, 16 << 10, 256 << 10} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			input := []byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"" + strings.Repeat("a", size) + "\"}\n\n")
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Header("Content-Type", "text/event-stream")
				writer := &publicErrorWriter{ResponseWriter: c.Writer, status: 200, size: -1, state: &publicErrorPrivacyState{}}
				for start := 0; start < len(input); start += 1024 {
					end := start + 1024
					if end > len(input) {
						end = len(input)
					}
					_, _ = writer.Write(input[start:end])
				}
				writer.finish()
			}
		})
	}
}

func TestPublicErrorPrivacyOversizedHTTPFlushAndLateWrites(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writer := &publicErrorWriter{ResponseWriter: c.Writer, status: 502, size: -1, state: &publicErrorPrivacyState{}}
	for i := 0; i < 20; i++ {
		_, _ = writer.WriteString(strings.Repeat("sharellm.net ", 1000))
		writer.Flush()
		if len(writer.pending) > errorprivacy.MaxErrorBytes {
			t.Fatal("unbounded HTTP error")
		}
	}
	writer.finish()
	if recorder.Code != 502 || strings.Contains(recorder.Body.String(), "sharellm") {
		t.Fatal(recorder.Body.String())
	}
	if _, err := writer.WriteString("late secret"); err == nil {
		t.Fatal("late write allowed")
	}
}

func TestPublicErrorPrivacyConcurrentKeepaliveAndWrite(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Header("Content-Type", "text/event-stream")
	writer := &publicErrorWriter{ResponseWriter: c.Writer, status: 200, size: -1, state: &publicErrorPrivacyState{}}
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		for i := 0; i < 10; i++ {
			_, _ = writer.WriteString(": ping\n\n")
		}
	}()
	go func() {
		defer group.Done()
		for i := 0; i < 10; i++ {
			writer.Flush()
		}
	}()
	group.Wait()
	writer.finish()
	if strings.Count(recorder.Body.String(), ": ping") != 10 {
		t.Fatal(recorder.Body.String())
	}
}

type privacyDisconnectedWriter struct{ gin.ResponseWriter }

func (w privacyDisconnectedWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("client disconnected")
}

func TestPublicErrorPrivacyStopsWritingAfterDisconnect(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Header("Content-Type", "text/event-stream")
	writer := &publicErrorWriter{ResponseWriter: privacyDisconnectedWriter{c.Writer}, status: 200, size: -1, state: &publicErrorPrivacyState{}}
	if _, err := writer.WriteString(": ping\n\n"); err == nil {
		t.Fatal("disconnect swallowed")
	}
	if _, err := writer.WriteString("event: error\ndata: secret\n\n"); err == nil {
		t.Fatal("continued writing after disconnect")
	}
	writer.finish()
	if len(writer.pending) != 0 {
		t.Fatal("pending data retained after request")
	}
}
