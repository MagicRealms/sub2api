//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// legacyProviderDefaultBaseURL 复述 provider profile 引入前 account.go 中逐平台
// switch 的默认端点逻辑，作为等价性基准。
func legacyProviderDefaultBaseURL(platform, mode, protocol string) string {
	coding := mode == AccountModeCoding
	zen := mode == AccountModeZen
	switch protocol {
	case APIProtocolAnthropic:
		switch platform {
		case PlatformKimi:
			if coding {
				return DefaultKimiCodingAnthropicBaseURL
			}
			return DefaultKimiPayGAnthropicBaseURL
		case PlatformZhipu:
			return DefaultZhipuAnthropicBaseURL
		case PlatformDeepseek:
			return DefaultDeepseekAnthropicBaseURL
		case PlatformMiniMax:
			return DefaultMiniMaxAnthropicBaseURL
		case PlatformOpenCodeGo:
			if zen {
				return DefaultOpenCodeZenAnthropicBaseURL
			}
			return DefaultOpenCodeGoAnthropicBaseURL
		}
	case APIProtocolChatCompletions, APIProtocolResponses:
		if platform == PlatformZhipu && protocol == APIProtocolResponses {
			// 唯一的函数级差异：旧 switch 对智谱 Responses 也返回 CC 基址，profile
			// 以"缺条目即不支持"建模后返回空串。所有运行时调用方都先经
			// UsesNativeCNResponses / SupportsNativeCNResponses 守卫，智谱不会走到这里。
			return ""
		}
		switch platform {
		case PlatformKimi:
			if coding {
				return DefaultKimiCodingBaseURL
			}
			return DefaultKimiPayGBaseURL
		case PlatformZhipu:
			if coding {
				return DefaultZhipuCodingBaseURL
			}
			return DefaultZhipuPayGBaseURL
		case PlatformDeepseek:
			return DefaultDeepseekBaseURL
		case PlatformMiniMax:
			return DefaultMiniMaxBaseURL
		case PlatformOpenCodeGo:
			if zen {
				return DefaultOpenCodeZenBaseURL
			}
			return DefaultOpenCodeGoBaseURL
		}
	}
	return ""
}

func TestProviderProfile_DefaultBaseURLsMatchLegacySwitches(t *testing.T) {
	t.Parallel()

	platforms := []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo}
	modes := []string{"", AccountModePayG, AccountModeCoding, AccountModeZen, AccountModeGo, " coding ", "unknown"}

	for _, platform := range platforms {
		for _, mode := range modes {
			legacyMode := mode
			if legacyMode == " coding " {
				legacyMode = AccountModeCoding
			}
			newAccount := func(protocol string) *Account {
				return &Account{
					Platform: platform,
					Type:     AccountTypeAPIKey,
					Credentials: map[string]any{
						"api_key":      "sk-test",
						"account_mode": mode,
						"api_protocol": protocol,
					},
				}
			}
			name := platform + "/" + mode

			chat := newAccount(APIProtocolChatCompletions)
			require.Equal(t, legacyProviderDefaultBaseURL(platform, legacyMode, APIProtocolChatCompletions), chat.GetOpenAIBaseURL(), name)

			anthropic := newAccount(APIProtocolAnthropic)
			require.Equal(t, legacyProviderDefaultBaseURL(platform, legacyMode, APIProtocolAnthropic), anthropic.GetAnthropicProtocolBaseURL(), name)
			require.Equal(t, legacyProviderDefaultBaseURL(platform, legacyMode, APIProtocolChatCompletions), anthropic.GetOpenAIFormatBaseURL(), name)

			adaptive := newAccount(APIProtocolAdaptive)
			for _, protocol := range []string{APIProtocolChatCompletions, APIProtocolResponses, APIProtocolAnthropic, "unknown"} {
				require.Equal(t, legacyProviderDefaultBaseURL(platform, legacyMode, protocol), adaptive.GetCNProtocolBaseURL(protocol), name+"/"+protocol)
			}
		}
	}
}

func TestProviderProfile_ProfilesMatchMultiProtocolPlatforms(t *testing.T) {
	t.Parallel()

	for _, platform := range []string{
		PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok,
		PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo, PlatformShareLLM, PlatformMiMo, PlatformComposite, "",
	} {
		want := IsCNProvider(platform) || platform == PlatformOpenCodeGo || platform == PlatformShareLLM || platform == PlatformMiMo
		require.Equal(t, want, IsMultiProtocolAPIKeyProvider(platform), platform)
		require.Equal(t, want, LookupProviderProfile(platform) != nil, platform)
	}

	for platform, profile := range providerProfiles {
		require.Equal(t, platform, profile.Platform)
		_, ok := profile.Modes[profile.DefaultMode]
		require.True(t, ok, "%s default mode %q must be registered", platform, profile.DefaultMode)
	}
}

