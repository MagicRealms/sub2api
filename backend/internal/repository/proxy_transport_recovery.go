package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.ProxyTransportRecoveryRepository = (*accountRepository)(nil)
var _ service.ProxyTransportCooldownRepository = (*accountRepository)(nil)

// An in-flight request can fail after the worker or an administrator changed
// its route. Never let that old proxy failure pause the newly selected route.
func (r *accountRepository) SetProxyTransportCooldownIfBindingUnchanged(ctx context.Context, account *service.Account, until time.Time, reason string) (bool, error) {
	var proxyID any
	if account.ProxyID != nil {
		proxyID = *account.ProxyID
	}
	result, err := r.sql.ExecContext(ctx, `
 WITH paused AS (
 UPDATE accounts a SET temp_unschedulable_until = $3, temp_unschedulable_reason = $4, updated_at = NOW()
 WHERE a.id = $1 AND a.proxy_id IS NOT DISTINCT FROM $2
 AND a.deleted_at IS NULL AND a.status = 'active' AND a.schedulable IS TRUE
 AND a.platform = 'openai' AND a.type = 'oauth'
 AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until < $3)
 AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW()
      OR starts_with(a.temp_unschedulable_reason,$5))
 RETURNING a.id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload)
 SELECT $6,id,NULL,NULL FROM paused`, account.ID, proxyID, until, reason,
		service.ProxyTransportCooldownPrefix, service.SchedulerOutboxEventAccountChanged)
	return r.finishProxyTransportRouteChange(ctx, account.ID, result, err)
}

// Limit recovery to OpenAI OAuth accounts: the probe checks the Codex endpoint.
// The database is authoritative, including when every account is out of the pool.
func (r *accountRepository) ListProxyTransportRecoveryCandidates(ctx context.Context, allowDirectFallback bool) ([]service.ProxyTransportRecoveryCandidate, error) {
	rows, err := r.sql.QueryContext(ctx, `
 SELECT a.id, a.updated_at, a.temp_unschedulable_until, COALESCE(a.temp_unschedulable_reason,''),
 a.proxy_id IS NULL,
 p.id, p.protocol, p.host, p.port, COALESCE(p.username,''), COALESCE(p.password,''), p.updated_at
 FROM accounts a JOIN proxies p ON p.id = COALESCE(a.proxy_id,a.proxy_fallback_origin_id)
 WHERE a.deleted_at IS NULL AND p.deleted_at IS NULL
 AND a.platform = 'openai' AND a.type = 'oauth' AND a.status = 'active' AND a.schedulable IS TRUE
 AND p.status = 'active' AND (p.expires_at IS NULL OR p.expires_at > NOW())
 AND ((a.proxy_id IS NOT NULL AND (
   ($2 AND a.proxy_fallback_origin_id IS NULL)
   OR (a.temp_unschedulable_until > NOW() AND starts_with(a.temp_unschedulable_reason, $1))))
 OR (a.proxy_id IS NULL AND a.proxy_fallback_origin_id IS NOT NULL
   AND a.extra @> '{"proxy_transport_fallback":true}'::jsonb))
 ORDER BY p.id, a.id`, service.ProxyTransportCooldownPrefix, allowDirectFallback)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []service.ProxyTransportRecoveryCandidate
	for rows.Next() {
		var c service.ProxyTransportRecoveryCandidate
		var until sql.NullTime
		if err := rows.Scan(&c.AccountID, &c.AccountUpdatedAt, &until, &c.Reason, &c.DirectFallback,
			&c.Proxy.ID, &c.Proxy.Protocol, &c.Proxy.Host, &c.Proxy.Port, &c.Proxy.Username, &c.Proxy.Password, &c.Proxy.UpdatedAt); err != nil {
			return nil, err
		}
		c.Until = until.Time
		result = append(result, c)
	}
	return result, rows.Err()
}

