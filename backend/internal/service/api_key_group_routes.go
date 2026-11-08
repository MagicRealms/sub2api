package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const MaxAPIKeyGroupRoutes = 16

// Optional batch projection keeps request policy fresh without loading account
// counts once per selected group. Small test repositories retain their fallback.
type groupRoutePolicyRepository interface {
	GetByIDsLite(context.Context, []int64) ([]*Group, error)
}

func (k *APIKey) OrderedGroupIDs() []int64 {
	if k == nil {
		return []int64{}
	}
	if len(k.GroupIDs) > 0 {
		return append([]int64(nil), k.GroupIDs...)
	}
	if k.GroupID != nil && *k.GroupID > 0 {
		return []int64{*k.GroupID}
	}
	return []int64{}
}

func firstAPIKeyGroupID(ids []int64) *int64 {
	if len(ids) == 0 {
		return nil
	}
	id := ids[0]
	return &id
}

func normalizeAPIKeyGroupRoutes(legacy *int64, ids []int64) ([]int64, error) {
	if ids == nil && legacy != nil {
		ids = []int64{*legacy}
	}
	if len(ids) > MaxAPIKeyGroupRoutes {
		return nil, infraerrors.BadRequest("INVALID_GROUP_ROUTES", "at most 16 ordered groups may be selected")
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return nil, infraerrors.BadRequest("INVALID_GROUP_ROUTES", "group routes must contain unique positive group IDs")
		}
		seen[id] = true
	}
	if legacy != nil && (len(ids) == 0 || *legacy != ids[0]) {
		return nil, infraerrors.BadRequest("INVALID_GROUP_ROUTES", "group_id must match the first ordered group")
	}
	return append([]int64{}, ids...), nil
}

func (s *APIKeyService) validateAPIKeyGroupRoutes(ctx context.Context, user *User, ids []int64) error {
	for _, id := range ids {
		group, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("get route group: %w", err)
		}
		if group == nil || !s.canUserBindGroup(ctx, user, group) {
			return ErrGroupNotAllowed
		}
	}
	return nil
}

// dropDeletedAPIKeyGroupRoutes removes historical route IDs whose groups were
// soft-deleted. Group deletion intentionally leaves those IDs on the key so
// monitor/audit snapshots can explain why the old route stopped working. When
// a user edits the key, however, those historical IDs must not poison the new
// ordered route submitted by the UI.
func (s *APIKeyService) dropDeletedAPIKeyGroupRoutes(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}
	active := make([]int64, 0, len(ids))
	for _, id := range ids {
		group, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				continue
			}
			return nil, fmt.Errorf("get route group: %w", err)
		}
		if group == nil {
			continue
		}
		active = append(active, id)
	}
	// An edit that contains only stale/deleted IDs should fail closed instead
	// of silently turning a still-routable key into an ungrouped key.
	if len(active) == 0 {
		return nil, ErrGroupNotAllowed
	}
	return active, nil
}

