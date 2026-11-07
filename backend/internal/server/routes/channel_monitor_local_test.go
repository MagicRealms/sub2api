//go:build unit

package routes

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type localProbeRepo struct {
	service.ChannelMonitorRepository
	monitor *service.ChannelMonitor
}

func (r *localProbeRepo) GetByID(context.Context, int64) (*service.ChannelMonitor, error) {
	m := *r.monitor
	return &m, nil
}
func (r *localProbeRepo) MarkChecked(context.Context, int64, time.Time) error { return nil }
func (r *localProbeRepo) InsertHistoryBatch(context.Context, []*service.ChannelMonitorHistoryRow) error {
	return nil
}

type localProbeKeyRepo struct{ orderedRouteKeyRepo }

func (r localProbeKeyRepo) GetByID(context.Context, int64) (*service.APIKey, error) {
	return r.key, nil
}

type localProbeCipher struct{}

func (localProbeCipher) Encrypt(s string) (string, error) { return s, nil }
func (localProbeCipher) Decrypt(s string) (string, error) { return s, nil }

type localProbeRuntime struct{}

func (localProbeRuntime) GetChannelMonitorRuntime(context.Context) service.ChannelMonitorRuntime {
	return service.ChannelMonitorRuntime{Enabled: true, Mode: service.ChannelMonitorModeV1}
}

func TestLocalMonitorProbePinsSelectedGroupAndNeverFallsBack(t *testing.T) {
	for _, tc := range []struct {
		name          string
		role          string
		selected      int64
		onlyOne       bool
		expectedCalls int
	}{
		{"secondary", service.RoleAdmin, 2, false, 1},
		{"first_error_no_fallback", service.RoleAdmin, 1, false, 1},
		{"single_group", service.RoleAdmin, 1, true, 1},
		{"unselected_group", service.RoleAdmin, 99, false, 0},
		{"non_admin", "user", 2, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groups := orderedTestGroups()
			if tc.onlyOne {
				groups = groups[:1]
			}
			ids := []int64{}
			byID := map[int64]*service.Group{}
			for _, g := range groups {
				ids = append(ids, g.ID)
				byID[g.ID] = g
			}
			key := &service.APIKey{ID: 10, UserID: 9, Key: "sk-internal-local-monitor", GroupID: &groups[0].ID, GroupIDs: ids, Group: groups[0], Status: service.StatusActive, User: &service.User{ID: 9, Role: tc.role, Status: service.StatusActive, Balance: 100}}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			keys := service.NewAPIKeyService(localProbeKeyRepo{orderedRouteKeyRepo{key: key}}, nil, orderedRouteGroupRepo{groups: byID}, nil, nil, nil, cfg)
			engine := gin.New()
			engine.Use(orderedKeyGroupRoutingWithSupport(engine, keys, func(context.Context, *service.Group, string, string) (bool, error) { return true, nil }))
			calls := []int64{}
			engine.POST("/v1/responses", gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg)), func(c *gin.Context) {
				effective, ok := middleware.GetAPIKeyFromContext(c)
				require.True(t, ok)
				calls = append(calls, *effective.GroupID)
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"type": "api_error", "message": "overloaded"}})
			})
			repo := &localProbeRepo{monitor: &service.ChannelMonitor{ID: 1, CreatedBy: 9, Provider: "openai", APIMode: "responses", Endpoint: "https://never-dial.invalid", APIKey: "sk-old-snapshot", PrimaryModel: "gpt-6.1-sol", LocalAPIKeyID: &key.ID, ProbeGroupID: &tc.selected}}
			monitor := service.NewChannelMonitorService(repo, localProbeCipher{})
			monitor.SetRuntimeReader(localProbeRuntime{})
			monitor.SetLocalProbeGateway(engine, keys)
			result, err := monitor.RunCheck(context.Background(), 1)
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Len(t, calls, tc.expectedCalls)
			if tc.expectedCalls > 0 {
				require.Equal(t, tc.selected, calls[0])
				require.Equal(t, service.MonitorStatusError, result[0].Status)
			}
		})
	}
}
