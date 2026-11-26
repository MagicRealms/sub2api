//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMiMoASRUsesChatCompletionsAndBillsAudioHours(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"mimo-v2.5-asr","messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"data:audio/wav;base64,AA=="}}]}],"max_completion_tokens":1024}`)
	price := 0.50
	billing := NewBillingService(nil, nil)
	key := &APIKey{Group: &Group{ID: 36, Platform: PlatformOpenAI, ModelPricing: []ChannelModelPricing{
		{Platform: PlatformOpenAI, Models: []string{mimoASRModel}, BillingMode: BillingModePerRequest,
			Intervals: []PricingInterval{{TierLabel: "stt", PerRequestPrice: &price}}},
	}}}
	account := rawChatCompletionsTestAccount()
	account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: true}

	for _, tc := range []struct {
		name, usage string
		status      int
	}{
		{name: "valid duration", usage: `{"prompt_tokens":350,"completion_tokens":20,"seconds":9}`, status: http.StatusOK},
		{name: "missing duration", usage: `{"prompt_tokens":350,"completion_tokens":20}`, status: http.StatusBadGateway},
		{name: "invalid duration", usage: `{"seconds":-9}`, status: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Set("api_key", key)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"hello"}}],"usage":` + tc.usage + `}`)),
			}}
			svc := &OpenAIGatewayService{
				cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream,
				billingService: billing, resolver: NewModelPricingResolver(nil, billing),
			}
			result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
			require.Equal(t, tc.status, rec.Code)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
			require.JSONEq(t, string(body), string(upstream.lastBody))
			if tc.status != http.StatusOK {
				require.Error(t, err)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 9.0/3600, result.AudioUsage.DurationOrUnits)
			cost, err := svc.calculateOpenAIRecordUsageCostForGroup(context.Background(), result, key,
				[]string{mimoASRModel}, 0.2, 1, 1, 0.2, UsageTokens{InputTokens: 350, OutputTokens: 20}, "", nil, false, time.Now())
			require.NoError(t, err)
			require.InDelta(t, 0.00125, cost.TotalCost, 1e-9)
			require.InDelta(t, 0.00025, cost.ActualCost, 1e-9)
		})
	}
}

func TestMiMoTTSUsesDedicatedAccountAndExplicitFreePrice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"mimo-v2.5-tts","messages":[{"role":"assistant","content":"hello"}],"audio":{"format":"wav","voice":"mimo_default"}}`)
	free := 0.0
	group := &Group{ID: 36, Platform: PlatformMiMo}
	key := &APIKey{Group: group}
	billing := NewBillingService(nil, nil)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"audio":{"data":"UklGRg=="}}}],"usage":{"prompt_tokens":4,"completion_tokens":3}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream,
		billingService: billing, resolver: NewModelPricingResolver(nil, billing),
	}
	request := InflightEstimateRequest{Model: mimoTTSModel, Kind: InflightEstimateToken}
	require.ErrorIs(t, svc.ValidateRequestPricing(context.Background(), key, request), ErrModelPricingUnavailable)
	group.ModelPricing = []ChannelModelPricing{
		{Platform: PlatformMiMo, Models: []string{mimoTTSModel}, BillingMode: BillingModePerRequest, PerRequestPrice: &free},
	}
	require.NoError(t, svc.ValidateRequestPricing(context.Background(), key, request))

	account := rawChatCompletionsTestAccount()
	account.Platform = PlatformMiMo
	account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: true}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("api_key", key)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.JSONEq(t, string(body), string(upstream.lastBody))
	require.Contains(t, rec.Body.String(), `"data":"UklGRg=="`)
	cost, err := svc.calculateOpenAIRecordUsageCostForGroup(context.Background(), result, key, []string{mimoTTSModel},
		0.2, 1, 1, 0.2, result.Usage.billingUsageTokens(), "", nil, false, time.Now())
	require.NoError(t, err)
	require.Zero(t, cost.ActualCost)
}

func TestMiMoDomesticPricingOverridesRemoteUSD(t *testing.T) {
	catalog := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"mimo-v2.6-flash": {PricingCurrency: "USD", InputCostPerToken: 0.14e-6, OutputCostPerToken: 0.28e-6, CacheReadInputTokenCost: 0.0028e-6},
		"mimo-v2.6-pro":   {PricingCurrency: "USD", InputCostPerToken: 0.435e-6, OutputCostPerToken: 0.87e-6, CacheReadInputTokenCost: 0.0036e-6},
	}}
	billing := NewBillingService(nil, catalog)
	ctx := context.Background()
	plaza := &ModelPlazaService{billingService: billing}
	memo := make(map[string]*PlazaOfficialPricing)
	for _, tc := range []struct {
		model string
		want  float64
	}{
		{model: "self-dploy/MiMo-V2.6-Flash", want: 0.00302},
		{model: "mimo-v2.6-pro", want: 0.009025},
	} {
		t.Run(tc.model, func(t *testing.T) {
			cost, err := billing.CalculateCost(tc.model, UsageTokens{InputTokens: 1000, OutputTokens: 1000, CacheReadTokens: 1000}, 0.15)
			require.NoError(t, err)
			require.InDelta(t, tc.want, cost.TotalCost, 1e-12)
			require.InDelta(t, tc.want*0.15, cost.ActualCost, 1e-12)
			require.Equal(t, "CNY", plaza.lookupOfficialPricing(ctx, tc.model, memo).Currency)
		})
	}
	for _, tc := range []struct {
		model, unit string
		price       float64
	}{
		{model: mimoASRModel, unit: "hour", price: 0.50},
		{model: mimoTTSModel, unit: "request", price: 0},
	} {
		p := plaza.lookupOfficialPricing(ctx, tc.model, memo)
		require.Equal(t, "CNY", p.Currency)
		require.Equal(t, tc.unit, p.Unit)
		require.Equal(t, tc.price, *p.UnitPrice)
	}
}
