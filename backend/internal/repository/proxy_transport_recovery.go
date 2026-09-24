package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.ProxyTransportRecoveryRepository = (*accountRepository)(nil)

// Limit recovery to OpenAI OAuth accounts: the probe checks the Codex endpoint.
// The database is authoritative, including when every account is out of the pool.
func (r *accountRepository) ListProxyTransportRecoveryCandidates(ctx context.Context) ([]service.ProxyTransportRecoveryCandidate, error) {
	rows, err := r.sql.QueryContext(ctx, `
 SELECT a.id, a.updated_at, a.temp_unschedulable_until, a.temp_unschedulable_reason,
 p.id, p.protocol, p.host, p.port, COALESCE(p.username,''), COALESCE(p.password,''), p.updated_at
 FROM accounts a JOIN proxies p ON p.id = a.proxy_id
 WHERE a.deleted_at IS NULL AND p.deleted_at IS NULL
 AND a.platform = 'openai' AND a.type = 'oauth' AND a.status = 'active' AND a.schedulable IS TRUE
 AND p.status = 'active' AND (p.expires_at IS NULL OR p.expires_at > NOW())
 AND a.temp_unschedulable_until > NOW()
 AND starts_with(a.temp_unschedulable_reason, $1)
 ORDER BY p.id, a.id`, service.ProxyTransportCooldownPrefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []service.ProxyTransportRecoveryCandidate
	for rows.Next() {
		var c service.ProxyTransportRecoveryCandidate
		if err := rows.Scan(&c.AccountID, &c.AccountUpdatedAt, &c.Until, &c.Reason,
			&c.Proxy.ID, &c.Proxy.Protocol, &c.Proxy.Host, &c.Proxy.Port, &c.Proxy.Username, &c.Proxy.Password, &c.Proxy.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// Compare-and-clear and scheduler notification are one atomic SQL statement.
// A new failure, manual edit, proxy change or concurrent recovery wins over this
// stale probe. Never clear rate limits, overload, status or schedulable flags.
func (r *accountRepository) ClearProxyTransportCooldownIfUnchanged(ctx context.Context, c service.ProxyTransportRecoveryCandidate) (bool, error) {
	result, err := r.sql.ExecContext(ctx, `
 WITH restored AS (
 UPDATE accounts a SET temp_unschedulable_until = NULL, temp_unschedulable_reason = NULL, updated_at = NOW()
 WHERE a.id = $1 AND a.proxy_id = $2 AND a.updated_at = $3
 AND a.temp_unschedulable_until = $4 AND a.temp_unschedulable_reason = $5
 AND starts_with(a.temp_unschedulable_reason, $6)
 AND a.deleted_at IS NULL AND a.status = 'active' AND a.schedulable IS TRUE
 AND a.platform = 'openai' AND a.type = 'oauth'
 AND EXISTS (SELECT 1 FROM proxies p WHERE p.id = a.proxy_id AND p.updated_at = $7
 AND p.deleted_at IS NULL AND p.status = 'active' AND (p.expires_at IS NULL OR p.expires_at > NOW()))
 RETURNING a.id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload)
 SELECT $8,id,NULL,NULL FROM restored`, c.AccountID, c.Proxy.ID, c.AccountUpdatedAt, c.Until, c.Reason,
		service.ProxyTransportCooldownPrefix, c.Proxy.UpdatedAt, service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, c.AccountID)
	return true, nil
}