// Both route transitions atomically publish the scheduler event. The saved
// origin survives restarts. Only worker-owned transport cooldowns are removed.
// Lock the checked proxy so a concurrent reconfiguration cannot commit a stale
// probe result; account edits and fresh failures are guarded by updated_at.
func (r *accountRepository) SetProxyTransportDirectFallbackIfUnchanged(ctx context.Context, c service.ProxyTransportRecoveryCandidate) (bool, error) {
	result, err := r.sql.ExecContext(ctx, `
 WITH checked_proxy AS MATERIALIZED (
 SELECT id FROM proxies WHERE id = $2 AND updated_at = $4
 AND deleted_at IS NULL AND status = 'active' AND (expires_at IS NULL OR expires_at > NOW())
 FOR SHARE), switched AS (
 UPDATE accounts a SET proxy_id = NULL, proxy_fallback_origin_id = $2,
 extra = COALESCE(a.extra,'{}'::jsonb) || '{"proxy_transport_fallback":true}'::jsonb,
 temp_unschedulable_until = CASE WHEN starts_with(a.temp_unschedulable_reason,$5) THEN NULL ELSE a.temp_unschedulable_until END,
 temp_unschedulable_reason = CASE WHEN starts_with(a.temp_unschedulable_reason,$5) THEN NULL ELSE a.temp_unschedulable_reason END,
 updated_at = NOW()
 WHERE a.id = $1 AND a.proxy_id = $2 AND a.updated_at = $3
 AND a.proxy_fallback_origin_id IS NULL
 AND a.deleted_at IS NULL AND a.status = 'active' AND a.schedulable IS TRUE
 AND a.platform = 'openai' AND a.type = 'oauth'
 AND EXISTS (SELECT 1 FROM checked_proxy)
 RETURNING a.id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload)
 SELECT $6,id,NULL,NULL FROM switched`, c.AccountID, c.Proxy.ID, c.AccountUpdatedAt, c.Proxy.UpdatedAt,
		service.ProxyTransportCooldownPrefix, service.SchedulerOutboxEventAccountChanged)
	return r.finishProxyTransportRouteChange(ctx, c.AccountID, result, err)
}

func (r *accountRepository) RestoreProxyTransportBindingIfUnchanged(ctx context.Context, c service.ProxyTransportRecoveryCandidate) (bool, error) {
	result, err := r.sql.ExecContext(ctx, `
 WITH checked_proxy AS MATERIALIZED (
 SELECT id FROM proxies WHERE id = $2 AND updated_at = $4
 AND deleted_at IS NULL AND status = 'active' AND (expires_at IS NULL OR expires_at > NOW())
 FOR SHARE), restored AS (
 UPDATE accounts a SET proxy_id = $2, proxy_fallback_origin_id = NULL,
 extra = a.extra - 'proxy_transport_fallback',
 temp_unschedulable_until = CASE WHEN starts_with(a.temp_unschedulable_reason,$5) THEN NULL ELSE a.temp_unschedulable_until END,
 temp_unschedulable_reason = CASE WHEN starts_with(a.temp_unschedulable_reason,$5) THEN NULL ELSE a.temp_unschedulable_reason END,
 updated_at = NOW()
 WHERE a.id = $1 AND a.proxy_id IS NULL AND a.proxy_fallback_origin_id = $2 AND a.updated_at = $3
 AND a.extra @> '{"proxy_transport_fallback":true}'::jsonb
 AND a.deleted_at IS NULL AND a.status = 'active' AND a.schedulable IS TRUE
 AND a.platform = 'openai' AND a.type = 'oauth'
 AND EXISTS (SELECT 1 FROM checked_proxy)
 RETURNING a.id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload)
 SELECT $6,id,NULL,NULL FROM restored`, c.AccountID, c.Proxy.ID, c.AccountUpdatedAt, c.Proxy.UpdatedAt,
		service.ProxyTransportCooldownPrefix, service.SchedulerOutboxEventAccountChanged)
	return r.finishProxyTransportRouteChange(ctx, c.AccountID, result, err)
}

func (r *accountRepository) finishProxyTransportRouteChange(ctx context.Context, accountID int64, result sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, accountID)
	return true, nil
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