func TestProviderProfile_NativeResponsesMatchesLegacy(t *testing.T) {
	t.Parallel()

	legacy := map[string]bool{
		PlatformDeepseek:   true,
		PlatformKimi:       true,
		PlatformMiniMax:    true,
		PlatformOpenCodeGo: true,
	}
	modes := []string{"", AccountModePayG, AccountModeCoding, AccountModeZen, AccountModeGo, "unknown"}
	for _, platform := range []string{PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo} {
		for _, mode := range modes {
			account := &Account{Platform: platform, Type: AccountTypeAPIKey, Credentials: map[string]any{"account_mode": mode}}
			require.Equal(t, legacy[platform], account.SupportsNativeCNResponses(), platform+"/"+mode)
		}
	}
	require.False(t, (*Account)(nil).SupportsNativeCNResponses())
}

func TestProviderProfile_BaseURLKeysAreNativeProtocols(t *testing.T) {
	t.Parallel()

	for platform, profile := range providerProfiles {
		for mode, endpoints := range profile.Modes {
			require.NotEmpty(t, endpoints.BaseURLs[APIProtocolChatCompletions], "%s/%s must have a chat_completions base", platform, mode)
			for protocol, baseURL := range endpoints.BaseURLs {
				require.True(t, isNativeUpstreamProtocol(protocol), "%s/%s has unknown protocol key %q", platform, mode, protocol)
				require.NotEmpty(t, baseURL, "%s/%s/%s", platform, mode, protocol)
			}
		}
	}
}

func TestProviderProfile_ResponsesPathMatchesLegacy(t *testing.T) {
	t.Parallel()

	legacy := func(platform, base string) string {
		if platform == PlatformDeepseek {
			return buildOpenAIEndpointURL(base, "/responses")
		}
		return buildOpenAIResponsesURL(base)
	}
	bases := []string{
		"https://api.deepseek.com", "https://relay.example.com", "https://relay.example.com/v1",
		"https://opencode.ai/zen/go/v1", "https://open.bigmodel.cn/api/paas/v4", "https://api.moonshot.cn/v1",
	}
	for _, platform := range []string{PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo} {
		for _, base := range bases {
			require.Equal(t, legacy(platform, base), buildOpenAIResponsesURLForPlatform(platform, base), platform+" "+base)
		}
	}
}

func TestProviderProfile_OpenCodeProtocolRulesByMode(t *testing.T) {
	t.Parallel()

	openCode := LookupProviderProfile(PlatformOpenCodeGo)
	require.Equal(t, DefaultOpenCodeGoProtocolRules(), openCode.Endpoints(AccountModeGo).ProtocolRules)
	require.Equal(t, DefaultOpenCodeGoProtocolRules(), openCode.Endpoints("").ProtocolRules)
	require.Equal(t, DefaultOpenCodeZenProtocolRules(), openCode.Endpoints(AccountModeZen).ProtocolRules)
	require.Empty(t, LookupProviderProfile(PlatformKimi).Endpoints(AccountModeCoding).ProtocolRules)
}

func TestProviderProfile_ShareLLMDefaultsAndProtocolRules(t *testing.T) {
	t.Parallel()

	profile := LookupProviderProfile(PlatformShareLLM)
	require.NotNil(t, profile)
	require.Equal(t, ProviderRoutingByModel, profile.Routing)
	require.True(t, profile.ModelCatalog)
	require.Equal(t, DefaultShareLLMBaseURL, profile.DefaultBaseURL(AccountModePayG, APIProtocolChatCompletions))
	require.Equal(t, DefaultShareLLMBaseURL, profile.DefaultBaseURL(AccountModePayG, APIProtocolResponses))
	require.Equal(t, DefaultShareLLMAnthropicBaseURL, profile.DefaultBaseURL(AccountModePayG, APIProtocolAnthropic))
	require.Equal(t, []ProtocolRule{
		{Pattern: "claude-*", Protocol: APIProtocolAnthropic},
		{Pattern: "anthropic/*", Protocol: APIProtocolAnthropic},
	}, profile.Endpoints(AccountModePayG).ProtocolRules)

	account := &Account{
		Platform: PlatformShareLLM,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":      "sk-test",
			"account_mode": AccountModePayG,
			"api_protocol": APIProtocolAdaptive,
		},
	}
	require.True(t, account.IsMultiProtocolAPIKey())
	require.True(t, account.routesByModel())
	require.True(t, account.SupportsNativeCNResponses())
	require.Equal(t, APIProtocolAnthropic, resolveUpstreamProtocol(account, APIProtocolChatCompletions, "claude-sonnet-4-6", nil))
	require.Equal(t, APIProtocolChatCompletions, resolveUpstreamProtocol(account, APIProtocolAnthropic, "deepseek-v4-flash", nil))
	require.Equal(t, APIProtocolChatCompletions, resolveUpstreamProtocol(account, APIProtocolResponses, "gpt-6.1", nil))
}
