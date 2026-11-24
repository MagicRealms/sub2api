package middleware

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/util/errorprivacy"
	"github.com/gin-gonic/gin"
)

// Public panel filters use the same platform alias returned in user views.
// Translate only those query fields; authorization still runs on real groups.
func normalizePublicPlatformFilters(c *gin.Context) {
	query := c.Request.URL.Query()
	changed := false
	for _, name := range []string{"platform", "platforms", "platform[]"} {
		for i, value := range query[name] {
			parts := strings.Split(value, ",")
			for j, part := range parts {
				if strings.TrimSpace(part) == errorprivacy.PublicRouteID {
					parts[j] = domain.PlatformShareLLM
					changed = true
				}
			}
			query[name][i] = strings.Join(parts, ",")
		}
	}
	if changed {
		c.Request.URL.RawQuery = query.Encode()
	}
}
