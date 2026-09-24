package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type recoveryRepoStub struct {
	candidates []ProxyTransportRecoveryCandidate
	cleared    []int64
	err        error
	changed    bool
}

func (r *recoveryRepoStub) ListProxyTransportRecoveryCandidates(context.Context) ([]ProxyTransportRecoveryCandidate, error) {
	return r.candidates, r.err
}
func (r *recoveryRepoStub) ClearProxyTransportCooldownIfUnchanged(_ context.Context, c ProxyTransportRecoveryCandidate) (bool, error) {
	r.cleared = append(r.cleared, c.AccountID)
	return r.changed, nil
}

func TestProxyTransportRecoverySharedProxyAndConsecutiveSuccess(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Proxy: Proxy{ID: 3}}, {AccountID: 2, Proxy: Proxy{ID: 3}}}, changed: true}
	svc := NewProxyTransportRecoveryService(repo)
	defer svc.Stop()
	calls := 0
	fail := false
	svc.probe = func(context.Context, *Proxy) error {
		calls++
		if fail {
			return errors.New("proxy unavailable")
		}
		return nil
	}
	svc.runOnce(context.Background())
	require.Equal(t, 1, calls, "shared proxy must be tested only once per pass")
	require.Empty(t, repo.cleared)
	fail = true
	svc.runOnce(context.Background())
	require.Empty(t, repo.cleared)
	fail = false
	svc.runOnce(context.Background())
	require.Empty(t, repo.cleared, "a failed probe resets consecutive successes")
	svc.runOnce(context.Background())
	require.Equal(t, []int64{1, 2}, repo.cleared)
}

func TestProxyTransportRecoveryChangedProxyAndScanFailureReset(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Proxy: Proxy{ID: 3}}}}
	svc := NewProxyTransportRecoveryService(repo)
	defer svc.Stop()
	svc.probe = func(context.Context, *Proxy) error { return nil }
	svc.runOnce(context.Background())
	repo.candidates[0].Proxy.UpdatedAt = time.Now()
	svc.runOnce(context.Background())
	require.Empty(t, repo.cleared, "old proxy success must not apply to replacement configuration")
	repo.err = errors.New("database unavailable")
	svc.runOnce(context.Background())
	repo.err = nil
	svc.runOnce(context.Background())
	require.Empty(t, repo.cleared)
	svc.runOnce(context.Background())
	require.Equal(t, []int64{1}, repo.cleared)
	repo.candidates = nil
	svc.runOnce(context.Background())
	require.Empty(t, svc.successes)
}

func TestProxyTransportRecoveryStopCancelsProbe(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Proxy: Proxy{ID: 3}}}}
	svc := NewProxyTransportRecoveryService(repo)
	entered := make(chan struct{})
	done := make(chan struct{})
	svc.probe = func(ctx context.Context, _ *Proxy) error { close(entered); <-ctx.Done(); return ctx.Err() }
	go func() { svc.runOnce(svc.ctx); close(done) }()
	<-entered
	svc.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe was not canceled")
	}
	require.Empty(t, repo.cleared)
}

func TestProxyTransportRecoveryNewFailureResetsHealthStreak(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Until: time.Now().Add(time.Minute), Proxy: Proxy{ID: 3}}}}
	svc := NewProxyTransportRecoveryService(repo)
	defer svc.Stop()
	svc.probe = func(context.Context, *Proxy) error { return nil }
	svc.runOnce(context.Background())
	repo.candidates[0].Until = repo.candidates[0].Until.Add(time.Second)
	svc.runOnce(context.Background())
	require.Empty(t, repo.cleared)
	svc.runOnce(context.Background())
	require.Equal(t, []int64{1}, repo.cleared)
}
