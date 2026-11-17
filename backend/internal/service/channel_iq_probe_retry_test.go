package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelIQOverloadContinuesPartialHTMLInSameRun(t *testing.T) {
	partial := "<!doctype html><html><body><svg><circle r=\"4\"/>"
	var continuations []string
	result := runChannelIQProbeUntil(context.Background(), time.Now().Add(time.Hour), func(_ context.Context, continuation string) (*ScheduledTestResult, error) {
		continuations = append(continuations, continuation)
		if len(continuations) == 1 {
			return &ScheduledTestResult{Status: "failed", ResponseText: partial, ErrorMessage: "Our servers are currently overloaded."}, nil
		}
		return &ScheduledTestResult{Status: "success", ResponseText: "</svg></body></html>", FinishedAt: time.Now()}, nil
	}, func(context.Context, time.Duration) error { return nil })

	require.Equal(t, []string{"", partial}, continuations)
	require.Equal(t, "success", result.Status)
	require.Equal(t, partial+"</svg></body></html>", result.ResponseText)
	require.Empty(t, result.ErrorMessage)
}

func TestChannelIQOverloadWithoutPartialRestartsOriginalPrompt(t *testing.T) {
	var continuations []string
	result := runChannelIQProbeUntil(context.Background(), time.Now().Add(time.Hour), func(_ context.Context, continuation string) (*ScheduledTestResult, error) {
		continuations = append(continuations, continuation)
		if len(continuations) == 1 {
			return &ScheduledTestResult{Status: "failed", ErrorMessage: "server_is_overloaded"}, nil
		}
		return &ScheduledTestResult{Status: "success", ResponseText: "<html><svg></svg></html>", FinishedAt: time.Now()}, nil
	}, func(context.Context, time.Duration) error { return nil })

	require.Equal(t, []string{"", ""}, continuations)
	require.Equal(t, "success", result.Status)
}

func TestChannelIQOverloadWaitsUntilNextScheduledRunThenRecordsReason(t *testing.T) {
	deadline := time.Now().Add(20 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	attempts := 0
	result := runChannelIQProbeUntil(ctx, deadline, func(context.Context, string) (*ScheduledTestResult, error) {
		attempts++
		return &ScheduledTestResult{Status: "failed", ResponseText: "<html>", ErrorMessage: "upstream overloaded"}, nil
	}, waitChannelIQRetry)

	require.Equal(t, 1, attempts)
	require.Equal(t, "failed", result.Status)
	require.Equal(t, "upstream overloaded", result.ErrorMessage)
	require.Equal(t, "<html>", result.ResponseText)
}

func TestChannelIQOtherErrorsAreTerminal(t *testing.T) {
	result := runChannelIQProbeUntil(context.Background(), time.Now().Add(time.Hour), func(context.Context, string) (*ScheduledTestResult, error) {
		return nil, errors.New("Authentication failed (401)")
	}, func(context.Context, time.Duration) error { t.Fatal("unexpected retry"); return nil })

	require.Equal(t, "failed", result.Status)
	require.Equal(t, "Authentication failed (401)", result.ErrorMessage)
}

func TestChannelIQRequestTimeoutContinuesBeforeNextScheduledRun(t *testing.T) {
	attempts := 0
	result := runChannelIQProbeUntil(context.Background(), time.Now().Add(time.Hour), func(context.Context, string) (*ScheduledTestResult, error) {
		attempts++
		if attempts == 1 {
			return &ScheduledTestResult{Status: "failed", ResponseText: "<html><svg>", ErrorMessage: "Stream read error: context deadline exceeded"}, nil
		}
		return &ScheduledTestResult{Status: "success", ResponseText: "</svg></html>", FinishedAt: time.Now()}, nil
	}, func(context.Context, time.Duration) error { return nil })

	require.Equal(t, 2, attempts)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "<html><svg></svg></html>", result.ResponseText)
}

func TestChannelIQIncompleteSuccessContinuesUntilDocumentCloses(t *testing.T) {
	attempts := 0
	result := runChannelIQProbeUntil(context.Background(), time.Now().Add(time.Hour), func(_ context.Context, continuation string) (*ScheduledTestResult, error) {
		attempts++
		if attempts == 1 {
			return &ScheduledTestResult{Status: "success", ResponseText: "<html><svg>", FinishedAt: time.Now()}, nil
		}
		require.Equal(t, "<html><svg>", continuation)
		return &ScheduledTestResult{Status: "success", ResponseText: "</svg></html>", FinishedAt: time.Now()}, nil
	}, func(context.Context, time.Duration) error { return nil })

	require.Equal(t, 2, attempts)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "<html><svg></svg></html>", result.ResponseText)
}

func TestChannelIQContinuationPayloadUsesSeparateContinueTurn(t *testing.T) {
	payload := createOpenAITestPayloadWithReasoning("gpt-6.1-sol", true, "Create an animated pelican.", "xhigh", "<html><svg>")
	input := payload["input"].([]map[string]any)
	require.Len(t, input, 3)
	require.Equal(t, "user", input[0]["role"])
	require.Equal(t, "assistant", input[1]["role"])
	require.Equal(t, "<html><svg>", input[1]["content"].([]map[string]any)[0]["text"])
	require.Equal(t, "output_text", input[1]["content"].([]map[string]any)[0]["type"])
	require.Equal(t, "continue", input[2]["content"].([]map[string]any)[0]["text"])
	require.Equal(t, false, payload["store"])
	require.Equal(t, "xhigh", payload["reasoning"].(map[string]any)["effort"])
	require.True(t, strings.HasSuffix(channelIQPlainContinuationPrompt("prompt", "partial"), "continue"))
}