// OrderedRouteGroups always reloads group policy. Secondary routes cannot retain
// access after a group is disabled, removed, or made exclusive.
func (s *APIKeyService) OrderedRouteGroups(ctx context.Context, key *APIKey) ([]*Group, error) {
	if key == nil || key.User == nil {
		return nil, ErrGroupNotAllowed
	}
	ids := key.OrderedGroupIDs()
	if len(ids) > MaxAPIKeyGroupRoutes {
		return nil, ErrGroupNotAllowed
	}
	groups := make([]*Group, 0, len(ids))
	var byID map[int64]*Group
	if repo, ok := s.groupRepo.(groupRoutePolicyRepository); ok {
		loaded, err := repo.GetByIDsLite(ctx, ids)
		if err != nil {
			return nil, err
		}
		byID = make(map[int64]*Group, len(loaded))
		for _, group := range loaded {
			if group != nil {
				byID[group.ID] = group
			}
		}
	}
	for _, id := range ids {
		var group *Group
		var err error
		if byID != nil {
			group = byID[id]
		} else {
			group, err = s.groupRepo.GetByID(ctx, id)
		}
		if errors.Is(err, ErrGroupNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if group != nil && group.IsActive() && s.canUserBindGroup(ctx, key.User, group) {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

type apiKeyGroupRouteContextKey struct{}
type apiKeyGroupRouteAttempt struct {
	Billed       atomic.Bool
	KeyID        int64
	Group        *Group
	BillingFloor *Group
	Model        string
}

// WithAPIKeyGroupRoute is server-owned request state, never an HTTP header.
func WithAPIKeyGroupRoute(ctx context.Context, keyID int64, group, floor *Group, model ...string) context.Context {
	attempt := &apiKeyGroupRouteAttempt{KeyID: keyID, Group: group, BillingFloor: floor}
	if len(model) > 0 {
		attempt.Model = model[0]
	}
	return context.WithValue(ctx, apiKeyGroupRouteContextKey{}, attempt)
}

func IsAPIKeyGroupRouteAttempt(ctx context.Context) bool {
	_, ok := ctx.Value(apiKeyGroupRouteContextKey{}).(*apiKeyGroupRouteAttempt)
	return ok
}

func (s *APIKeyService) ResolveAPIKeyGroupRoute(ctx context.Context, key *APIKey) (*APIKey, error) {
	attempt, ok := ctx.Value(apiKeyGroupRouteContextKey{}).(*apiKeyGroupRouteAttempt)
	if !ok {
		return key, nil
	}
	if key == nil || key.ID != attempt.KeyID || attempt.Group == nil ||
		!slices.Contains(key.OrderedGroupIDs(), attempt.Group.ID) || !attempt.Group.IsActive() {
		return nil, ErrGroupNotAllowed
	}
	copyKey := *key
	routeGroup := *attempt.Group
	// Explicit key routes own cross-group ordering. A group's legacy fallback
	// must not jump to an unselected or differently priced group behind the key.
	routeGroup.FallbackGroupID = nil
	routeGroup.FallbackGroupIDOnInvalidRequest = nil
	copyKey.Group = &routeGroup
	copyKey.GroupID = firstAPIKeyGroupID([]int64{attempt.Group.ID})
	copyKey.RoutingBillingFloor = attempt.BillingFloor
	copyKey.RoutingPricingAt = APIKeyGroupRoutePricingAt(ctx, time.Time{})
	copyKey.RoutingModel = attempt.Model
	return &copyKey, nil
}

func orderedRoutingFloorKey(key *APIKey) *APIKey {
	if key == nil || key.Group == nil || key.RoutingBillingFloor == nil || key.RoutingBillingFloor.ID == key.Group.ID {
		return nil
	}
	floor := *key
	floor.Group = key.RoutingBillingFloor
	floor.GroupID = firstAPIKeyGroupID([]int64{floor.Group.ID})
	floor.RoutingBillingFloor = nil
	return &floor
}

type apiKeyRouteDiscoveryContextKey struct{}

func WithAPIKeyRouteDiscovery(ctx context.Context, groups []*Group) context.Context {
	return context.WithValue(ctx, apiKeyRouteDiscoveryContextKey{}, groups)
}
func APIKeyRouteDiscovery(ctx context.Context) []*Group {
	groups, _ := ctx.Value(apiKeyRouteDiscoveryContextKey{}).([]*Group)
	return groups
}

func MarkAPIKeyGroupRouteBilled(ctx context.Context) {
	if attempt, ok := ctx.Value(apiKeyGroupRouteContextKey{}).(*apiKeyGroupRouteAttempt); ok {
		attempt.Billed.Store(true)
	}
}
func APIKeyGroupRouteBilled(ctx context.Context) bool {
	attempt, ok := ctx.Value(apiKeyGroupRouteContextKey{}).(*apiKeyGroupRouteAttempt)
	return ok && attempt.Billed.Load()
}

type apiKeyGroupRoutePricingAtKey struct{}

func WithAPIKeyGroupRoutePricingAt(ctx context.Context, at time.Time) context.Context {
	return context.WithValue(ctx, apiKeyGroupRoutePricingAtKey{}, at)
}
func APIKeyGroupRoutePricingAt(ctx context.Context, fallback time.Time) time.Time {
	if at, ok := ctx.Value(apiKeyGroupRoutePricingAtKey{}).(time.Time); ok && !at.IsZero() {
		return at
	}
	return fallback
}
