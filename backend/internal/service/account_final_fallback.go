package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

const (
	AccountFinalFallbackEnabledKey  = "account_final_fallback_enabled"
	AccountFinalFallbackGroupsKey   = "account_final_fallback_group_ids"
	MaxAccountFinalFallbackAttempts = 3
	maxAccountFinalFallbackGroups   = 64
)

func accountFinalFallbackGroupIDs(extra map[string]any) ([]int64, error) {
	raw := extra[AccountFinalFallbackGroupsKey]
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid fallback groups: %w", err)
	}
	var ids []int64
	if err := json.Unmarshal(data, &ids); err != nil || len(ids) > maxAccountFinalFallbackGroups {
		return nil, fmt.Errorf("fallback group IDs must be an array of at most %d positive integers", maxAccountFinalFallbackGroups)
	}
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || id > 1<<53-1 {
			return nil, fmt.Errorf("fallback group IDs must be positive safe integers")
		}
		if !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	return result, nil
}

func hasAccountFinalFallbackSettings(extra map[string]any) bool {
	_, enabled := extra[AccountFinalFallbackEnabledKey]
	_, groups := extra[AccountFinalFallbackGroupsKey]
	return enabled || groups
}

func (a *Account) FinalFallbackGroupIDs() []int64 {
	if a == nil || a.Extra[AccountFinalFallbackEnabledKey] != true {
		return nil
	}
	ids, _ := accountFinalFallbackGroupIDs(a.Extra)
	return ids
}

func (a *Account) IsFinalFallbackForGroup(groupID *int64) bool {
	if groupID == nil || a == nil || a.Extra[AccountFinalFallbackEnabledKey] != true {
		return false
	}
	switch ids := a.Extra[AccountFinalFallbackGroupsKey].(type) {
	case []int64:
		return slices.Contains(ids, *groupID)
	case []any: // JSONB and Redis decode numbers as float64.
		return slices.Contains(ids, any(float64(*groupID)))
	case []int:
		return slices.Contains(ids, int(*groupID))
	default:
		return false
	}
}

func validateAccountFinalFallbackEditor(ctx context.Context, extra map[string]any) error {
	if hasAccountFinalFallbackSettings(extra) && adminaccess.RestrictedFromContext(ctx) != nil {
		return infraerrors.Forbidden("ACCOUNT_FINAL_FALLBACK_ADMIN_ONLY", "only a full administrator may configure final fallback")
	}
	return nil
}

func (s *adminServiceImpl) needsFinalFallbackTransaction(ctx context.Context) bool {
	return s.entClient != nil && dbent.TxFromContext(ctx) == nil
}

// Keep fallback metadata on unrelated extra edits (including quota resets).
func preserveAccountFinalFallbackSettings(current, incoming map[string]any) {
	for _, key := range []string{AccountFinalFallbackEnabledKey, AccountFinalFallbackGroupsKey} {
		if _, provided := incoming[key]; !provided {
			if value, exists := current[key]; exists {
				incoming[key] = value
			}
		}
	}
}

// The existing group edge is the authorization and snapshot-invalidation boundary.
// An account's extra identifies which of those edges are fallback-only.
func (s *adminServiceImpl) prepareAccountFinalFallbackBindings(ctx context.Context, account *Account, normalGroupIDs []int64) ([]int64, error) {
	if !hasAccountFinalFallbackSettings(account.Extra) {
		return normalGroupIDs, nil
	}
	bad := func(message string) ([]int64, error) {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_FINAL_FALLBACK", message)
	}
	if raw, present := account.Extra[AccountFinalFallbackEnabledKey]; present {
		if _, ok := raw.(bool); !ok {
			return bad("account_final_fallback_enabled must be a boolean")
		}
	}
	ids, err := accountFinalFallbackGroupIDs(account.Extra)
	if err != nil {
		return bad(err.Error())
	}
	if account.Extra[AccountFinalFallbackEnabledKey] != true {
		account.Extra[AccountFinalFallbackGroupsKey] = []int64{}
		return normalGroupIDs, nil
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return bad("final fallback groups require standard mode")
	}
	if len(ids) == 0 {
		return bad("select at least one final fallback group")
	}
	if s.groupRepo == nil {
		return nil, fmt.Errorf("group repository not configured")
	}
	for _, id := range ids {
		group, err := s.groupRepo.GetByIDLite(ctx, id)
		if err != nil {
			return nil, err
		}
		if group == nil {
			return nil, ErrGroupNotFound
		}
		if group.Platform != account.Platform && group.Platform != PlatformComposite {
			return bad("fallback account and group must use the same platform, or a composite group")
		}
		if group.RequireOAuthOnly && account.Type == AccountTypeAPIKey {
			return bad("this fallback group does not allow API-key accounts")
		}
	}
	account.Extra[AccountFinalFallbackGroupsKey] = ids
	return mergeFinalFallbackBindings(normalGroupIDs, ids), nil
}

