package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
)

// Only this gateway-owned transport cooldown is eligible; quota, credentials,
// manual disabling and other cooldowns remain untouched.
const ProxyTransportCooldownPrefix = "upstream transport error (proxy/network): "

type ProxyTransportRecoveryCandidate struct {
	AccountID        int64
	AccountUpdatedAt time.Time
	Until            time.Time
	Reason           string
	Proxy            Proxy
}

type ProxyTransportRecoveryRepository interface {
	ListProxyTransportRecoveryCandidates(context.Context) ([]ProxyTransportRecoveryCandidate, error)
	ClearProxyTransportCooldownIfUnchanged(context.Context, ProxyTransportRecoveryCandidate) (bool, error)
}

type proxyRecoveryKey struct {
	id           int64
	updated      time.Time
	failureUntil time.Time
}

type ProxyTransportRecoveryService struct {
	repo      ProxyTransportRecoveryRepository
	probe     func(context.Context, *Proxy) error
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	successes map[proxyRecoveryKey]int
}

func NewProxyTransportRecoveryService(repo ProxyTransportRecoveryRepository) *ProxyTransportRecoveryService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProxyTransportRecoveryService{repo: repo, probe: probeOpenAIProxyRecovery, ctx: ctx, cancel: cancel, successes: make(map[proxyRecoveryKey]int)}
}

func (s *ProxyTransportRecoveryService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	slog.Info("proxy.transport_recovery_started", "interval_seconds", 15, "required_successes", 2)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.runOnce(s.ctx)
			}
		}
	}()
}

func (s *ProxyTransportRecoveryService) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

func (s *ProxyTransportRecoveryService) runOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	candidates, err := s.repo.ListProxyTransportRecoveryCandidates(ctx)
	if err != nil {
		clear(s.successes)
		if parent.Err() == nil {
			slog.Warn("proxy.transport_recovery_scan_failed")
		}
		return
	}
	groups := make(map[proxyRecoveryKey][]ProxyTransportRecoveryCandidate)
	latestFailure := make(map[int64]time.Time)
	for _, c := range candidates {
		if c.Until.After(latestFailure[c.Proxy.ID]) {
			latestFailure[c.Proxy.ID] = c.Until
		}
	}
	for _, c := range candidates {
		key := proxyRecoveryKey{c.Proxy.ID, c.Proxy.UpdatedAt, latestFailure[c.Proxy.ID]}
		groups[key] = append(groups[key], c)
	}
	// No state is retained after a cooldown disappears or proxy configuration changes.
	for key := range s.successes {
		if _, ok := groups[key]; !ok {
			delete(s.successes, key)
		}
	}
	for key, group := range groups {
		probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
		err := s.probe(probeCtx, &group[0].Proxy)
		probeCancel()
		if err != nil {
			delete(s.successes, key)
			continue
		}
		s.successes[key]++
		if s.successes[key] < 2 {
			continue
		}
		for _, c := range group {
			changed, err := s.repo.ClearProxyTransportCooldownIfUnchanged(ctx, c)
			if err != nil {
				slog.Warn("proxy.transport_recovery_clear_failed", "account_id", c.AccountID)
				continue
			}
			if changed {
				slog.Info("proxy.transport_recovery_restored", "account_id", c.AccountID, "proxy_id", c.Proxy.ID)
			}
		}
	}
}

// Probe the actual Codex HTTPS destination, not merely the proxy TCP port or
// an unrelated IP lookup site. No account token or generation request is sent.
// 401 is expected without credentials; redirects, 403, 429 and 5xx fail closed.
func probeOpenAIProxyRecovery(ctx context.Context, p *Proxy) error {
	_, proxyURL, err := proxyurl.Parse(p.URL())
	if err != nil {
		return err
	}
	transport := &http.Transport{TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 4 * time.Second}
	if err := proxyutil.ConfigureTransportProxy(transport, proxyURL); err != nil {
		return err
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://chatgpt.com/backend-api/codex/models", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("probe status %d", resp.StatusCode)
	}
	return nil
}
