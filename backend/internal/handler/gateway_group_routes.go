package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GroupRouteModels uses the same catalog and allowlist as normal discovery.
// A temporarily unhealthy account must not change the request's billing floor.
func (h *GatewayHandler) GroupRouteModels(ctx context.Context, group *service.Group) ([]string, error) {
	if group == nil {
		return nil, nil
	}
	return h.gatewayService.ConfiguredGroupRouteModels(ctx, group, defaultModelIDsForPlatform(group.Platform))
}

func (h *GatewayHandler) GroupRouteSupportsModel(ctx context.Context, group *service.Group, model, path string) (bool, error) {
	if group == nil || !group.ModelAllowlist.Allows(model) {
		return false, nil
	}
	if model == "" {
		return true, nil
	}
	if strings.Contains(path, "/images/") && !group.AllowImageGeneration {
		return false, nil
	}
	models, err := h.GroupRouteModels(ctx, group)
	if err != nil {
		return false, err
	}
	return (service.GroupModelAllowlist{Enabled: true, Models: models}).Allows(model), nil
}

func (h *GatewayHandler) OrderedGroupModels(c *gin.Context, groups []*service.Group) {
	models := make([]string, 0)
	seen := make(map[string]bool)
	for _, group := range groups {
		groupModels, err := h.GroupRouteModels(c.Request.Context(), group)
		if err != nil {
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Group model catalog is temporarily unavailable")
			return
		}
		for _, model := range groupModels {
			if !seen[model] && !strings.Contains(model, "*") {
				models = append(models, model)
				seen[model] = true
			}
		}
	}
	if c.Query("client_version") == "" {
		writeModelsList(c, service.PlatformComposite, models)
		return
	}
	group := &service.Group{Platform: service.PlatformComposite}
	body, err := h.gatewayService.BuildCodexModelsManifestForGroup(c.Request.Context(), group, "", models)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to build ordered group models")
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}
