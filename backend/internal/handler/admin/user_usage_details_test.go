//go:build unit

package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserUsageDetailsRejectsInvalidParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/users/nope/usage-details", "/users/0/usage-details", "/users/-1/usage-details", "/users/42/usage-details?timezone=Not/A_Zone", "/users/42/usage-details?timezone=Local"} {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			router.GET("/users/:id/usage-details", (&UserHandler{}).GetUserUsageDetails)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}
