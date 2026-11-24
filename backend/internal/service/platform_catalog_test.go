//go:build unit

package service

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/util/errorprivacy"
	"github.com/stretchr/testify/require"
)

func TestPublicPlatformCatalogHidesOnlyPrivateProvider(t *testing.T) {
	public := BuildPublicPlatformCatalog()
	raw, err := json.Marshal(public)
	require.NoError(t, err)
	require.NotContains(t, strings.ToLower(string(raw)), "sharellm")
	admin := BuildPlatformCatalog()
	for i, entry := range admin.Platforms {
		if entry.ID != domain.PlatformShareLLM {
			require.Equal(t, entry, public.Platforms[i], entry.ID)
			continue
		}
		require.Equal(t, errorprivacy.PublicRouteID, public.Platforms[i].ID)
		require.Equal(t, errorprivacy.PublicRouteAlias, public.Platforms[i].DisplayName)
		require.Equal(t, DefaultShareLLMBaseURL, entry.MultiProtocol.Modes[0].BaseURLs[APIProtocolChatCompletions])
		require.Empty(t, public.Platforms[i].MultiProtocol.Modes[0].BaseURLs)
	}
	// Generating a public view must never change the real routing defaults.
	require.Equal(t, DefaultShareLLMBaseURL, LookupProviderProfile(domain.PlatformShareLLM).DefaultBaseURL(AccountModePayG, APIProtocolChatCompletions))
}

func TestBuildPlatformCatalogMirrorsPlatformListAndProfiles(t *testing.T) {
	catalog := BuildPlatformCatalog()
	require.Equal(t, domain.CompositePrecedencePlatformIDs(), catalog.CompositePrecedence)
	require.Len(t, catalog.Platforms, len(domain.ConcretePlatformIDs()))

	for i, entry := range catalog.Platforms {
		require.Equal(t, domain.ConcretePlatformIDs()[i], entry.ID)
		profile := LookupProviderProfile(entry.ID)
		if profile == nil {
			require.Nil(t, entry.MultiProtocol, entry.ID)
			continue
		}
		require.NotNil(t, entry.MultiProtocol, entry.ID)
		require.Equal(t, profile.DefaultMode, entry.MultiProtocol.DefaultMode, entry.ID)
		require.Equal(t, profile.DefaultMode, entry.MultiProtocol.Modes[0].Mode, entry.ID)
		require.Equal(t, profile.ResponsesPath, entry.MultiProtocol.ResponsesPath, entry.ID)
		require.Len(t, entry.MultiProtocol.Modes, len(profile.Modes), entry.ID)
		for _, mode := range entry.MultiProtocol.Modes {
			endpoints := profile.Modes[mode.Mode]
			require.Equal(t, endpoints.BaseURLs, mode.BaseURLs, "%s/%s", entry.ID, mode.Mode)
			require.Equal(t, len(endpoints.ProtocolRules), len(mode.ProtocolRules), "%s/%s", entry.ID, mode.Mode)
			for j, rule := range endpoints.ProtocolRules {
				require.Equal(t, rule, mode.ProtocolRules[j])
			}
		}
	}
}

func TestBuildPlatformCatalogJSONShape(t *testing.T) {
	raw, err := json.Marshal(BuildPlatformCatalog())
	require.NoError(t, err)

	type modeJSON struct {
		Mode          string            `json:"mode"`
		BaseURLs      map[string]string `json:"base_urls"`
		ProtocolRules []ProtocolRule    `json:"protocol_rules"`
	}
	type platformJSON struct {
		ID            string `json:"id"`
		DisplayName   string `json:"display_name"`
		Gateway       string `json:"gateway"`
		CNProvider    *bool  `json:"cn_provider"`
		MultiProtocol *struct {
			DefaultMode   string     `json:"default_mode"`
			Routing       string     `json:"routing"`
			ResponsesPath string     `json:"responses_path"`
			Modes         []modeJSON `json:"modes"`
		} `json:"multi_protocol"`
	}
	var decoded struct {
		Platforms           []platformJSON `json:"platforms"`
		CompositePrecedence []string       `json:"composite_precedence"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, domain.CompositePrecedencePlatformIDs(), decoded.CompositePrecedence)
	byID := map[string]platformJSON{}
	for _, p := range decoded.Platforms {
		byID[p.ID] = p
	}

	anthropic := byID[PlatformAnthropic]
	require.Equal(t, "Anthropic", anthropic.DisplayName)
	require.Equal(t, "anthropic", anthropic.Gateway)
	require.NotNil(t, anthropic.CNProvider)
	require.False(t, *anthropic.CNProvider)
	require.Nil(t, anthropic.MultiProtocol)
	require.NotContains(t, string(raw), `"multi_protocol":null`)

	deepseek := byID[PlatformDeepseek].MultiProtocol
	require.NotNil(t, deepseek)
	require.Equal(t, "by_inbound", deepseek.Routing)
	require.Equal(t, "/responses", deepseek.ResponsesPath)

	openCode := byID[PlatformOpenCodeGo].MultiProtocol
	require.NotNil(t, openCode)
	require.Equal(t, "by_model", openCode.Routing)
	require.Equal(t, AccountModeGo, openCode.DefaultMode)
	require.Len(t, openCode.Modes, 2)
	require.Equal(t, AccountModeGo, openCode.Modes[0].Mode)
	require.Equal(t, DefaultOpenCodeGoBaseURL, openCode.Modes[0].BaseURLs[APIProtocolChatCompletions])
	require.Equal(t, DefaultOpenCodeGoProtocolRules(), openCode.Modes[0].ProtocolRules)
	require.Equal(t, AccountModeZen, openCode.Modes[1].Mode)
}

// 下发的是副本：调用方修改结果不影响内置 profile。
func TestBuildPlatformCatalogReturnsCopies(t *testing.T) {
	catalog := BuildPlatformCatalog()
	for _, entry := range catalog.Platforms {
		if entry.MultiProtocol == nil {
			continue
		}
		for _, mode := range entry.MultiProtocol.Modes {
			for protocol := range mode.BaseURLs {
				mode.BaseURLs[protocol] = "mutated"
			}
			for i := range mode.ProtocolRules {
				mode.ProtocolRules[i].Protocol = "mutated"
			}
		}
	}
	require.Equal(t, DefaultOpenCodeGoBaseURL, LookupProviderProfile(PlatformOpenCodeGo).DefaultBaseURL(AccountModeGo, APIProtocolChatCompletions))
	require.NotEqual(t, "mutated", LookupProviderProfile(PlatformOpenCodeGo).Modes[AccountModeGo].ProtocolRules[0].Protocol)
}

// 前端内置的平台清单（接口加载前与加载失败时使用）必须与后端一致。
// 更新平台清单或 profile 后运行：
//
//	UPDATE_PLATFORM_CATALOG=1 go test -tags unit ./internal/service/ -run TestFrontendBuiltinPlatformCatalogInSync
func TestFrontendBuiltinPlatformCatalogInSync(t *testing.T) {
	const path = "../../../frontend/src/constants/platformCatalog.builtin.json"
	want, err := json.MarshalIndent(BuildPublicPlatformCatalog(), "", "  ")
	require.NoError(t, err)
	want = append(want, '\n')

	if os.Getenv("UPDATE_PLATFORM_CATALOG") == "1" {
		require.NoError(t, os.WriteFile(path, want, 0o644))
		return
	}
	got, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("frontend sources not available")
	}
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got),
		"frontend/src/constants/platformCatalog.builtin.json 与后端平台清单不一致，按测试注释重新生成")
}