func mergeFinalFallbackBindings(normal, reserves []int64) []int64 {
	if len(reserves) == 0 {
		return normal
	}
	bound := slices.Clone(normal)
	for _, id := range reserves {
		if !slices.Contains(bound, id) {
			bound = append(bound, id)
		}
	}
	return bound
}

type accountFallbackContextKey struct{}

// AccountFinalFallbackState is request-owned. It cannot be enabled by headers.
// One shared budget spans every selected group and both routing phases.
type AccountFinalFallbackState struct {
	mu            sync.Mutex
	attempted     map[int64]struct{}
	platforms     map[int64]string
	rpmChecked    map[int64]bool
	finalAttempts int
}

type accountFallbackAttempt struct {
	state *AccountFinalFallbackState
	group *Group
	final bool
	local map[int64]struct{}
	retry atomic.Bool
}

func NewAccountFinalFallbackState() *AccountFinalFallbackState {
	return &AccountFinalFallbackState{attempted: make(map[int64]struct{}), platforms: make(map[int64]string), rpmChecked: make(map[int64]bool)}
}

func WithAccountFinalFallbackAttempt(ctx context.Context, state *AccountFinalFallbackState, group *Group, final bool) context.Context {
	return context.WithValue(ctx, accountFallbackContextKey{}, &accountFallbackAttempt{state: state, group: group, final: final, local: make(map[int64]struct{})})
}

func IsAccountFinalFallbackAttempt(ctx context.Context) bool {
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	return attempt != nil && attempt.final
}

func RecordAccountFallbackPlatform(ctx context.Context, groupID *int64, platform string) {
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	if attempt == nil || attempt.final || groupID == nil || *groupID != attempt.group.ID || platform == "" {
		return
	}
	attempt.state.mu.Lock()
	defer attempt.state.mu.Unlock()
	if attempt.state.platforms[*groupID] == "" {
		attempt.state.platforms[*groupID] = platform
	}
}

func RecordAccountFallbackAttempt(ctx context.Context, accountID int64, platform string) {
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	if attempt == nil {
		return
	}
	attempt.state.mu.Lock()
	defer attempt.state.mu.Unlock()
	if attempt.final {
		if _, exists := attempt.local[accountID]; !exists {
			attempt.state.finalAttempts++
		}
		attempt.local[accountID] = struct{}{}
	} else if platform != "" && attempt.state.platforms[attempt.group.ID] == "" {
		attempt.state.platforms[attempt.group.ID] = platform
	}
	attempt.state.attempted[accountID] = struct{}{}
}

// AccountAllowedForSchedulingPhase applies before scoring, sticky lookup and
// fresh-account rechecks. All original capability/health/billing gates still run.
func AccountAllowedForSchedulingPhase(ctx context.Context, account *Account, groupID *int64) bool {
	if account == nil {
		return false
	}
	reserve := account.IsFinalFallbackForGroup(groupID)
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	if attempt == nil || !attempt.final {
		return !reserve
	}
	if groupID == nil || *groupID != attempt.group.ID || !reserve || !openAIStickyAccountMatchesGroup(account, groupID) {
		return false
	}
	if (attempt.group.RequireOAuthOnly && account.Type == AccountTypeAPIKey) ||
		(attempt.group.RequirePrivacySet && !account.IsPrivacySet()) {
		return false
	}
	attempt.state.mu.Lock()
	defer attempt.state.mu.Unlock()
	if platform := attempt.state.platforms[*groupID]; platform == "" || account.Platform != platform {
		return false
	}
	if _, local := attempt.local[account.ID]; local {
		return true
	}
	_, tried := attempt.state.attempted[account.ID]
	return !tried && attempt.state.finalAttempts < MaxAccountFinalFallbackAttempts
}

