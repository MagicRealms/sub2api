package service

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	AutoUpstreamModelSyncEnabledExtraKey       = "auto_sync_upstream_models_enabled"
	AutoUpstreamModelSyncIntervalExtraKey      = "auto_sync_upstream_models_interval_hours"
	AutoUpstreamModelSyncLastAttemptExtraKey   = "auto_sync_upstream_models_last_attempt_ms"
	AutoUpstreamModelSyncOneMinuteHours        = 1.0 / 60
	autoUpstreamModelSyncPollInterval          = 5 * time.Second
	autoUpstreamModelSyncStoreTimeout          = 5 * time.Second
	autoUpstreamModelSyncRequestTimeout        = 2 * time.Minute
	autoUpstreamModelSyncMaxConcurrentAccounts = 3
)

type AutoUpstreamModelSyncCandidate struct {
	AccountID     int64
	IntervalHours float64
}

// AutoUpstreamModelSyncStore is implemented by the account repository. Claiming
// is atomic so multiple app instances cannot sync the same account at once.
type AutoUpstreamModelSyncStore interface {
	ListDueAutoUpstreamModelSync(ctx context.Context, now time.Time) ([]AutoUpstreamModelSyncCandidate, error)
	ClaimAutoUpstreamModelSync(ctx context.Context, accountID int64, intervalHours float64, now time.Time) (bool, error)
	GetByID(ctx context.Context, id int64) (*Account, error)
	UpdateAccountModelMapping(ctx context.Context, id int64, mapping map[string]any) error
}

type UpstreamModelCatalogSyncer interface {
	SyncUpstreamModelCatalog(ctx context.Context, account *Account) (*UpstreamModelCatalog, error)
}

type AutoUpstreamModelSyncService struct {
	store       AutoUpstreamModelSyncStore
	catalogSync UpstreamModelCatalogSyncer
	rootCtx     context.Context
	cancel      context.CancelFunc
	startOnce   sync.Once
	stopOnce    sync.Once
	stop        chan struct{}
	wg          sync.WaitGroup
	slots       chan struct{}
	inFlight    sync.Map
	scanMu      sync.Mutex
}

func NewAutoUpstreamModelSyncService(store AutoUpstreamModelSyncStore, catalogSync UpstreamModelCatalogSyncer) *AutoUpstreamModelSyncService {
	rootCtx, cancel := context.WithCancel(context.Background())
	return &AutoUpstreamModelSyncService{
		store:       store,
		catalogSync: catalogSync,
		rootCtx:     rootCtx,
		cancel:      cancel,
		stop:        make(chan struct{}),
		slots:       make(chan struct{}, autoUpstreamModelSyncMaxConcurrentAccounts),
	}
}

func AutoUpstreamModelSyncInterval(hours float64) (time.Duration, bool) {
	switch hours {
	case AutoUpstreamModelSyncOneMinuteHours:
		return time.Minute, true
	case 0.5:
		return 30 * time.Minute, true
	case 1:
		return time.Hour, true
	case 1.5:
		return 90 * time.Minute, true
	case 2:
		return 2 * time.Hour, true
	case 3:
		return 3 * time.Hour, true
	case 4:
		return 4 * time.Hour, true
	case 5:
		return 5 * time.Hour, true
	default:
		return 0, false
	}
}

func (s *AutoUpstreamModelSyncService) Start() {
	if s == nil || s.store == nil || s.catalogSync == nil {
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			select {
			case <-s.stop:
				return
			default:
			}
			ticker := time.NewTicker(autoUpstreamModelSyncPollInterval)
			defer ticker.Stop()
			launch := func(now time.Time) {
				s.wg.Add(1)
				go func() {
					defer s.wg.Done()
					s.runOnce(s.rootCtx, now)
				}()
			}
			launch(time.Now())
			for {
				select {
				case now := <-ticker.C:
					launch(now)
				case <-s.stop:
					return
				}
			}
		}()
	})
}

func (s *AutoUpstreamModelSyncService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stop) })
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *AutoUpstreamModelSyncService) runOnce(ctx context.Context, now time.Time) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// A slow database must not accumulate a new scan goroutine on every tick.
	if !s.scanMu.TryLock() {
		return
	}
	listCtx, listCancel := context.WithTimeout(ctx, autoUpstreamModelSyncStoreTimeout)
	candidates, err := s.store.ListDueAutoUpstreamModelSync(listCtx, now)
	listCancel()
	s.scanMu.Unlock()
	if err != nil {
		slog.Warn("auto_upstream_model_sync_list_failed", "error", err)
		return
	}
	if len(candidates) == 0 {
		return
	}

	var wg sync.WaitGroup
