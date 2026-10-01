package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProxyTransportRecoveryCandidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Now()
	mock.ExpectQuery("SELECT a.id, a.updated_at").WithArgs(service.ProxyTransportCooldownPrefix, false).WillReturnRows(sqlmock.NewRows([]string{"id", "updated_at", "until", "reason", "direct_fallback", "proxy_id", "protocol", "host", "port", "username", "password", "proxy_updated_at"}).AddRow(1, now, now.Add(time.Minute), service.ProxyTransportCooldownPrefix+"connection refused", false, 3, "socks5", "example.test", 1080, "u", "p", now).AddRow(2, now, nil, "", true, 3, "socks5", "example.test", 1080, "u", "p", now))
	repo := newAccountRepositoryWithSQL(nil, db, nil)
	candidates, err := repo.ListProxyTransportRecoveryCandidates(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, int64(3), candidates[0].Proxy.ID)
	require.Equal(t, "socks5", candidates[0].Proxy.Protocol)
	require.True(t, candidates[1].DirectFallback)
	require.True(t, candidates[1].Until.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyTransportFallbackTransitionsAreConditionalAndAtomic(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "restore"}[restore], func(t *testing.T) {
			exec := &recordingSQLExecutor{result: rowsAffectedResult(0)}
			repo := newAccountRepositoryWithSQL(nil, exec, nil)
			var changed bool
			var err error
			if restore {
				changed, err = repo.RestoreProxyTransportBindingIfUnchanged(context.Background(), service.ProxyTransportRecoveryCandidate{})
			} else {
				changed, err = repo.SetProxyTransportDirectFallbackIfUnchanged(context.Background(), service.ProxyTransportRecoveryCandidate{})
			}
			require.NoError(t, err)
			require.False(t, changed)
			require.Len(t, exec.execQueries, 1)
			q := normalizeSQLWhitespace(exec.execQueries[0])
			for _, guard := range []string{"a.updated_at = $3", "updated_at = $4", "FOR SHARE", "a.schedulable IS TRUE", "a.status = 'active'", "a.type = 'oauth'", "INSERT INTO scheduler_outbox", "CASE WHEN starts_with(a.temp_unschedulable_reason,$5)"} {
				require.Contains(t, q, guard)
			}
			if restore {
				require.Contains(t, q, "a.proxy_id IS NULL AND a.proxy_fallback_origin_id = $2")
				require.Contains(t, q, "a.extra @> '{\"proxy_transport_fallback\":true}'::jsonb")
			} else {
				require.Contains(t, q, "a.proxy_id = $2")
				require.Contains(t, q, "a.proxy_fallback_origin_id IS NULL")
			}
			require.NotRegexp(t, regexp.MustCompile(`SET[^W]+(?:rate_limit_reset_at|overload_until|schedulable|status)\s*=`), q)
		})
	}
}

func TestProxyTransportRecoveryClearIsConditionalAndAtomic(t *testing.T) {
	exec := &recordingSQLExecutor{result: rowsAffectedResult(0)}
	repo := newAccountRepositoryWithSQL(nil, exec, nil)
	changed, err := repo.ClearProxyTransportCooldownIfUnchanged(context.Background(), service.ProxyTransportRecoveryCandidate{})
	require.NoError(t, err)
	require.False(t, changed)
	require.Len(t, exec.execQueries, 1)
	q := normalizeSQLWhitespace(exec.execQueries[0])
	for _, guard := range []string{"a.proxy_id = $2", "a.updated_at = $3", "a.temp_unschedulable_until = $4", "a.temp_unschedulable_reason = $5", "starts_with(a.temp_unschedulable_reason, $6)", "p.updated_at = $7", "a.schedulable IS TRUE", "p.status = 'active'", "INSERT INTO scheduler_outbox"} {
		require.Contains(t, q, guard)
	}
	require.NotRegexp(t, regexp.MustCompile(`SET[^W]+(?:rate_limit_reset_at|overload_until|schedulable|status)\s*=`), q)
}
