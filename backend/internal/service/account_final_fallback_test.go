//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinalFallbackAccountScopeAndBudget(t *testing.T) {
	ctx := context.Background()
	a := &Group{ID: 1, Platform: PlatformOpenAI}
	b := &Group{ID: 2, Platform: PlatformComposite}
	c := &Group{ID: 3, Platform: PlatformOpenAI}
	state := NewAccountFinalFallbackState()
	primary := WithAccountFinalFallbackAttempt(ctx, state, a, false)
	reserve := Account{ID: 50, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, GroupIDs: []int64{1, 2, 3}, Extra: map[string]any{
		AccountFinalFallbackEnabledKey: true, AccountFinalFallbackGroupsKey: []any{float64(2), float64(3)},
	}}
	require.True(t, AccountAllowedForSchedulingPhase(primary, &reserve, &a.ID))
	require.False(t, AccountAllowedForSchedulingPhase(ctx, &reserve, &b.ID))
	RecordAccountFallbackAttempt(primary, reserve.ID, PlatformOpenAI)
	for _, group := range []*Group{b, c} {
		RecordAccountFallbackPlatform(WithAccountFinalFallbackAttempt(ctx, state, group, false), &group.ID, PlatformOpenAI)
	}
	finalB := WithAccountFinalFallbackAttempt(ctx, state, b, true)
	finalC := WithAccountFinalFallbackAttempt(ctx, state, c, true)
	require.False(t, AccountAllowedForSchedulingPhase(finalB, &reserve, &b.ID), "a normal attempt in another group must not be repeated")
	otherPlatform := reserve
	otherPlatform.ID, otherPlatform.Platform = 100, PlatformDeepseek
	require.False(t, AccountAllowedForSchedulingPhase(finalB, &otherPlatform, &b.ID))
	missingBinding := reserve
	missingBinding.ID, missingBinding.GroupIDs = 101, []int64{1}
	require.False(t, AccountAllowedForSchedulingPhase(finalB, &missingBinding, &b.ID))
	for i := 0; i < MaxAccountFinalFallbackAttempts; i++ {
		candidate := reserve
		candidate.ID += int64(i + 1)
		require.True(t, AccountAllowedForSchedulingPhase(finalB, &candidate, &b.ID))
		RecordAccountFallbackAttempt(finalB, candidate.ID, PlatformOpenAI)
		RecordAccountFallbackAttempt(finalB, candidate.ID, PlatformOpenAI)
		require.False(t, AccountAllowedForSchedulingPhase(finalC, &candidate, &c.ID), "reserve IDs are deduplicated across groups")
	}
	require.Equal(t, MaxAccountFinalFallbackAttempts, state.Attempts())
	reserve.ID++
	require.False(t, AccountAllowedForSchedulingPhase(finalC, &reserve, &c.ID))
	reserve.ID = 500
	require.False(t, AccountAllowedForSchedulingPhase(finalC, &reserve, &c.ID), "request-wide reserve budget is exhausted")
}

func TestCreateAccountFinalFallbackAcceptsAPIKeyAndValidatesGroupPlatform(t *testing.T) {
	repo := &accountRepoStubForBulkUpdate{createID: 7}
	groups := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive},
		2: {ID: 2, Platform: PlatformComposite, Status: StatusActive},
		3: {ID: 3, Platform: PlatformDeepseek, Status: StatusActive},
	}}
	svc := &adminServiceImpl{accountRepo: repo, groupRepo: groups}
	input := CreateAccountInput{Name: "reserve", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, GroupIDs: []int64{1},
		SkipMixedChannelCheck: true, SkipDefaultGroupBind: true, Extra: map[string]any{
			AccountFinalFallbackEnabledKey: true, AccountFinalFallbackGroupsKey: []any{float64(2), float64(2)},
		}}
	account, err := svc.CreateAccount(context.Background(), &input)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, repo.bindGroupsByAccount[7])
	require.Equal(t, []int64{2}, account.FinalFallbackGroupIDs())
	input.Extra[AccountFinalFallbackGroupsKey] = []int64{3}
	_, err = svc.CreateAccount(context.Background(), &input)
	require.ErrorContains(t, err, "same platform")
}

func TestFinalFallbackAccountSkippedByStrictSchedulerUntilFinalPhase(t *testing.T) {
	svc, accounts := strictPriorityTestService()
	group := &Group{ID: strictPriorityTestGroup, Platform: PlatformOpenAI}
	accounts[0].Extra = map[string]any{
		AccountFinalFallbackEnabledKey: true,
		AccountFinalFallbackGroupsKey:  []int64{group.ID},
	}
	state := NewAccountFinalFallbackState()
	selectAccount := func(ctx context.Context, excluded map[int64]struct{}) (*AccountSelectionResult, error) {
		result, _, err := svc.SelectAccountWithScheduler(ctx, &group.ID, "", "", "gpt-6.1-sol", excluded, OpenAIUpstreamTransportAny, false)
		return result, err
	}
	primary := WithAccountFinalFallbackAttempt(context.Background(), state, group, false)
	result, err := selectAccount(primary, nil)
	require.NoError(t, err)
	require.Equal(t, accounts[1].ID, result.Account.ID)
	result.ReleaseFunc()
	_, err = selectAccount(primary, map[int64]struct{}{accounts[1].ID: {}, accounts[2].ID: {}})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	final := WithAccountFinalFallbackAttempt(context.Background(), state, group, true)
	result, err = selectAccount(final, nil)
	require.NoError(t, err)
	require.Equal(t, accounts[0].ID, result.Account.ID)
	result.ReleaseFunc()
}
