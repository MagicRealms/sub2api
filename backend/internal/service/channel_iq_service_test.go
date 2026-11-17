package service

import (
	"testing"
	"time"
)

func TestChannelIQSchedulesSupportedIntervals(t *testing.T) {
	for interval, want := range map[int]string{
		channelIQHourlyInterval:       "0 */1 * * *",
		channelIQNinetyMinuteInterval: "@every 90m",
		channelIQTwoHourInterval:      "0 */2 * * *",
	} {
		got, err := channelIQCronForInterval(interval)
		if err != nil || got != want {
			t.Fatalf("interval %d schedule = %q, %v; want %q", interval, got, err, want)
		}
		if gotInterval := channelIQIntervalFromCron(got); gotInterval != interval {
			t.Fatalf("schedule %q maps to %d seconds; want %d", got, gotInterval, interval)
		}
	}
	for _, interval := range []int{60, 300, 1800, 86400} {
		if _, err := channelIQCronForInterval(interval); err == nil {
			t.Fatalf("interval %d must not schedule extra tests", interval)
		}
	}
}

func TestChannelIQNinetyMinuteScheduleKeepsItsCadence(t *testing.T) {
	base := time.Date(2026, 11, 17, 12, 10, 0, 0, time.UTC)
	plan := &ScheduledTestPlan{CronExpression: "@every 90m", NextRunAt: &base}

	next, err := nextChannelIQRun(plan, base.Add(20*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if want := base.Add(90 * time.Minute); !next.Equal(want) {
		t.Fatalf("next run = %s; want %s", next, want)
	}

	next, err = nextChannelIQRun(plan, base.Add(95*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if want := base.Add(180 * time.Minute); !next.Equal(want) {
		t.Fatalf("missed schedule should advance to the next cadence slot: got %s, want %s", next, want)
	}
}

func TestChannelIQTwoHourScheduleRunsOnEvenHour(t *testing.T) {
	from := time.Date(2026, 11, 17, 13, 42, 0, 0, time.UTC)
	next, err := computeNextRun("0 */2 * * *", from)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 17, 14, 0, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("next run = %s; want %s", next, want)
	}
}

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
