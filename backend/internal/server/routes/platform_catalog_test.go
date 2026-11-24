//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type catalogUsers struct{ service.UserRepository }

func (*catalogUsers) GetByID(_ context.Context, id int64) (*service.User, error) {
	role := service.RoleUser
	if id == 1 {
		role = service.RoleAdmin
	}
	return &service.User{ID: id, Role: role, Status: service.StatusActive, TokenVersion: 2, TokenVersionResolved: true}, nil
}

func (*catalogUsers) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func TestPlatformCatalogRequiresAdministrator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "catalog-test-secret", ExpireHour: 1}}
	auth := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	users := service.NewUserService(&catalogUsers{}, nil, nil, nil)
	engine := gin.New()
	RegisterAdminRoutes(engine.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{}},
		middleware.NewAdminAuthMiddleware(auth, users, nil, nil),
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)
	for _, tc := range []struct {
		id     int64
		status int
	}{{0, http.StatusUnauthorized}, {2, http.StatusForbidden}, {1, http.StatusOK}} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/platform-catalog", nil)
		if tc.id != 0 {
			token, err := auth.GenerateToken(context.Background(), &service.User{ID: tc.id, TokenVersion: 2, TokenVersionResolved: true})
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+token)
		}
		result := httptest.NewRecorder()
		engine.ServeHTTP(result, req)
		require.Equal(t, tc.status, result.Code, result.Body.String())
		if tc.status == http.StatusOK {
			require.Equal(t, "private, no-store", result.Header().Get("Cache-Control"))
			require.Contains(t, result.Body.String(), "https://sharellm.net/v1")
		} else {
			require.NotContains(t, result.Body.String(), "sharellm")
		}
	}
}
