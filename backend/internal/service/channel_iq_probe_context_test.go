package service

import (
	"testing"
	"time"
)

func TestChannelIQProbeContextIsIndependentAndBounded(t *testing.T) {
	first, cancelFirst := newChannelIQProbeContext()
	second, cancelSecond := newChannelIQProbeContext()
	defer cancelSecond()

	deadline, ok := second.Deadline()
	remaining := time.Until(deadline)
	if !ok || remaining < channelIQTestTimeout-time.Second || remaining > channelIQTestTimeout {
		t.Fatalf("unexpected probe deadline: %v, present=%t", deadline, ok)
	}
	cancelFirst()
	if first.Err() == nil || second.Err() != nil {
		t.Fatalf("probe contexts must be independent: first=%v, second=%v", first.Err(), second.Err())
	}
	if !HTTPUpstreamRedirectsDisabled(second) {
		t.Fatal("probe redirects must be disabled to prevent replaying a POST")
	}
}
