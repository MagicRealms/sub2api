package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// A reference-based monitor never stores a second copy of the real credential.
// It also fails authentication safely if rolled back to an older application.
const localMonitorKeyPlaceholder = "local-key-reference"

type internalMonitorProbeKey struct{}
type internalMonitorProbe struct{ keyID, groupID int64 }
type localMonitorGateway struct {
	handler http.Handler
	keys    *APIKeyService
}

// Declared here to keep the internal capability out of HTTP headers/query data.
// Only a server-created context can select a group without normal ordered fallback.
func InternalMonitorProbeGroup(ctx context.Context, keyID int64) (int64, bool) {
	p, ok := ctx.Value(internalMonitorProbeKey{}).(internalMonitorProbe)
	return p.groupID, ok && p.keyID == keyID && p.groupID > 0
}

func (s *ChannelMonitorService) SetLocalProbeGateway(h http.Handler, keys *APIKeyService) {
	if h != nil && keys != nil {
		s.localConfig.Store(&localMonitorGateway{handler: h, keys: keys})
	}
}
func (s *ChannelMonitorService) localProbeKeys() *APIKeyService {
	if cfg := s.localConfig.Load(); cfg != nil {
		return cfg.keys
	}
	return nil
}

func (s *ChannelMonitorService) localProbeGroup(ctx context.Context, keyID, groupID int64) (*APIKey, *Group, error) {
	keys := s.localProbeKeys()
	if keys == nil {
		return nil, nil, localProbeInvalid("local gateway is not ready")
	}
	key, err := keys.GetByID(ctx, keyID)
	if err != nil || key == nil {
		return nil, nil, localProbeInvalid("selected local API key is unavailable")
	}
	groups, err := keys.OrderedRouteGroups(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	for _, group := range groups {
		if group.ID == groupID {
			return key, group, nil
		}
	}
	return nil, nil, localProbeInvalid("selected group is no longer authorized for this API key")
}

// ListLocalProbeModels reads the selected group's model catalog through the
// normal in-process gateway. It never invokes model generation or billing.
func (s *ChannelMonitorService) ListLocalProbeModels(ctx context.Context, keyID, groupID int64) ([]string, error) {
	key, group, err := s.localProbeGroup(ctx, keyID, groupID)
	if err != nil {
		return nil, err
	}
	if key.User == nil || !key.User.IsAdmin() || !key.User.IsActive() || !key.IsActive() ||
		(key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt)) || group.IsExclusive || !group.IsActive() {
		return nil, localProbeInvalid("select an active administrator key and public active group")
	}
	provider := group.Platform
	if provider == "composite" {
		provider = MonitorProviderOpenAI
	}
	if _, _, ok := providerAdapterFor(provider, ""); !ok {
		return nil, localProbeInvalid("selected group does not support an active V2 probe adapter")
	}
	reqCtx, cancel := context.WithTimeout(ctx, monitorRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://localhost/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.doLocalProbe(req, keyID, groupID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model catalog HTTP %d", resp.StatusCode)
	}
	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, monitorResponseMaxBytes)).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode model catalog: %w", err)
	}
	models := make([]string, 0, len(data.Data))
	for _, item := range data.Data {
		if strings.TrimSpace(item.ID) != "" {
			models = append(models, strings.TrimSpace(item.ID))
		}
	}
	slices.Sort(models)
	return models, nil
}

func (s *ChannelMonitorService) ValidateV2Probe(ctx context.Context, actorID, keyID, groupID int64, model string) error {
	key, group, err := s.localProbeGroup(ctx, keyID, groupID)
	if err != nil {
		return err
	}
	if actorID <= 0 || key.UserID != actorID || key.User == nil || !key.User.IsAdmin() || !key.User.IsActive() || !key.IsActive() {
		return localProbeInvalid("select an active API key owned by this administrator")
	}
	if group.IsExclusive || !group.IsActive() {
		return localProbeInvalid("select a public active group")
	}
	models, err := s.ListLocalProbeModels(ctx, keyID, groupID)
	if err != nil {
		return err
	}
	for _, available := range models {
		if available == strings.TrimSpace(model) {
			return nil
		}
	}
	return localProbeInvalid("selected model is not listed by the selected group")
}

