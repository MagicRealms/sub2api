package service

import "context"

// OpsIncludedGroupSQL retains unassigned records and filters by their recorded
// group. Callers pass a fixed SQL column name.
func OpsIncludedGroupSQL(groupIDColumn string) string {
	return "COALESCE(" + groupIDColumn + ", 0) NOT IN (SELECT id FROM groups WHERE exclude_from_ops)"
}

// OpsGroupExclusionKey changes whenever the set of excluded groups changes.
// It keeps cached totals scoped to the policy used to request them.
func (s *OpsService) OpsGroupExclusionKey(ctx context.Context) (string, error) {
	if s.opsRepo == nil {
		return "", nil
	}
	return s.opsRepo.OpsGroupExclusionKey(ctx)
}
