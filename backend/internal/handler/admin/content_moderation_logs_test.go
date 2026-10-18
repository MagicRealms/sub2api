package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type moderationLogsRepository struct {
	service.ContentModerationRepository
	filter service.ContentModerationLogFilter
	calls  int
}

func (r *moderationLogsRepository) ListLogs(_ context.Context, filter service.ContentModerationLogFilter) ([]service.ContentModerationLog, *pagination.PaginationResult, error) {
	r.filter = filter
	r.calls++
	return []service.ContentModerationLog{}, &pagination.PaginationResult{Page: filter.Pagination.Page, PageSize: filter.Pagination.PageSize}, nil
}

func TestContentModerationLogsAttributionFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &moderationLogsRepository{}
	svc := service.NewContentModerationService(nil, repo, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/logs", NewContentModerationHandler(svc).ListLogs)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs?user_id=1001&api_key_id=42&group_id=3&search=prompt&page=2&page_size=10", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.calls)
	require.EqualValues(t, 1001, *repo.filter.UserID)
	require.EqualValues(t, 42, *repo.filter.APIKeyID)
	require.EqualValues(t, 3, *repo.filter.GroupID)
	require.Equal(t, "prompt", repo.filter.Search)
	require.Equal(t, 2, repo.filter.Pagination.Page)
}

func TestContentModerationLogsRejectInvalidAttributionIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, field := range []string{"user_id", "api_key_id", "group_id"} {
		for _, value := range []string{"0", "-1", "1.5", "not-an-id", "9223372036854775808"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				router := gin.New()
				router.GET("/logs", NewContentModerationHandler(nil).ListLogs)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logs?"+field+"="+value, nil))
				require.Equal(t, http.StatusBadRequest, w.Code)
				require.Contains(t, w.Body.String(), "Invalid "+field)
			})
		}
	}
}
