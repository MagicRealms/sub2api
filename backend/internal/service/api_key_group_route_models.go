package service

import (
	"context"
	"sort"
)

// Model capability must be independent of temporary scheduling failures. Using
// only healthy accounts here would silently move a premium-first key's billing
// baseline to a cheaper group whenever the premium upstream is unavailable.
func (s *GatewayService) ConfiguredGroupRouteModels(ctx context.Context, group *Group, defaults []string) ([]string, error) {
	accounts, err := s.accountRepo.ListByGroup(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool)
	unmapped := len(accounts) == 0
	for i := range accounts {
		account := &accounts[i]
		if account.Platform != group.Platform && group.Platform != PlatformComposite && !mixedListingAccountAllowed(group.Platform, account) {
			continue
		}
		mapping := account.GetModelMapping()
		if len(mapping) == 0 || account.IsOpenAIPassthroughEnabled() {
			unmapped = true
		}
		for model := range mapping {
			set[model] = true
		}
	}
	if unmapped {
		for _, model := range defaults {
			set[model] = true
		}
	}
	// Explicit allowlist entries remain configured capabilities during outages,
	// including custom models absent from the built-in fallback catalog.
	if group.ModelAllowlistEnabled() {
		for _, model := range group.ModelAllowlist.Models {
			set[model] = true
		}
	}
	models := make([]string, 0, len(set))
	for model := range set {
		models = append(models, model)
	}
	if group.Platform == PlatformOpenAI {
		models = filterRetiredOpenAIModelIDs(accounts, models)
	}
	sort.Strings(models)
	if group.ModelAllowlistEnabled() {
		models = group.ModelAllowlist.FilterForListing(models)
	}
	return models, nil
}
