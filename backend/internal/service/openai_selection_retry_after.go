package service

import "time"

// Compute a hint only on the failed-selection path, and only when every
// candidate was excluded by a known finite model cooldown. Empty pools,
// unsupported models, policy/profit vetoes, request exclusions and persisted
// quota/auth cooldowns must retain their existing classification and behavior.
func (s *OpenAIGatewayService) openAISelectionModelCooldownRetryAt(accounts []Account, model string, stats openAISelectionFilterStats) time.Time {
	if s == nil || len(accounts) == 0 || len(stats.reasons) != 1 || stats.reasons["runtime_blocked"] != len(accounts) {
		return time.Time{}
	}
	state := s.getOpenAIAccountModelTransientState()
	now := time.Now()
	var earliest time.Time
	for i := range accounts {
		account := &accounts[i]
		if accountPersistedSchedulingCooldownActive(account) {
			return time.Time{}
		}
		until := state.blockedUntil(account.ID, canonicalOpenAIAccountSchedulingModel(account, model), now)
		if until.IsZero() {
			return time.Time{}
		}
		if earliest.IsZero() || until.Before(earliest) {
			earliest = until
		}
	}
	return earliest
}
