//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type plazaAccountRepo struct {
	AccountRepository
	accounts []Account
	err      error
	calls    int
}

type plazaCatalogChannels struct {
	ChannelRepository
	channels []Channel
}

func (r plazaCatalogChannels) ListAll(context.Context) ([]Channel, error) { return r.channels, nil }

type plazaCatalogGroups struct {
	GroupRepository
	groups []Group
}

func (r plazaCatalogGroups) ListActive(context.Context) ([]Group, error) { return r.groups, nil }
func newAccountPlazaService(channels []Channel, groups []Group, pricing *PricingService) *ModelPlazaService {
	return NewModelPlazaService(plazaCatalogChannels{channels: channels}, plazaCatalogGroups{groups: groups}, pricing, nil, nil, nil)
}

func (r *plazaAccountRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, includeGrouped bool) ([]Account, error) {
	r.calls++
	if groupID != nil || !includeGrouped || len(platforms) == 0 {
		panic("catalog must load grouped accounts in one bulk query")
	}
	return r.accounts, r.err
}

func TestListPlazaGroups_AccountCatalogWithoutChannels(t *testing.T) {
	groups := []Group{
		{ID: 1, Name: "GPT", Platform: PlatformOpenAI},
		{ID: 2, Name: "Claude", Platform: PlatformAnthropic, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"claude-opus-*"}}},
		{ID: 3, Name: "Other GPT", Platform: PlatformOpenAI},
	}
	repo := &plazaAccountRepo{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, GroupIDs: []int64{1, 3}, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6.1-sol": "gpt-6.1-sol", "gpt-6.1-terra": "gpt-6.1-terra", "codex-auto-review": "codex-auto-review", "future-*": "future-*"}}},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, GroupIDs: []int64{2}, Credentials: map[string]any{"model_mapping": map[string]any{"claude-opus-5-5": "claude-opus-5-5", "claude-haiku-5-5": "claude-haiku-5-5"}}},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, GroupIDs: []int64{99}, Credentials: map[string]any{"model_mapping": map[string]any{"hidden": "hidden"}}},
	}}
	svc := newAccountPlazaService(nil, groups, nil)
	svc.accountRepo = repo
	result, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 3)
	require.Equal(t, 1, repo.calls)
	for _, g := range result {
		names := make([]string, 0, len(g.Models))
		for _, m := range g.Models {
			names = append(names, m.Name)
		}
		if g.ID == 2 {
			require.Equal(t, []string{"claude-opus-5-5"}, names)
		} else {
			require.Equal(t, []string{"gpt-6.1-sol", "gpt-6.1-terra"}, names)
		}
	}
}

func TestListPlazaGroups_ReconcilesStaleChannelAndKeepsNewModels(t *testing.T) {
	price := 3e-6
	svc := newAccountPlazaService([]Channel{{ID: 1, Name: "legacy", Status: StatusActive, GroupIDs: []int64{1}, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"gpt-5.6-sol", "gpt-6.1-sol"}, BillingMode: BillingModeToken, InputPrice: &price}}}}, []Group{{ID: 1, Platform: PlatformOpenAI}}, nil)
	svc.accountRepo = &plazaAccountRepo{accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, GroupIDs: []int64{1}, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6.1-sol": "gpt-6.1-sol", "gpt-6.1-terra": "gpt-6.1-terra"}}}}}
	result, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Len(t, result[0].Models, 2)
	require.Equal(t, "gpt-6.1-sol", result[0].Models[0].Name)
	require.NotNil(t, result[0].Models[0].Pricing)
	require.Equal(t, "gpt-6.1-terra", result[0].Models[1].Name)
}

func TestListPlazaGroups_AccountReadFailureDoesNotReturnIncompleteCatalog(t *testing.T) {
	svc := newAccountPlazaService(nil, nil, nil)
	sentinel := errors.New("account catalog unavailable")
	svc.accountRepo = &plazaAccountRepo{err: sentinel}
	_, err := svc.ListGroups(context.Background())
	require.ErrorIs(t, err, sentinel)
}
