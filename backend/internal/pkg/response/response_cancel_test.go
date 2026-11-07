//go:build unit

package response

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestErrorFromCancelledDatabaseRequest(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	for _, cancelled := range []bool{true, false} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ctx, cancel := context.WithCancel(t.Context())
		c.Request = httptest.NewRequest("GET", "/api/v1/usage/dashboard/stats", nil).WithContext(ctx)
		if cancelled {
			cancel()
		}
		logs.Reset()
		require.True(t, ErrorFrom(c, errors.New("pq: canceling statement due to user request")))
		if cancelled {
			require.Equal(t, 499, w.Code)
			require.Empty(t, logs.String())
			require.Empty(t, w.Body.String())
		} else {
			require.Equal(t, 500, w.Code)
			require.Contains(t, logs.String(), "[ERROR]")
		}
		cancel()
	}
}
