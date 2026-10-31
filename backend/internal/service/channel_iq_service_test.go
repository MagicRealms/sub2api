package service

import "testing"

func TestParseChannelIQConfigDefaultsAndClamps(t *testing.T) {
	defaultConfig := ParseChannelIQConfig(nil)
	if defaultConfig.Enabled || defaultConfig.IntervalSeconds != 300 {
		t.Fatalf("unexpected default config: %+v", defaultConfig)
	}

	config := ParseChannelIQConfig(map[string]any{
		ChannelIQFeatureKey: map[string]any{
			"enabled":          true,
			"model":            "  gpt-5.6  ",
			"prompt":           "  answer briefly  ",
			"interval_seconds": 10,
		},
	})
	if !config.Enabled || config.Model != "gpt-5.6" || config.Prompt != "answer briefly" {
		t.Fatalf("unexpected parsed config: %+v", config)
	}
	if config.IntervalSeconds != 60 {
		t.Fatalf("interval should clamp to 60 seconds, got %d", config.IntervalSeconds)
	}

	maxConfig := ParseChannelIQConfig(map[string]any{
		ChannelIQFeatureKey: map[string]any{"interval_seconds": float64(999999)},
	})
	if maxConfig.IntervalSeconds != 86400 {
		t.Fatalf("interval should clamp to one day, got %d", maxConfig.IntervalSeconds)
	}
}