dispatch:
	for _, candidate := range candidates {
		interval, valid := AutoUpstreamModelSyncInterval(candidate.IntervalHours)
		if !valid || candidate.AccountID <= 0 {
			continue
		}
		if _, loaded := s.inFlight.LoadOrStore(candidate.AccountID, struct{}{}); loaded {
			continue
		}
		select {
		case s.slots <- struct{}{}:
		case <-ctx.Done():
			s.inFlight.Delete(candidate.AccountID)
			wg.Wait()
			return
		case <-s.stop:
			s.inFlight.Delete(candidate.AccountID)
			wg.Wait()
			return
		default:
			// Poll again when capacity is free instead of building a queue of
			// goroutines behind a slow upstream. Slots are shared across polls.
			s.inFlight.Delete(candidate.AccountID)
			break dispatch
		}
		claimCtx, claimCancel := context.WithTimeout(ctx, autoUpstreamModelSyncStoreTimeout)
		claimed, claimErr := s.store.ClaimAutoUpstreamModelSync(claimCtx, candidate.AccountID, candidate.IntervalHours, time.Now())
		claimCancel()
		if claimErr != nil {
			<-s.slots
			s.inFlight.Delete(candidate.AccountID)
			slog.Warn("auto_upstream_model_sync_claim_failed", "account_id", candidate.AccountID, "error", claimErr)
			continue
		}
		if !claimed {
			<-s.slots
			s.inFlight.Delete(candidate.AccountID)
			continue
		}
		wg.Add(1)
		go func(accountID int64, interval time.Duration) {
			defer wg.Done()
			defer func() {
				s.inFlight.Delete(accountID)
				<-s.slots
			}()
			s.syncOne(ctx, accountID, interval)
		}(candidate.AccountID, interval)
	}
	wg.Wait()
}

func (s *AutoUpstreamModelSyncService) syncOne(parent context.Context, accountID int64, interval time.Duration) {
	started := time.Now()
	timeout := autoUpstreamModelSyncRequestTimeout
	if interval/2 < timeout {
		// Finish a short-interval attempt before its next claim can become due,
		// including when another application instance is running the worker.
		timeout = interval / 2
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	account, err := s.store.GetByID(ctx, accountID)
	if err != nil {
		slog.Warn("auto_upstream_model_sync_account_read_failed", "account_id", accountID, "error", err)
		return
	}
	catalog, err := s.catalogSync.SyncUpstreamModelCatalog(ctx, account)
	if err != nil {
		kind := "unknown"
		var syncErr *UpstreamModelSyncError
		if errors.As(err, &syncErr) {
			kind = string(syncErr.Kind)
		}
		slog.Warn("auto_upstream_model_sync_upstream_failed", "account_id", accountID, "kind", kind)
		return
	}
	if catalog == nil || !catalog.LiveListAvailable {
		slog.Warn("auto_upstream_model_sync_live_catalog_unavailable", "account_id", accountID)
		return
	}

	mapping := make(map[string]any, len(catalog.Models))
	for _, modelID := range catalog.Models {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" || strings.Contains(modelID, "*") {
			continue
		}
		mapping[modelID] = modelID
	}
	if len(mapping) == 0 {
		slog.Warn("auto_upstream_model_sync_empty_catalog", "account_id", accountID)
		return
	}
	if account != nil && reflect.DeepEqual(account.Credentials["model_mapping"], mapping) {
		slog.Debug("auto_upstream_model_sync_unchanged", "account_id", accountID, "model_count", len(mapping), "duration_ms", time.Since(started).Milliseconds())
		return
	}
	if err := s.store.UpdateAccountModelMapping(ctx, accountID, mapping); err != nil {
		slog.Warn("auto_upstream_model_sync_save_failed", "account_id", accountID, "error", err)
		return
	}
	slog.Info("auto_upstream_model_sync_succeeded", "account_id", accountID, "model_count", len(mapping), "interval_hours", interval.Hours(), "duration_ms", time.Since(started).Milliseconds())
}

func parseAutoUpstreamModelSyncInterval(raw string) (float64, bool) {
	hours, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	_, valid := AutoUpstreamModelSyncInterval(hours)
	return hours, valid
}

func ValidateAutoUpstreamModelSyncExtra(extra map[string]any) error {
	enabledValue, enabledSet := extra[AutoUpstreamModelSyncEnabledExtraKey]
	enabled, enabledOK := enabledValue.(bool)
	if enabledSet && !enabledOK {
		return errors.New("auto upstream model sync enabled must be a boolean")
	}

	intervalValue, intervalSet := extra[AutoUpstreamModelSyncIntervalExtraKey]
	intervalText, intervalOK := intervalValue.(string)
	if !intervalOK {
		if numeric, ok := intervalValue.(float64); ok {
			intervalText = strconv.FormatFloat(numeric, 'f', -1, 64)
			intervalOK = true
		}
	}
	if intervalSet {
		if !intervalOK {
			return errors.New("auto upstream model sync interval must be a string")
		}
		hours, valid := parseAutoUpstreamModelSyncInterval(strings.TrimSpace(intervalText))
		if !valid {
			return errors.New("auto upstream model sync interval must be 1 minute or one of 0.5, 1, 1.5, 2, 3, 4, or 5 hours")
		}
		extra[AutoUpstreamModelSyncIntervalExtraKey] = strconv.FormatFloat(hours, 'f', -1, 64)
	}
	if enabledSet && enabled && !intervalSet {
		return errors.New("auto upstream model sync interval is required when enabled")
	}
	return nil
}
