package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type autoUpstreamModelSyncStoreStub struct {
	candidates []AutoUpstreamModelSyncCandidate
	accounts   map[int64]*Account
	claimed    bool
	mappingID  int64
	mapping    map[string]any
	updated    int
}

func (s *autoUpstreamModelSyncStoreStub) ListDueAutoUpstreamModelSync(context.Context, time.Time) ([]AutoUpstreamModelSyncCandidate, error) {
	return s.candidates, nil
}

func (s *autoUpstreamModelSyncStoreStub) ClaimAutoUpstreamModelSync(_ context.Context, _ int64, _ float64, _ time.Time) (bool, error) {
	return s.claimed, nil
}

func (s *autoUpstreamModelSyncStoreStub) GetByID(_ context.Context, id int64) (*Account, error) {
	return s.accounts[id], nil
}

func (s *autoUpstreamModelSyncStoreStub) UpdateAccountModelMapping(_ context.Context, id int64, mapping map[string]any) error {
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