func filterAccountsForSchedulingPhase(ctx context.Context, accounts []Account, groupID *int64, platform string) []Account {
	RecordAccountFallbackPlatform(ctx, groupID, platform)
	for i := range accounts {
		if !AccountAllowedForSchedulingPhase(ctx, &accounts[i], groupID) {
			filtered := append(make([]Account, 0, len(accounts)-1), accounts[:i]...)
			for j := i + 1; j < len(accounts); j++ {
				if AccountAllowedForSchedulingPhase(ctx, &accounts[j], groupID) {
					filtered = append(filtered, accounts[j])
				}
			}
			return filtered
		}
	}
	return accounts
}

// Fresh/sticky paths with no explicit group argument inherit the routing scope.
func accountAllowedForRoutingAttempt(ctx context.Context, account *Account) bool {
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	if attempt == nil {
		return account != nil
	}
	return AccountAllowedForSchedulingPhase(ctx, account, &attempt.group.ID)
}

func MarkAccountFinalFallbackEligible(ctx context.Context) {
	if attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt); attempt != nil {
		attempt.retry.Store(true)
	}
}

func AccountFinalFallbackEligible(ctx context.Context) bool {
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	return attempt != nil && attempt.retry.Load()
}

func (s *AccountFinalFallbackState) Attempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finalAttempts
}

// A terminal request or provider-policy error cannot be made retryable by the
// final phase. Upstream credential and transport failures can be account-local.
func MarkAccountFinalFallbackUpstreamFailure(ctx context.Context, failoverErr *UpstreamFailoverError) {
	if !failoverErr.ShouldRetryNextAccount() || failoverErr.Scope == GatewayFailureScopeRequest || failoverErr.BoundUpstreamPassthrough {
		return
	}
	status := failoverErr.StatusCode
	if status == 401 || status == 408 || status == 429 || status >= 500 || failoverErr.IsCredentialFailure() {
		MarkAccountFinalFallbackEligible(ctx)
	}
}

func FinalFallbackSessionHash(ctx context.Context, hash string) string {
	if hash != "" && IsAccountFinalFallbackAttempt(ctx) {
		return "final-fallback:" + hash
	}
	return hash
}

func recordAccountFallbackRPMCheck(ctx context.Context, group *Group) {
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	if attempt != nil && group != nil && group.ID == attempt.group.ID {
		attempt.state.mu.Lock()
		attempt.state.rpmChecked[group.ID] = true
		attempt.state.mu.Unlock()
	}
}

func accountFinalFallbackRPMAlreadyChecked(ctx context.Context, group *Group) bool {
	attempt, _ := ctx.Value(accountFallbackContextKey{}).(*accountFallbackAttempt)
	if attempt == nil || !attempt.final || group == nil || group.ID != attempt.group.ID {
		return false
	}
	attempt.state.mu.Lock()
	defer attempt.state.mu.Unlock()
	return attempt.state.rpmChecked[group.ID]
}

type accountFinalFallbackRepository interface {
	ListAccountFinalFallbackGroups(ctx context.Context) ([]int64, error)
}

type accountFinalFallbackGroupIndex struct {
	groups  map[int64]struct{}
	expires time.Time
}

type accountFinalFallbackGroupCache struct {
	index atomic.Pointer[accountFinalFallbackGroupIndex]
	load  singleflight.Group
}

func (s *GatewayService) HasGroupFinalFallback(ctx context.Context, groupIDs []int64) (bool, error) {
	if s == nil || len(groupIDs) == 0 || (s.cfg != nil && s.cfg.RunMode == config.RunModeSimple) {
		return false, nil
	}
	repo, ok := s.accountRepo.(accountFinalFallbackRepository)
	if !ok {
		return false, nil
	}
	cache := &s.accountFinalFallbackGroups
	read := func() *accountFinalFallbackGroupIndex {
		entry := cache.index.Load()
		if entry != nil && time.Now().Before(entry.expires) {
			return entry
		}
		return nil
	}
	entry := read()
	if entry == nil {
		loaded, err, _ := cache.load.Do("groups", func() (any, error) {
			if cached := read(); cached != nil {
				return cached, nil
			}
			ids, err := repo.ListAccountFinalFallbackGroups(ctx)
			if err != nil {
				return nil, err
			}
			result := &accountFinalFallbackGroupIndex{groups: make(map[int64]struct{}, len(ids)), expires: time.Now().Add(3 * time.Second)}
			for _, id := range ids {
				result.groups[id] = struct{}{}
			}
			cache.index.Store(result)
			return result, nil
		})
		if err != nil {
			return false, err
		}
		entry = loaded.(*accountFinalFallbackGroupIndex)
	}
	for _, id := range groupIDs {
		if _, ok := entry.groups[id]; ok {
			return true, nil
		}
	}
	return false, nil
}