// RunV2LocalProbe executes one short challenge against exactly one selected
// group. Composite groups use the OpenAI-compatible adapter; the gateway still
// resolves the model's actual platform internally.
func (s *ChannelMonitorService) RunV2LocalProbe(ctx context.Context, keyID, groupID int64, model string) *CheckResult {
	started := time.Now()
	result := &CheckResult{Model: model, Status: MonitorStatusError, CheckedAt: started}
	key, group, err := s.localProbeGroup(ctx, keyID, groupID)
	if err != nil {
		result.Message = sanitizeErrorMessage(err.Error())
		return result
	}
	if key.User == nil || !key.User.IsAdmin() || !key.User.IsActive() || !key.IsActive() ||
		(key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt)) || group.IsExclusive || !group.IsActive() {
		result.Message = "selected API key or group is not active/public"
		return result
	}
	provider := group.Platform
	if provider == "composite" {
		provider = MonitorProviderOpenAI
	}
	if _, _, ok := providerAdapterFor(provider, ""); !ok {
		result.Message = "selected group does not support an active V2 probe adapter"
		return result
	}
	checkCtx, cancel := context.WithTimeout(ctx, monitorRequestTimeout)
	defer cancel()
	opts := &CheckOptions{LocalRequest: func(req *http.Request) (*http.Response, error) {
		return s.doLocalProbe(req, keyID, groupID)
	}}
	return runCheckForModel(checkCtx, provider, "", "", model, opts)
}
func positiveMonitorID(id *int64) *int64 {
	if id == nil || *id <= 0 {
		return nil
	}
	return cloneInt64Pointer(id)
}
func localProbeInvalid(message string) error {
	return infraerrors.BadRequest("INVALID_LOCAL_GROUP_PROBE", message)
}

func (s *ChannelMonitorService) prepareLocalCreate(ctx context.Context, p *ChannelMonitorCreateParams) error {
	if p.LocalAPIKeyID == nil && p.ProbeGroupID == nil {
		return nil
	}
	m := &ChannelMonitor{LocalAPIKeyID: p.LocalAPIKeyID, ProbeGroupID: p.ProbeGroupID, CreatedBy: p.CreatedBy, Provider: p.Provider, CheckMode: p.CheckMode}
	if err := s.validateLocalMonitor(ctx, m); err != nil {
		return err
	}
	p.APIKey = localMonitorKeyPlaceholder
	return nil
}

func (s *ChannelMonitorService) validateLocalMonitor(ctx context.Context, m *ChannelMonitor) error {
	if m.LocalAPIKeyID == nil && m.ProbeGroupID == nil {
		return nil
	}
	if m.LocalAPIKeyID == nil || m.ProbeGroupID == nil || *m.LocalAPIKeyID <= 0 || *m.ProbeGroupID <= 0 {
		return localProbeInvalid("select both a local API key and a probe group")
	}
	if defaultCheckMode(m.CheckMode) != MonitorCheckModeProbe && m.CheckMode != MonitorCheckModeCatalog {
		return localProbeInvalid("local group probes use generation mode")
	}
	keys := s.localProbeKeys()
	if keys == nil {
		return localProbeInvalid("local gateway is not ready")
	}
	key, err := keys.GetByID(ctx, *m.LocalAPIKeyID)
	if err != nil {
		return localProbeInvalid("selected local API key is unavailable")
	}
	if key.UserID != m.CreatedBy || key.User == nil || !key.User.IsAdmin() || !key.User.IsActive() || !key.IsActive() {
		return localProbeInvalid("select an active API key owned by this administrator")
	}
	if key.ExpiresAt != nil && !time.Now().Before(*key.ExpiresAt) {
		return localProbeInvalid("selected API key has expired")
	}
	groups, err := keys.OrderedRouteGroups(ctx, key)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.ID == *m.ProbeGroupID {
			if group.Platform != m.Provider && group.Platform != "composite" {
				return localProbeInvalid("probe protocol does not match the selected group")
			}
			return nil
		}
	}
	return localProbeInvalid("selected group is no longer authorized for this API key")
}

// No URL is dialed here: this invokes the complete, existing gateway in-process.
// The request still passes API-key auth, group access, quotas and normal billing.
func (s *ChannelMonitorService) doLocalProbe(req *http.Request, keyID, groupID int64) (*http.Response, error) {
	cfg := s.localConfig.Load()
	if cfg == nil {
		return nil, fmt.Errorf("local gateway unavailable")
	}
	path := req.URL.Path
	if path != "/v1/models" && path != "/v1/responses" && path != "/v1/chat/completions" && path != "/v1/messages" &&
		!(strings.HasPrefix(path, "/v1beta/models/") && strings.HasSuffix(path, ":generateContent")) {
		return nil, fmt.Errorf("unsupported local probe path")
	}
	ctx := context.WithValue(req.Context(), internalMonitorProbeKey{}, internalMonitorProbe{keyID: keyID, groupID: groupID})
	req = req.Clone(ctx)
	req.URL.Scheme = "http"
	req.URL.Host = "localhost"
	req.URL.RawQuery = ""
	req.Host = "localhost"
	req.RequestURI = path
	req.RemoteAddr = "127.0.0.1:0"
	// Custom headers may not replace the selected record's credential or fake IPs.
	key, err := cfg.keys.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	req.Header = make(http.Header)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("x-api-key", key.Key)
	req.Header.Set("x-goog-api-key", key.Key)
	req.Header.Set("anthropic-version", monitorAnthropicAPIVersion)
	response := &localMonitorResponse{header: make(http.Header)}
	cfg.handler.ServeHTTP(response, req)
	if response.overflow {
		return nil, fmt.Errorf("local probe response exceeds limit")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Header: response.header, Body: io.NopCloser(bytes.NewReader(response.body.Bytes())), Request: req}, nil
}

