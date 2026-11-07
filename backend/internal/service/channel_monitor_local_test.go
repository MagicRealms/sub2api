//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestInternalMonitorProbeRequiresMatchingKey(t *testing.T) {
	ctx := context.WithValue(context.Background(), internalMonitorProbeKey{}, internalMonitorProbe{keyID: 4, groupID: 8})
	group, ok := InternalMonitorProbeGroup(ctx, 4)
	require.True(t, ok)
	require.Equal(t, int64(8), group)
	_, ok = InternalMonitorProbeGroup(ctx, 5)
	require.False(t, ok)
	_, ok = InternalMonitorProbeGroup(context.Background(), 4)
	require.False(t, ok)
}
func TestLocalMonitorResponseBoundedAndStatusPreserved(t *testing.T) {
	w := &localMonitorResponse{header: make(http.Header)}
	w.WriteHeader(http.StatusServiceUnavailable)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("small"))
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, w.status)
	_, err = w.Write(make([]byte, monitorResponseMaxBytes))
	require.Error(t, err)
	require.True(t, w.overflow)
	require.Equal(t, "small", w.body.String())
}
func TestLocalMonitorCatalogRequiresKeyPair(t *testing.T) {
	err := validateCreateParams(ChannelMonitorCreateParams{CheckMode: MonitorCheckModeCatalog})
	require.Error(t, err)
	s := NewChannelMonitorService(nil, nil)
	id := int64(1)
	require.Error(t, s.validateLocalMonitor(context.Background(), &ChannelMonitor{LocalAPIKeyID: &id}))
	require.Error(t, s.validateLocalMonitor(context.Background(), &ChannelMonitor{ProbeGroupID: &id}))
}

func TestMonitorChallengeHasSmallInputBudget(t *testing.T) {
	for i := 0; i < 100; i++ {
		challenge := generateChallenge()
		require.LessOrEqual(t, len(challenge.Prompt), 60)
		require.NotEmpty(t, challenge.Expected)
	}
}

type monitorViewerUsers struct {
	UserRepository
	user *User
}

func (r *monitorViewerUsers) GetByID(context.Context, int64) (*User, error) { return r.user, nil }

type monitorViewerGroups struct {
	GroupRepository
	groups []Group
}

func (r monitorViewerGroups) ListActive(context.Context) ([]Group, error) { return r.groups, nil }

type monitorViewerSubscriptions struct{ UserSubscriptionRepository }

func (monitorViewerSubscriptions) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

func TestLocalMonitorVisibilityFollowsCurrentGroupPermissions(t *testing.T) {
	public, private := int64(1), int64(2)
	user := &User{ID: 8, Status: StatusActive}
	keys := NewAPIKeyService(nil, &monitorViewerUsers{user: user}, monitorViewerGroups{groups: []Group{{ID: public, Status: StatusActive}, {ID: private, Status: StatusActive, IsExclusive: true}}}, monitorViewerSubscriptions{}, nil, nil, nil)
	svc := NewChannelMonitorService(nil, nil)
	svc.SetLocalProbeGateway(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), keys)
	monitors := []*ChannelMonitor{{ID: 1, ProbeGroupID: &public}, {ID: 2, ProbeGroupID: &private}, {ID: 3}}
	ctx := context.WithValue(context.Background(), localMonitorViewerKey{}, user.ID)
	got, err := svc.filterLocalMonitorVisibility(ctx, monitors)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, int64(1), got[0].ID)
	user.AllowedGroups = []int64{private}
	got, err = svc.filterLocalMonitorVisibility(ctx, monitors)
	require.NoError(t, err)
	require.Len(t, got, 3)
	user.AllowedGroups = nil
	got, err = svc.filterLocalMonitorVisibility(ctx, monitors)
	require.NoError(t, err)
	require.Len(t, got, 2)
	got, err = svc.filterLocalMonitorVisibility(context.Background(), monitors)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(3), got[0].ID)
}

type localMonitorSelectedKeys struct {
	APIKeyRepository
	key *APIKey
}

