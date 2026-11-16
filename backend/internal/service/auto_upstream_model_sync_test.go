package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type autoUpstreamModelSyncStoreStub struct {
	mu         sync.Mutex
	candidates []AutoUpstreamModelSyncCandidate
	accounts   map[int64]*Account
	claimed    bool
	mappingID  int64
	mapping    map[string]any
	updated    int
	claims     []int64
}

func (s *autoUpstreamModelSyncStoreStub) ListDueAutoUpstreamModelSync(context.Context, time.Time) ([]AutoUpstreamModelSyncCandidate, error) {
	return s.candidates, nil
}

func (s *autoUpstreamModelSyncStoreStub) ClaimAutoUpstreamModelSync(_ context.Context, id int64, _ float64, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims = append(s.claims, id)
	return s.claimed, nil
}

func (s *autoUpstreamModelSyncStoreStub) GetByID(_ context.Context, id int64) (*Account, error) {
	return s.accounts[id], nil
}

func (s *autoUpstreamModelSyncStoreStub) UpdateAccountModelMapping(_ context.Context, id int64, mapping map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mappingID = id
	s.mapping = mapping
	s.updated++
	return nil
}

type autoUpstreamModelCatalogSyncerStub struct {
	catalog *UpstreamModelCatalog
	err     error
	account *Account
}

func (s *autoUpstreamModelCatalogSyncerStub) SyncUpstreamModelCatalog(_ context.Context, account *Account) (*UpstreamModelCatalog, error) {
	s.account = account
	return s.catalog, s.err
}

func TestAutoUpstreamModelSyncInterval(t *testing.T) {
	for _, hours := range []float64{0.5, 1, 1.5, 2, 3, 4, 5} {
		interval, ok := AutoUpstreamModelSyncInterval(hours)
		require.True(t, ok)
		require.Equal(t, time.Duration(hours*float64(time.Hour)), interval)
	}
	_, ok := AutoUpstreamModelSyncInterval(0.75)
	require.False(t, ok)
	interval, ok := AutoUpstreamModelSyncInterval(AutoUpstreamModelSyncOneMinuteHours)
	require.True(t, ok)
	require.Equal(t, time.Minute, interval)
}

func TestValidateAutoUpstreamModelSyncExtra(t *testing.T) {
	extra := map[string]any{
		AutoUpstreamModelSyncEnabledExtraKey:  true,
		AutoUpstreamModelSyncIntervalExtraKey: 1.5,
	}
	require.NoError(t, ValidateAutoUpstreamModelSyncExtra(extra))
	require.Equal(t, "1.5", extra[AutoUpstreamModelSyncIntervalExtraKey])
	require.Error(t, ValidateAutoUpstreamModelSyncExtra(map[string]any{
		AutoUpstreamModelSyncEnabledExtraKey:  true,
		AutoUpstreamModelSyncIntervalExtraKey: "0.75",
	}))
	require.Error(t, ValidateAutoUpstreamModelSyncExtra(map[string]any{
		AutoUpstreamModelSyncEnabledExtraKey: "yes",
	}))
	require.Error(t, ValidateAutoUpstreamModelSyncExtra(map[string]any{
		AutoUpstreamModelSyncEnabledExtraKey: true,
	}))
	minute := map[string]any{
		AutoUpstreamModelSyncEnabledExtraKey:  true,
		AutoUpstreamModelSyncIntervalExtraKey: AutoUpstreamModelSyncOneMinuteHours,
	}
	require.NoError(t, ValidateAutoUpstreamModelSyncExtra(minute))
	require.Equal(t, "0.016666666666666666", minute[AutoUpstreamModelSyncIntervalExtraKey])
}

type autoUpstreamModelCatalogSyncFunc func(context.Context, *Account) (*UpstreamModelCatalog, error)

func (f autoUpstreamModelCatalogSyncFunc) SyncUpstreamModelCatalog(ctx context.Context, account *Account) (*UpstreamModelCatalog, error) {
	return f(ctx, account)
}

func TestAutoUpstreamModelSyncUnchangedMappingDoesNotWrite(t *testing.T) {
	store := &autoUpstreamModelSyncStoreStub{
		accounts: map[int64]*Account{41: {
			ID:          41,
			Credentials: map[string]any{"model_mapping": map[string]any{"model-a": "model-a", "model-b": "model-b"}},
		}},
	}
	syncer := &autoUpstreamModelCatalogSyncerStub{catalog: &UpstreamModelCatalog{
		Models: []string{"model-b", "model-a", "model-a"}, LiveListAvailable: true,
	}}
	svc := NewAutoUpstreamModelSyncService(store, syncer)
	t.Cleanup(svc.Stop)
	svc.syncOne(context.Background(), 41, time.Minute)
	require.Zero(t, store.updated, "an unchanged catalog must not publish scheduler changes")

	store.accounts[41].Credentials["model_mapping"] = map[string]any{"alias": "model-a"}
	svc.syncOne(context.Background(), 41, time.Minute)
	require.Equal(t, 1, store.updated, "an actual mapping change must still be persisted")
}

