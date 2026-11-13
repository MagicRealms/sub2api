package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelMonitorV2GroupAuthorizerStub struct {
	groups []service.Group
	err    error
	calls  []int64
}

type channelMonitorV2PublicGroupListerStub struct {
	groups []int64
	err    error
}

func (s *channelMonitorV2PublicGroupListerStub) ListPublicActiveGroupIDs(context.Context) ([]int64, error) {
	return s.groups, s.err
}

func (s *channelMonitorV2GroupAuthorizerStub) GetAvailableGroups(_ context.Context, userID int64) ([]service.Group, error) {
	s.calls = append(s.calls, userID)
	return s.groups, s.err
}

func monitorV2HandlerWithPublicGroups(publicIDs []int64, authorizer *channelMonitorV2GroupAuthorizerStub) *ChannelMonitorV2Handler {
	return &ChannelMonitorV2Handler{
		service:           service.NewChannelMonitorV2Service(nil),
		publicGroupLister: &channelMonitorV2PublicGroupListerStub{groups: publicIDs},
		apiKeyService:     authorizer,
	}
}

func TestChannelMonitorV2QueryListSupportsRepeatedAndCommaValues(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest("GET", "/?platform=openai,grok&platform=anthropic", nil)
	require.Equal(t, []string{"openai", "grok", "anthropic"}, queryList(c, "platform"))
}

func TestChannelMonitorV2GroupByQueryDefaultsAndRejectsInvalid(t *testing.T) {
	groupBy, err := service.ParseChannelMonitorV2GroupBy("")
	require.NoError(t, err)
	require.Equal(t, service.ChannelMonitorV2GroupByPlatformGroup, groupBy)
	_, err = service.ParseChannelMonitorV2GroupBy("invalid")
	require.Error(t, err)
}

func TestChannelMonitorV2MatrixHandlerRejectsInvalidGroupBy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/matrix?group_by=invalid", nil)
	h := NewChannelMonitorV2Handler(service.NewChannelMonitorV2Service(nil), nil)
	h.Matrix(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestChannelMonitorV2ScopeFilterUsesAvailableGroupsForOrdinaryUser(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/snapshot", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	authorizer := &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 3}, {ID: 7}, {ID: 9, IsExclusive: true}}}
	h := monitorV2HandlerWithPublicGroups([]int64{3, 7}, authorizer)
	filter := service.ChannelMonitorV2Filter{GroupIDs: []int64{7, 9}}

	require.True(t, h.scopeFilter(c, &filter, false))
	require.True(t, filter.RestrictGroups)
	require.Equal(t, []int64{3, 7}, filter.AllowedGroupIDs)
	require.Equal(t, []int64{42}, authorizer.calls)
}

func TestChannelMonitorV2ScopeFilterPreservesEmptyOrdinaryUserScope(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/snapshot", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h := monitorV2HandlerWithPublicGroups(nil, &channelMonitorV2GroupAuthorizerStub{})
	filter := service.ChannelMonitorV2Filter{}

	require.True(t, h.scopeFilter(c, &filter, false))
	require.True(t, filter.RestrictGroups)
	require.Empty(t, filter.AllowedGroupIDs)
}

func TestChannelMonitorV2ScopeFilterRestrictsAdminToPublicGroups(t *testing.T) {
	authorizer := &channelMonitorV2GroupAuthorizerStub{}
	h := monitorV2HandlerWithPublicGroups([]int64{3, 7}, authorizer)
	filter := service.ChannelMonitorV2Filter{GroupIDs: []int64{9}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/channel-monitor-v2/snapshot", nil)

	require.True(t, h.scopeFilter(c, &filter, true))
	require.True(t, filter.RestrictGroups)
	require.Equal(t, []int64{3, 7}, filter.AllowedGroupIDs)
	require.Empty(t, authorizer.calls)
}
