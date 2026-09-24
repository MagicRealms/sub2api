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
	mock.ExpectQuery("SELECT a.id, a.updated_at").WithArgs(service.ProxyTransportCooldownPrefix).WillReturnRows(sqlmock.NewRows([]string{"id", "updated_at", "until", "reason", "proxy_id", "protocol", "host", "port", "username", "password", "proxy_updated_at"}).AddRow(1, now, now.Add(time.Minute), service.ProxyTransportCooldownPrefix+"connection refused", 3, "socks5", "example.test", 1080, "u", "p", now))
	repo := newAccountRepositoryWithSQL(nil, db, nil)
	candidates, err := repo.ListProxyTransportRecoveryCandidates(context.Background())
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(3), candidates[0].Proxy.ID)
	require.Equal(t, "socks5", candidates[0].Proxy.Protocol)
	require.NoError(t, mock.ExpectationsWereMet())
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