func TestAutoUpstreamModelSyncPollsShareConcurrencyAndSkipInFlightAccounts(t *testing.T) {
	store := &autoUpstreamModelSyncStoreStub{accounts: make(map[int64]*Account), claimed: true}
	for id := int64(1); id <= 5; id++ {
		store.accounts[id] = &Account{ID: id}
		store.candidates = append(store.candidates, AutoUpstreamModelSyncCandidate{AccountID: id, IntervalHours: AutoUpstreamModelSyncOneMinuteHours})
	}
	started := make(chan int64, 5)
	release := make(chan struct{})
	var releaseOnce sync.Once
	svc := NewAutoUpstreamModelSyncService(store, autoUpstreamModelCatalogSyncFunc(func(ctx context.Context, account *Account) (*UpstreamModelCatalog, error) {
		started <- account.ID
		select {
		case <-release:
			return &UpstreamModelCatalog{Models: []string{"model-a"}, LiveListAvailable: true}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); svc.Stop() })
	firstDone := make(chan struct{})
	go func() { defer close(firstDone); svc.runOnce(context.Background(), time.Now()) }()
	for i := 0; i < autoUpstreamModelSyncMaxConcurrentAccounts; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("expected the first three accounts to start")
		}
	}
	secondDone := make(chan struct{})
	go func() { defer close(secondDone); svc.runOnce(context.Background(), time.Now()) }()
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("a poll must not queue behind occupied worker slots")
	}
	store.mu.Lock()
	claims := append([]int64(nil), store.claims...)
	store.mu.Unlock()
	require.Equal(t, []int64{1, 2, 3}, claims, "busy and in-flight accounts must not be claimed again")
	releaseOnce.Do(func() { close(release) })
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("workers should finish when the upstream returns")
	}
	require.Equal(t, 3, store.updated)
	require.Empty(t, svc.slots)
}

func TestAutoUpstreamModelSyncMinuteAttemptExpiresBeforeNextClaim(t *testing.T) {
	store := &autoUpstreamModelSyncStoreStub{accounts: map[int64]*Account{41: {ID: 41}}}
	svc := NewAutoUpstreamModelSyncService(store, autoUpstreamModelCatalogSyncFunc(func(ctx context.Context, _ *Account) (*UpstreamModelCatalog, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 30*time.Second)
		require.Greater(t, time.Until(deadline), 25*time.Second)
		return nil, errors.New("test upstream unavailable")
	}))
	t.Cleanup(svc.Stop)
	svc.syncOne(context.Background(), 41, time.Minute)
	require.Zero(t, store.updated)
}

func TestAutoUpstreamModelSyncStopCancelsActiveAttempt(t *testing.T) {
	store := &autoUpstreamModelSyncStoreStub{
		accounts: map[int64]*Account{41: {ID: 41}}, claimed: true,
		candidates: []AutoUpstreamModelSyncCandidate{{AccountID: 41, IntervalHours: AutoUpstreamModelSyncOneMinuteHours}},
	}
	started := make(chan struct{})
	svc := NewAutoUpstreamModelSyncService(store, autoUpstreamModelCatalogSyncFunc(func(ctx context.Context, _ *Account) (*UpstreamModelCatalog, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	t.Cleanup(svc.Stop)
	svc.Start()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the startup attempt to run")
	}
	stopped := make(chan struct{})
	go func() { defer close(stopped); svc.Stop() }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stopping the scheduler must cancel its active attempts")
	}
	require.Zero(t, store.updated)
}

func TestAutoUpstreamModelSyncReplacesOnlyMappingFromLiveCatalog(t *testing.T) {
	store := &autoUpstreamModelSyncStoreStub{
		candidates: []AutoUpstreamModelSyncCandidate{{AccountID: 41, IntervalHours: 1}},
		accounts: map[int64]*Account{41: {
			ID:          41,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"base_url": "https://relay.example/v1", "api_key": "sk-test"},
		}},
		claimed: true,
	}
	syncer := &autoUpstreamModelCatalogSyncerStub{catalog: &UpstreamModelCatalog{
		Models:            []string{" model-a ", "model-b", "model-a", "wild*"},
		LiveListAvailable: true,
	}}
	svc := NewAutoUpstreamModelSyncService(store, syncer)
	svc.runOnce(context.Background(), time.Now())

	require.Equal(t, 1, store.updated)
	require.Equal(t, int64(41), store.mappingID)
	require.Equal(t, map[string]any{"model-a": "model-a", "model-b": "model-b"}, store.mapping)
	require.Equal(t, "https://relay.example/v1", syncer.account.Credentials["base_url"])
}

func TestAutoUpstreamModelSyncKeepsMappingUnlessLiveCatalogIsUsable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		catalog *UpstreamModelCatalog
		err     error
	}{
		{name: "empty", catalog: &UpstreamModelCatalog{LiveListAvailable: true}},
		{name: "fallback", catalog: &UpstreamModelCatalog{Models: []string{"model-a"}, LiveListAvailable: false}},
		{name: "upstream error", err: errors.New("upstream unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &autoUpstreamModelSyncStoreStub{
				candidates: []AutoUpstreamModelSyncCandidate{{AccountID: 42, IntervalHours: 0.5}},
				accounts:   map[int64]*Account{42: {ID: 42}},
				claimed:    true,
			}
			svc := NewAutoUpstreamModelSyncService(store, &autoUpstreamModelCatalogSyncerStub{catalog: tc.catalog, err: tc.err})
			svc.runOnce(context.Background(), time.Now())
			require.Zero(t, store.updated)
		})
	}
}

func TestAutoUpstreamModelSyncDoesNotRunWhenClaimWasLost(t *testing.T) {
	store := &autoUpstreamModelSyncStoreStub{
		candidates: []AutoUpstreamModelSyncCandidate{{AccountID: 43, IntervalHours: 2}},
		accounts:   map[int64]*Account{43: {ID: 43}},
	}
	svc := NewAutoUpstreamModelSyncService(store, &autoUpstreamModelCatalogSyncerStub{
		catalog: &UpstreamModelCatalog{Models: []string{"model-a"}, LiveListAvailable: true},
	})
	svc.runOnce(context.Background(), time.Now())
	require.Zero(t, store.updated)
}
