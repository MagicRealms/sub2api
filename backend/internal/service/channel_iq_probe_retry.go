package service

import (
	"context"
	"strings"
	"time"
)

const channelIQContinuationLimit = 96 << 10

type channelIQProbeAttempt func(context.Context, string) (*ScheduledTestResult, error)

func channelIQPlainContinuationPrompt(prompt, partial string) string {
	if partial == "" {
		return prompt
	}
	return prompt + "\n\nPrevious assistant output, interrupted before completion:\n" + partial + "\n\ncontinue"
}

func channelIQContinuationText(partial string) string {
	if len(partial) <= channelIQContinuationLimit {
		return partial
	}
	return strings.ToValidUTF8(partial[len(partial)-channelIQContinuationLimit:], "")
}

func channelIQMergeOutput(previous, current string) string {
	if previous == "" {
		return current
	}
	if channelIQCompleteHTML(current) {
		return current
	}
	return previous + current
}

func channelIQCompleteHTML(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "<html") && strings.Contains(lower, "</html>") &&
		strings.Contains(lower, "<svg") && strings.Contains(lower, "</svg>")
}

func channelIQOverloaded(reason string) bool {
	lower := strings.ToLower(reason)
	return strings.Contains(lower, "overload") || strings.Contains(lower, "server_is_overloaded") ||
		strings.Contains(lower, "slow_down")
}

func channelIQRetryableFailure(reason string) bool {
	lower := strings.ToLower(reason)
	return channelIQOverloaded(reason) || strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout")
}

func channelIQRetryDelay(attempt int) time.Duration {
	return min(time.Duration(10<<min(attempt, 3))*time.Second, time.Minute)
}

func waitChannelIQRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// A single result stays running while overloads are retried. Each request gets
// a fresh 30-minute context; the outer context ends at the next scheduled run.
func runChannelIQProbeUntil(ctx context.Context, deadline time.Time, attempt channelIQProbeAttempt, wait func(context.Context, time.Duration) error) *ScheduledTestResult {
	startedAt := time.Now()
	partial := ""
	lastReason := ""
	for count := 0; ; count++ {
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			if lastReason == "" {
				lastReason = "测试未在下次计划运行前完成"
			}
			return channelIQFailedResult(startedAt, partial, lastReason)
		}

		attemptCtx, cancel := context.WithTimeout(ctx, channelIQTestTimeout)
		result, err := attempt(attemptCtx, channelIQContinuationText(partial))
		cancel()
		if result == nil && err == nil {
			return channelIQFailedResult(startedAt, partial, "测试没有返回结果")
		}
		if result != nil {
			partial = channelIQMergeOutput(partial, result.ResponseText)
		}
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			if lastReason == "" {
				lastReason = channelIQResultReason(result, err)
			}
			if lastReason == "" {
				lastReason = "测试未在下次计划运行前完成"
			}
			return channelIQFailedResult(startedAt, partial, lastReason)
		}
		if err == nil && result.Status == "success" && channelIQCompleteHTML(partial) {
			result.ResponseText = partial
			result.StartedAt = startedAt
			result.LatencyMs = result.FinishedAt.Sub(startedAt).Milliseconds()
			return result
		}

		reason := channelIQResultReason(result, err)
		if err == nil && result.Status == "success" {
			reason = "HTML/SVG 内容尚未生成完整"
		} else if !channelIQRetryableFailure(reason) {
			return channelIQFailedResult(startedAt, partial, reason)
		}
		lastReason = reason
		if wait(ctx, channelIQRetryDelay(count)) != nil {
			return channelIQFailedResult(startedAt, partial, lastReason)
		}
	}
}

func channelIQResultReason(result *ScheduledTestResult, err error) string {
	if result != nil && strings.TrimSpace(result.ErrorMessage) != "" {
		return strings.TrimSpace(result.ErrorMessage)
	}
	if err != nil {
		return err.Error()
	}
	return "测试失败"
}

func channelIQFailedResult(startedAt time.Time, partial, reason string) *ScheduledTestResult {
	finishedAt := time.Now()
	return &ScheduledTestResult{
		Status: "failed", ResponseText: partial, ErrorMessage: reason,
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMs: finishedAt.Sub(startedAt).Milliseconds(),
	}
}
