package service

import (
	"strings"
	"testing"
)

func TestEvaluateChannelIQResponseUsesPelicanRubric(t *testing.T) {
	response := `<!doctype html><svg viewBox="0 0 100 60"></svg>
<!-- CHANNEL_IQ_RUBRIC: {"wheels":"pass","leg_pedal":"pass","view_consistent":"pass","pelican_beak":"pass"} -->`

	evaluation := evaluateChannelIQResponse(response)
	if !evaluation.Graded || evaluation.Grade != "baseline" || evaluation.Score != 4 {
		t.Fatalf("expected baseline evaluation, got %+v", evaluation)
	}
}

func TestEvaluateChannelIQResponseMarksPartialAbility(t *testing.T) {
	response := `<!-- CHANNEL_IQ_RUBRIC: {"wheels":"pass","leg_pedal":"fail","view_consistent":"pass","pelican_beak":"pass"} -->`

	evaluation := evaluateChannelIQResponse(response)
	if !evaluation.Graded || evaluation.Grade != "mild" || evaluation.Score != 3 {
		t.Fatalf("expected mild degradation, got %+v", evaluation)
	}
}

func TestEvaluateChannelIQResponseFollowsModerateAndSevereMatrix(t *testing.T) {
	moderate := evaluateChannelIQResponse(`<!-- CHANNEL_IQ_RUBRIC: {"wheels":"pass","leg_pedal":"fail","view_consistent":"fail","pelican_beak":"partial"} -->`)
	if moderate.Grade != "moderate" || moderate.Score != 1 {
		t.Fatalf("expected moderate degradation, got %+v", moderate)
	}

	severe := evaluateChannelIQResponse(`<!-- CHANNEL_IQ_RUBRIC: {"wheels":"fail","leg_pedal":"fail","view_consistent":"fail","pelican_beak":"partial"} -->`)
	if severe.Grade != "severe" || severe.Score != 0 {
		t.Fatalf("expected severe degradation, got %+v", severe)
	}
}

func TestEvaluateChannelIQResponseDoesNotTreatUnmarkedOutputAsHealthy(t *testing.T) {
	evaluation := evaluateChannelIQResponse(`<svg viewBox="0 0 100 60"><circle cx="20" cy="30" r="10" /></svg>`)
	if evaluation.Graded || evaluation.Grade != "ungraded" || evaluation.Score != 0 {
		t.Fatalf("expected ungraded output, got %+v", evaluation)
	}
	for name, criterion := range map[string]ChannelIQCriterion{
		"wheels":          evaluation.Criteria.Wheels,
		"leg_pedal":       evaluation.Criteria.LegPedal,
		"view_consistent": evaluation.Criteria.ViewConsistent,
		"pelican_beak":    evaluation.Criteria.PelicanBeak,
	} {
		if criterion.Status != ChannelIQCriterionUnknown {
			t.Errorf("%s status = %q, want unknown", name, criterion.Status)
		}
	}
}

func TestChannelIQPromptForExecutionAddsHiddenRubric(t *testing.T) {
	prompt := channelIQPromptForExecution("创建鹈鹕骑自行车的 SVG")
	if prompt == "创建鹈鹕骑自行车的 SVG" || !strings.Contains(prompt, channelIQRubricMarker) {
		t.Fatalf("expected execution prompt to contain the hidden rubric: %q", prompt)
	}
}
