package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAdminGroupDTOUsesPerGroupSettingBeforeLegacyFallback(t *testing.T) {
	h := &GroupHandler{cfg: &config.Config{}}
	h.cfg.Gateway.OpenAIScheduler.StrictPriorityGroupIDs = []int64{6, 10}

	legacy := &service.Group{ID: 6}
	if got := h.adminGroupDTO(legacy).StrictPriorityEnabled; !got {
		t.Fatal("legacy configured groups should retain their effective enabled state")
	}

	disabled := false
	legacy.StrictPriorityEnabled = &disabled
	if got := h.adminGroupDTO(legacy).StrictPriorityEnabled; got {
		t.Fatal("an explicit group opt-out should override the legacy configuration")
	}

	enabled := true
	other := &service.Group{ID: 31, StrictPriorityEnabled: &enabled}
	if got := h.adminGroupDTO(other).StrictPriorityEnabled; !got {
		t.Fatal("an explicit group opt-in should work outside the legacy ID list")
	}
}