func (r localMonitorSelectedKeys) GetByID(context.Context, int64) (*APIKey, error) {
	if r.key == nil {
		return nil, ErrAPIKeyNotFound
	}
	return r.key, nil
}
func (r monitorViewerGroups) GetByID(_ context.Context, id int64) (*Group, error) {
	for _, g := range r.groups {
		if g.ID == id {
			return &g, nil
		}
	}
	return nil, ErrGroupNotFound
}
func TestLocalMonitorCreateKeepsOnlyKeyReference(t *testing.T) {
	keyID, groupID := int64(1), int64(6)
	key := &APIKey{ID: keyID, UserID: 9, Key: "sk-do-not-copy-this-credential", Status: StatusActive, GroupID: &groupID, User: &User{ID: 9, Role: RoleAdmin, Status: StatusActive}}
	keys := NewAPIKeyService(localMonitorSelectedKeys{key: key}, nil, monitorViewerGroups{groups: []Group{{ID: groupID, Platform: "openai", Status: StatusActive}}}, nil, nil, nil, nil)
	svc := NewChannelMonitorService(nil, nil)
	svc.SetLocalProbeGateway(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), keys)
	p := ChannelMonitorCreateParams{CreatedBy: 9, Provider: "openai", LocalAPIKeyID: &keyID, ProbeGroupID: &groupID, APIKey: "another-client-supplied-secret"}
	require.NoError(t, svc.prepareLocalCreate(context.Background(), &p))
	require.Equal(t, localMonitorKeyPlaceholder, p.APIKey)
	require.NotEqual(t, key.Key, p.APIKey)
}

type localMonitorRevokedRepo struct {
	ChannelMonitorRepository
	monitor *ChannelMonitor
}

func (r *localMonitorRevokedRepo) GetByID(context.Context, int64) (*ChannelMonitor, error) {
	m := *r.monitor
	return &m, nil
}
func (r *localMonitorRevokedRepo) Update(_ context.Context, m *ChannelMonitor) error {
	r.monitor = m
	return nil
}

type localMonitorIdentityCipher struct{}

func (localMonitorIdentityCipher) Encrypt(s string) (string, error) { return s, nil }
func (localMonitorIdentityCipher) Decrypt(s string) (string, error) { return s, nil }
func TestLocalMonitorWithRevokedKeyCanBeDisabledButNotEnabled(t *testing.T) {
	keyID, groupID := int64(1), int64(6)
	repo := &localMonitorRevokedRepo{monitor: &ChannelMonitor{ID: 1, CreatedBy: 9, Provider: "openai", Endpoint: "https://example.com", PrimaryModel: "gpt-6.1-sol", APIKey: localMonitorKeyPlaceholder, Enabled: true, LocalAPIKeyID: &keyID, ProbeGroupID: &groupID}}
	svc := NewChannelMonitorService(repo, localMonitorIdentityCipher{})
	keys := NewAPIKeyService(localMonitorSelectedKeys{}, nil, nil, nil, nil, nil, nil)
	svc.SetLocalProbeGateway(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), keys)
	disabled := false
	_, err := svc.Update(context.Background(), 1, ChannelMonitorUpdateParams{Enabled: &disabled})
	require.NoError(t, err)
	require.False(t, repo.monitor.Enabled)
	enabled := true
	_, err = svc.Update(context.Background(), 1, ChannelMonitorUpdateParams{Enabled: &enabled})
	require.Error(t, err)
	require.False(t, repo.monitor.Enabled)
}

func TestMonitorRejectsExplicitTruncationAndIgnoresGeminiThoughts(t *testing.T) {
	for _, tc := range []struct{ provider, mode, body string }{
		{"openai", "responses", `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output_text":"2"}`},
		{"openai", "chat_completions", `{"choices":[{"finish_reason":"length","message":{"content":"2"}}]}`},
		{"anthropic", "chat_completions", `{"stop_reason":"max_tokens","content":[{"type":"text","text":"2"}]}`},
		{"gemini", "chat_completions", `{"candidates":[{"finishReason":"MAX_TOKENS"}]}`},
	} {
		require.NotEmpty(t, monitorIncompleteReason(tc.provider, tc.mode, tc.body))
	}
	require.Empty(t, monitorIncompleteReason("openai", "responses", `{"status":"completed"}`))
	require.Equal(t, "2", extractGeminiMonitorText([]byte(`{"candidates":[{"content":{"parts":[{"thought":true,"text":"ignore thinking 999"},{"text":"2"}]}}]}`)))
}
