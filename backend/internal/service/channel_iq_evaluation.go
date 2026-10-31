package service

import (
	"encoding/json"
	"strings"
)

const channelIQRubricMarker = "CHANNEL_IQ_RUBRIC"

const channelIQRubricInstruction = `

完成 HTML 后，请在完整代码末尾追加一行 HTML 注释作为系统评分数据，不要把注释显示在画面中：
<!-- CHANNEL_IQ_RUBRIC: {"wheels":"pass","leg_pedal":"pass","view_consistent":"pass","pelican_beak":"pass"} -->
四个字段必须只填写 pass、partial 或 fail，并且只能根据最终 SVG 画面真实情况填写：wheels 检查前后两个车轮，leg_pedal 检查腿是否连接并踩到脚踏，view_consistent 检查鸟和自行车视角是否统一，pelican_beak 检查是否有鹈鹕下垂喉囊。`

type ChannelIQCriterionStatus string

const (
	ChannelIQCriterionPass    ChannelIQCriterionStatus = "pass"
	ChannelIQCriterionPartial ChannelIQCriterionStatus = "partial"
	ChannelIQCriterionFail    ChannelIQCriterionStatus = "fail"
	ChannelIQCriterionUnknown ChannelIQCriterionStatus = "unknown"
)

type ChannelIQCriterion struct {
	Status ChannelIQCriterionStatus `json:"status"`
}

type ChannelIQCriteria struct {
	Wheels         ChannelIQCriterion `json:"wheels"`
	LegPedal       ChannelIQCriterion `json:"leg_pedal"`
	ViewConsistent ChannelIQCriterion `json:"view_consistent"`
	PelicanBeak    ChannelIQCriterion `json:"pelican_beak"`
}

type ChannelIQEvaluation struct {
	Graded   bool              `json:"graded"`
	Grade    string            `json:"grade"`
	Score    int               `json:"score"`
	Criteria ChannelIQCriteria `json:"criteria"`
}

func channelIQPromptForExecution(prompt string) string {
	value := defaultChannelIQPrompt(prompt)
	if strings.Contains(value, channelIQRubricMarker) {
		return value
	}
	return value + channelIQRubricInstruction
}

func normalizeChannelIQCriterion(value string) ChannelIQCriterionStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pass", "passed", "ok", "true", "通过":
		return ChannelIQCriterionPass
	case "partial", "部分", "勉强":
		return ChannelIQCriterionPartial
	case "fail", "failed", "false", "未通过", "不通过":
		return ChannelIQCriterionFail
	default:
		return ChannelIQCriterionUnknown
	}
}

func ungradedChannelIQEvaluation() ChannelIQEvaluation {
	unknown := ChannelIQCriterion{Status: ChannelIQCriterionUnknown}
	return ChannelIQEvaluation{
		Grade:    "ungraded",
		Criteria: ChannelIQCriteria{Wheels: unknown, LegPedal: unknown, ViewConsistent: unknown, PelicanBeak: unknown},
	}
}

func extractChannelIQRubric(response string) (ChannelIQCriteria, bool) {
	start := strings.Index(response, "<!-- "+channelIQRubricMarker)
	if start < 0 {
		return ChannelIQCriteria{}, false
	}
	open := strings.Index(response[start:], "{")
	if open < 0 {
		return ChannelIQCriteria{}, false
	}
	decoder := json.NewDecoder(strings.NewReader(response[start+open:]))
	var payload struct {
		Wheels         string `json:"wheels"`
		LegPedal       string `json:"leg_pedal"`
		ViewConsistent string `json:"view_consistent"`
		PelicanBeak    string `json:"pelican_beak"`
	}
	if err := decoder.Decode(&payload); err != nil {
		return ChannelIQCriteria{}, false
	}
	criteria := ChannelIQCriteria{
		Wheels:         ChannelIQCriterion{Status: normalizeChannelIQCriterion(payload.Wheels)},
		LegPedal:       ChannelIQCriterion{Status: normalizeChannelIQCriterion(payload.LegPedal)},
		ViewConsistent: ChannelIQCriterion{Status: normalizeChannelIQCriterion(payload.ViewConsistent)},
		PelicanBeak:    ChannelIQCriterion{Status: normalizeChannelIQCriterion(payload.PelicanBeak)},
	}
	return criteria, criteria.Wheels.Status != ChannelIQCriterionUnknown &&
		criteria.LegPedal.Status != ChannelIQCriterionUnknown &&
		criteria.ViewConsistent.Status != ChannelIQCriterionUnknown &&
		criteria.PelicanBeak.Status != ChannelIQCriterionUnknown
}

func evaluateChannelIQResponse(response string) ChannelIQEvaluation {
	criteria, graded := extractChannelIQRubric(response)
	if !graded {
		return ungradedChannelIQEvaluation()
	}
	score := 0
	for _, criterion := range []ChannelIQCriterion{criteria.Wheels, criteria.LegPedal, criteria.ViewConsistent, criteria.PelicanBeak} {
		if criterion.Status == ChannelIQCriterionPass {
			score++
		}
	}

	// The page's matrix defines the named levels by the first structural
	// relationship that breaks: leg/pedal, then view, then wheels. Keep those
	// labels aligned with that matrix instead of reducing them to a raw score.
	grade := "severe"
	switch {
	case score == 4:
		grade = "baseline"
	case criteria.Wheels.Status == ChannelIQCriterionPass &&
		criteria.LegPedal.Status == ChannelIQCriterionFail &&
		criteria.ViewConsistent.Status == ChannelIQCriterionPass &&
		criteria.PelicanBeak.Status == ChannelIQCriterionPass:
		grade = "mild"
	case criteria.Wheels.Status == ChannelIQCriterionPass &&
		criteria.LegPedal.Status != ChannelIQCriterionPass &&
		criteria.ViewConsistent.Status == ChannelIQCriterionFail:
		grade = "moderate"
	}
	return ChannelIQEvaluation{Graded: true, Grade: grade, Score: score, Criteria: criteria}
}
