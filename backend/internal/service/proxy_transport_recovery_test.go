package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type recoveryRepoStub struct {
	candidates []ProxyTransportRecoveryCandidate
	cleared    []int64
	fallbacks  []int64
	restored   []int64
	err        error
	changed    bool
}

func (r *recoveryRepoStub) ListProxyTransportRecoveryCandidates(context.Context, bool) ([]ProxyTransportRecoveryCandidate, error) {
	return r.candidates, r.err
}
func (r *recoveryRepoStub) SetProxyTransportDirectFallbackIfUnchanged(_ context.Context, c ProxyTransportRecoveryCandidate) (bool, error) {
	r.fallbacks = append(r.fallbacks, c.AccountID)
	return r.changed, nil
}
func (r *recoveryRepoStub) RestoreProxyTransportBindingIfUnchanged(_ context.Context, c ProxyTransportRecoveryCandidate) (bool, error) {
	r.restored = append(r.restored, c.AccountID)
	return r.changed, nil
}
func (r *recoveryRepoStub) ClearProxyTransportCooldownIfUnchanged(_ context.Context, c ProxyTransportRecoveryCandidate) (bool, error) {
	r.cleared = append(r.cleared, c.AccountID)
	return r.changed, nil
}

func TestProxyTransportRecoverySharedProxyAndConsecutiveSuccess(t *testing.T) {
	until := time.Now().Add(time.Minute)
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Until: until, Reason: ProxyTransportCooldownPrefix, Proxy: Proxy{ID: 3}}, {AccountID: 2, Until: until, Reason: ProxyTransportCooldownPrefix, Proxy: Proxy{ID: 3}}}, changed: true}
	svc := NewProxyTransportRecoveryService(repo, nil)
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
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Until: time.Now().Add(time.Minute), Reason: ProxyTransportCooldownPrefix, Proxy: Proxy{ID: 3}}}}
	svc := NewProxyTransportRecoveryService(repo, nil)
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
	svc := NewProxyTransportRecoveryService(repo, nil)
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
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Until: time.Now().Add(time.Minute), Reason: ProxyTransportCooldownPrefix, Proxy: Proxy{ID: 3}}}}
	svc := NewProxyTransportRecoveryService(repo, nil)
	defer svc.Stop()
	svc.probe = func(context.Context, *Proxy) error { return nil }
	svc.runOnce(context.Background())
	repo.candidates[0].Until = repo.candidates[0].Until.Add(time.Second)
	svc.runOnce(context.Background())
	require.Empty(t, repo.cleared)
	svc.runOnce(context.Background())
	require.Equal(t, []int64{1}, repo.cleared)
}

func directFallbackConfig() *config.Config {
	return &config.Config{Gateway: config.GatewayConfig{ProxyTransportRecovery: config.GatewayProxyTransportRecoveryConfig{
		AllowDirectFallback: true, FailureThreshold: 3,
	}}}
}

func TestProxyTransportFallbackRequiresRepeatedFailuresAndHealthyDirect(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{
		{AccountID: 1, Proxy: Proxy{ID: 3}}, {AccountID: 2, Proxy: Proxy{ID: 3}},
		{AccountID: 3, Proxy: Proxy{ID: 4}},
	}, changed: true}
	svc := NewProxyTransportRecoveryService(repo, directFallbackConfig())
	defer svc.Stop()
	proxyHealthy, directHealthy := false, false
	proxyCalls, directCalls := 0, 0
	svc.probe = func(_ context.Context, p *Proxy) error {
		if p == nil {
			directCalls++
			if directHealthy {
				return nil
			}
		} else {
			proxyCalls++
			if proxyHealthy {
				return nil
			}
		}
		return errors.New("unavailable")
	}
	svc.runOnce(context.Background())
	svc.runOnce(context.Background())
	require.Empty(t, repo.fallbacks)
	require.Zero(t, directCalls)
	proxyHealthy = true
	svc.runOnce(context.Background())
	proxyHealthy = false
	svc.runOnce(context.Background())
	svc.runOnce(context.Background())
	require.Empty(t, repo.fallbacks, "healthy proxy resets the failure streak")
	svc.runOnce(context.Background())
	require.Empty(t, repo.fallbacks, "an unavailable direct route must not replace the proxy")
	require.Equal(t, 1, directCalls, "share the direct probe across proxies and accounts")
	directHealthy = true
	svc.runOnce(context.Background())
	require.ElementsMatch(t, []int64{1, 2, 3}, repo.fallbacks)
	require.Equal(t, 14, proxyCalls, "one probe per proxy in each of seven scans")
	require.Equal(t, 2, directCalls)
	require.Empty(t, repo.cleared, "ordinary or quota cooldowns must not be cleared")
}