type localMonitorResponse struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (w *localMonitorResponse) Header() http.Header { return w.header }
func (w *localMonitorResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *localMonitorResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.body.Len()+len(p) > int(monitorResponseMaxBytes) {
		w.overflow = true
		return 0, fmt.Errorf("probe response limit exceeded")
	}
	return w.body.Write(p)
}
func (w *localMonitorResponse) Flush() {}

// A dedicated context carries the authenticated viewer for local group cards.
type localMonitorViewerKey struct{}

func (s *ChannelMonitorService) ListUserViewForUser(ctx context.Context, userID int64) ([]*UserMonitorView, error) {
	return s.ListUserView(context.WithValue(ctx, localMonitorViewerKey{}, userID))
}
func (s *ChannelMonitorService) GetUserDetailForUser(ctx context.Context, id, userID int64) (*UserMonitorDetail, error) {
	return s.GetUserDetail(context.WithValue(ctx, localMonitorViewerKey{}, userID), id)
}
func (s *ChannelMonitorService) filterLocalMonitorVisibility(ctx context.Context, monitors []*ChannelMonitor) ([]*ChannelMonitor, error) {
	needsGroups := false
	for _, m := range monitors {
		if m.ProbeGroupID != nil {
			needsGroups = true
			break
		}
	}
	if !needsGroups {
		return monitors, nil
	}
	allowed := map[int64]bool{}
	viewer, _ := ctx.Value(localMonitorViewerKey{}).(int64)
	keys := s.localProbeKeys()
	if viewer > 0 && keys != nil {
		groups, err := keys.GetAvailableGroups(ctx, viewer)
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			allowed[group.ID] = true
		}
	}
	out := make([]*ChannelMonitor, 0, len(monitors))
	for _, m := range monitors {
		if m.ProbeGroupID == nil || allowed[*m.ProbeGroupID] {
			out = append(out, m)
		}
	}
	return out, nil
}

// A catalog check is explicitly not an inference-health claim. In particular it
// avoids paying to generate an image merely to show the image-only group card.
func (s *ChannelMonitorService) runLocalCatalogCheck(ctx context.Context, m *ChannelMonitor) []*CheckResult {
	start := time.Now()
	models := append([]string{m.PrimaryModel}, m.ExtraModels...)
	result := make([]*CheckResult, 0, len(models))
	ctx, cancel := context.WithTimeout(ctx, monitorRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/v1/models", nil)
	var body []byte
	var status int
	if err == nil {
		var resp *http.Response
		resp, err = s.doLocalProbe(req, *m.LocalAPIKeyID, *m.ProbeGroupID)
		if err == nil {
			status = resp.StatusCode
			body, err = io.ReadAll(io.LimitReader(resp.Body, monitorResponseMaxBytes))
			resp.Body.Close()
		}
	}
	available := map[string]bool{}
	if err == nil && status == http.StatusOK {
		var data struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(body, &data)
		for _, model := range data.Data {
			available[model.ID] = true
		}
	}
	elapsed := int(time.Since(start) / time.Millisecond)
	for _, model := range models {
		r := &CheckResult{Model: model, CheckedAt: start, Status: MonitorStatusError, LatencyMs: &elapsed, Message: "Catalog only; upstream generation is not verified"}
		if err != nil {
			r.Message = truncateMessage(sanitizeErrorMessage(err.Error()))
		} else if status != http.StatusOK {
			r.Message = fmt.Sprintf("Local catalog HTTP %d", status)
		} else if !available[model] {
			r.Status = MonitorStatusFailed
			r.Message = "Selected group does not advertise the configured model"
		} else {
			r.Status = MonitorStatusOperational
		}
		result = append(result, r)
	}
	return result
}

func sameMonitorID(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
