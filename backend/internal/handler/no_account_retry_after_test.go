//go:build unit

package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type selectionRetryAfterTestError struct{ delay time.Duration }

func (e selectionRetryAfterTestError) Error() string             { return "no available accounts" }
func (e selectionRetryAfterTestError) RetryAfter() time.Duration { return e.delay }

func TestClassifySelectionFailureErrorCoolingPool(t *testing.T) {
	fallback := noAccountErrorClassification{Status: http.StatusServiceUnavailable, ErrType: "api_error", Message: "Service temporarily unavailable"}
	got := classifySelectionFailureError(selectionRetryAfterTestError{delay: 10 * time.Second}, fallback)
	require.Equal(t, http.StatusServiceUnavailable, got.Status)
	require.Contains(t, got.Message, "cooling down")
	require.Equal(t, 10, got.RetryAfterSeconds)
}

func TestSelectionRetryAfterHeaderSafetyAndClassification(t *testing.T) {
	for _, tc := range []struct {
		name  string
		delay time.Duration
		want  string
	}{
		{"positive", 10 * time.Second, "10"}, {"fraction_rounds_up", 1500 * time.Millisecond, "2"},
		{"expired", -time.Second, ""}, {"zero", 0, ""}, {"unbounded", time.Hour, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(r)
			fallback := noAccountErrorClassification{Status: 503, ErrType: "api_error", Message: "Service temporarily unavailable"}
			got := classifySelectionFailureErrorFromGin(c, fmt.Errorf("wrapped: %w", selectionRetryAfterTestError{tc.delay}), fallback)
			require.Equal(t, tc.want, r.Header().Get("Retry-After"))
			require.Equal(t, 503, got.Status)
		})
	}
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	unsupported := noAccountErrorClassification{Status: 404, ErrType: "model_not_found", ModelNotFound: true}
	require.Equal(t, unsupported, classifySelectionFailureErrorFromGin(c, selectionRetryAfterTestError{10 * time.Second}, unsupported))
	require.Empty(t, r.Header().Get("Retry-After"))
	c.Status(200)
	c.Writer.WriteHeaderNow()
	classifySelectionFailureErrorFromGin(c, selectionRetryAfterTestError{10 * time.Second}, noAccountErrorClassification{Status: 503})
	require.Empty(t, r.Header().Get("Retry-After"), "do not pretend headers can change after streaming starts")
}

func TestGenericFailoverExhaustionPreservesSafeRetryAfter(t *testing.T) {
	for _, value := range []string{"60", "120", "refresh_token=must-not-leak", "1\r\nInjected: secret"} {
		r := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(r)
		h := &GatewayHandler{}
		h.handleFailoverExhausted(c, &service.UpstreamFailoverError{StatusCode: 502, ResponseHeaders: http.Header{"Retry-After": []string{value}, "Set-Cookie": []string{"secret=value"}}}, service.PlatformAnthropic, false)
		if value == "60" || value == "120" {
			require.Equal(t, value, r.Header().Get("Retry-After"))
		} else {
			require.Empty(t, r.Header().Get("Retry-After"))
		}
		require.Empty(t, r.Header().Get("Set-Cookie"))
		require.Equal(t, 502, r.Code)
	}
}