func TestProxyTransportFallbackRestoreSurvivesRestartAndDisabledSetting(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{
		{AccountID: 1, DirectFallback: true, Reason: "quota exhausted", Proxy: Proxy{ID: 3}},
	}, changed: true}
	// A fresh process still recovers saved fallback bindings even after the
	// operator disables new direct fallbacks. No cooldown deadline is needed.
	svc := NewProxyTransportRecoveryService(repo, nil)
	defer svc.Stop()
	fail := false
	svc.probe = func(_ context.Context, p *Proxy) error {
		require.NotNil(t, p, "already-direct accounts do not need direct probes")
		if fail {
			return errors.New("unavailable")
		}
		return nil
	}
	svc.runOnce(context.Background())
	require.Empty(t, repo.restored)
	fail = true
	svc.runOnce(context.Background())
	fail = false
	svc.runOnce(context.Background())
	require.Empty(t, repo.restored)
	svc.runOnce(context.Background())
	require.Equal(t, []int64{1}, repo.restored)
	require.Empty(t, repo.cleared)
	require.Empty(t, repo.fallbacks)
}

func TestProxyTransportFallbackConfigurationScanAndCancellationReset(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{{AccountID: 1, Proxy: Proxy{ID: 3}}}}
	svc := NewProxyTransportRecoveryService(repo, directFallbackConfig())
	defer svc.Stop()
	svc.probe = func(context.Context, *Proxy) error { return errors.New("unavailable") }
	svc.runOnce(context.Background())
	svc.runOnce(context.Background())
	repo.candidates[0].Proxy.UpdatedAt = time.Now()
	svc.runOnce(context.Background())
	require.Empty(t, repo.fallbacks)
	repo.err = errors.New("database unavailable")
	svc.runOnce(context.Background())
	require.Empty(t, svc.failures)
	repo.err = nil
	svc.runOnce(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.runOnce(ctx)
	require.Empty(t, svc.failures, "shutdown cancellation is not a proxy failure")
	require.Empty(t, repo.fallbacks)
	repo.candidates = nil
	svc.runOnce(context.Background())
	require.Empty(t, svc.successes)
	require.Empty(t, svc.failures)
}

func TestProxyTransportFallbackDisabledAndFreshFailures(t *testing.T) {
	repo := &recoveryRepoStub{candidates: []ProxyTransportRecoveryCandidate{
		{AccountID: 1, Until: time.Now().Add(time.Minute), Reason: ProxyTransportCooldownPrefix, Proxy: Proxy{ID: 3}},
	}}
	svc := NewProxyTransportRecoveryService(repo, nil)
	defer svc.Stop()
	directCalls := 0
	svc.probe = func(_ context.Context, p *Proxy) error {
		if p == nil {
			directCalls++
			return nil
		}
		return errors.New("unavailable")
	}
	for i := 0; i < 3; i++ {
		svc.runOnce(context.Background())
	}
	require.Empty(t, repo.fallbacks)
	require.Zero(t, directCalls)
	svc.allowDirectFallback = true
	// New request failures reset the recovery-success streak, but must not
	// hide an already confirmed proxy outage from the failure counter.
	repo.candidates[0].Until = repo.candidates[0].Until.Add(time.Minute)
	svc.runOnce(context.Background())
	require.Equal(t, []int64{1}, repo.fallbacks)
	require.Equal(t, 1, directCalls)
}

func TestProxyTransportFallbackAdminProxyChoiceCancelsRestore(t *testing.T) {
	for _, edit := range []string{"direct", "another_proxy", "unrelated"} {
		t.Run(edit, func(t *testing.T) {
			origin := int64(3)
			repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
				1: {ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
					ProxyFallbackOriginID: &origin, Extra: map[string]any{ProxyTransportFallbackExtraKey: true}},
			}}
			input := &UpdateAccountInput{Name: "edited"}
			proxy := int64(0)
			if edit != "unrelated" {
				if edit == "another_proxy" {
					proxy = 4
				}
				input.ProxyID = &proxy
			}
			updated, err := (&adminServiceImpl{accountRepo: &upstreamBillingProbeAdminRepo{repo}}).UpdateAccount(context.Background(), 1, input)
			require.NoError(t, err)
			if edit == "unrelated" {
				require.Equal(t, &origin, updated.ProxyFallbackOriginID)
				require.Equal(t, true, updated.Extra[ProxyTransportFallbackExtraKey])
			} else {
				require.Nil(t, updated.ProxyFallbackOriginID)
				require.NotEqual(t, true, updated.Extra[ProxyTransportFallbackExtraKey])
				if edit == "direct" {
					require.Nil(t, updated.ProxyID)
				} else {
					require.Equal(t, &proxy, updated.ProxyID)
				}
			}
		})
	}
}
