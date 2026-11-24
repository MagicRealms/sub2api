package service

import (
	"context"
	"strings"

	"github.com/tidwall/gjson"
)

const mimoASRModel = "mimo-v2.5-asr"
const mimoTTSModel = "mimo-v2.5-tts"
const mimoVoiceProtocolError = "MiMo voice models require non-streaming Chat Completions"

func isMiMoASRModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), mimoASRModel)
}

func isMiMoTTSModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), mimoTTSModel)
}

func isMiMoVoiceModel(model string) bool {
	return isMiMoASRModel(model) || isMiMoTTSModel(model)
}

func mimoASRHourlyPrice(ctx context.Context, resolver *ModelPricingResolver, key *APIKey) (float64, bool) {
	if resolver == nil || key == nil || key.Group == nil {
		return 0, false
	}
	resolved := resolver.Resolve(ctx, PricingInput{Model: mimoASRModel, Group: key.Group, GroupID: &key.Group.ID})
	if resolved.Mode != BillingModePerRequest || (resolved.Source != PricingSourceGroup && resolved.Source != PricingSourceChannel) {
		return 0, false
	}
	// Voice pricing uses named tiers so a default flat request fee is never an hourly rate.
	for _, tier := range resolved.RequestTiers {
		if tier.TierLabel == "stt" && tier.PerRequestPrice != nil && validCost(*tier.PerRequestPrice) {
			return *tier.PerRequestPrice, true
		}
	}
	return 0, false
}

func mimoASRAudioUsage(body []byte) *AudioUsage {
	seconds := gjson.GetBytes(body, "usage.seconds")
	if !gjson.ValidBytes(body) || gjson.GetBytes(body, "error").Type != gjson.Null ||
		seconds.Type != gjson.Number || !validCost(seconds.Num) ||
		gjson.GetBytes(body, "choices.0.message.content").Type != gjson.String {
		return nil
	}
	return &AudioUsage{Mode: "stt", DurationOrUnits: seconds.Num / 3600}
}
