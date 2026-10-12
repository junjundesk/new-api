package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelModelRatioPricingAndMapping(t *testing.T) {
	savedConfig := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		savedConfig[key] = value
		return nil
	}))
	priceSettings := []struct {
		saved  string
		update func(string) error
		value  string
	}{
		{ratio_setting.ModelPrice2JSONString(), ratio_setting.UpdateModelPriceByJSONString, `{"channel-ratio-fixed":0.04,"channel-ratio-free":0,"upstream-alias":99,"client-alias":99}`},
		{ratio_setting.ModelRatio2JSONString(), ratio_setting.UpdateModelRatioByJSONString, `{"channel-ratio-token":2,"channel-ratio-expression":400}`},
		{ratio_setting.CompletionRatio2JSONString(), ratio_setting.UpdateCompletionRatioByJSONString, `{"channel-ratio-token":3,"channel-ratio-expression":100}`},
		{ratio_setting.CacheRatio2JSONString(), ratio_setting.UpdateCacheRatioByJSONString, `{}`},
		{ratio_setting.CreateCacheRatio2JSONString(), ratio_setting.UpdateCreateCacheRatioByJSONString, `{}`},
		{ratio_setting.ImageRatio2JSONString(), ratio_setting.UpdateImageRatioByJSONString, `{}`},
		{ratio_setting.AudioRatio2JSONString(), ratio_setting.UpdateAudioRatioByJSONString, `{}`},
		{ratio_setting.AudioCompletionRatio2JSONString(), ratio_setting.UpdateAudioCompletionRatioByJSONString, `{}`},
	}
	savedQuotaPerUnit, savedPreConsume := common.QuotaPerUnit, common.PreConsumedQuota
	savedGinMode := gin.Mode()
	t.Cleanup(func() {
		assert.NoError(t, config.GlobalConfig.LoadFromDB(savedConfig))
		for _, setting := range priceSettings {
			assert.NoError(t, setting.update(setting.saved))
		}
		common.QuotaPerUnit, common.PreConsumedQuota = savedQuotaPerUnit, savedPreConsume
		gin.SetMode(savedGinMode)
	})
	gin.SetMode(gin.TestMode)
	common.QuotaPerUnit, common.PreConsumedQuota = 500000, 500
	for _, setting := range priceSettings {
		require.NoError(t, setting.update(setting.value))
	}
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"group_ratio_setting.group_ratio":             `{"initial":3,"selected":1.7}`,
		"group_ratio_setting.group_group_ratio":       `{"member":{"initial":0.2,"selected":0.6}}`,
		"quota_setting.enable_free_model_pre_consume": "false",
		"billing_setting.billing_mode":                `{"channel-ratio-expression":"tiered_expr"}`,
		"billing_setting.billing_expr":                `{"channel-ratio-expression":"tier(\"base\", p * 3 + c * 15)"}`,
	}))

	discount := 0.8
	// Expected quotas are fixed billing examples, not calls to pricing/conversion
	// helpers: 1100 tokens at ratio 2 = 2200; $0.04 x 3 = 60000 quota;
	// (1000 x $3 + 100 x $15)/1M = $0.0045 = 2250 quota before group.
	tests := []struct {
		name            string
		model           string
		override        *float64
		wantGroupRatio  float64
		wantSource      string
		wantQuota       int
		wantBeforeGroup float64
		wantModelPrice  float64
		wantModelRatio  float64
		wantCompletion  float64
		wantUsePrice    bool
		wantFree        bool
		tiered          bool
	}{
		{
			name: "missing billing key ignores client and upstream aliases", model: "channel-ratio-token",
			wantGroupRatio: 0.6, wantSource: hosttypes.GroupRatioSourceUserGroup,
			wantQuota: 1320, wantBeforeGroup: 2200, wantModelPrice: -1, wantModelRatio: 2, wantCompletion: 3,
		},
		{
			name: "explicit zero makes paid token model free", model: "channel-ratio-token", override: new(float64),
			wantGroupRatio: 0, wantSource: hosttypes.GroupRatioSourceChannelModel,
			wantQuota: 0, wantBeforeGroup: 2200, wantModelPrice: -1, wantModelRatio: 2, wantCompletion: 3, wantFree: true,
		},
		{
			name: "discount preserves fixed model price and request count", model: "channel-ratio-fixed", override: &discount,
			wantGroupRatio: 0.8, wantSource: hosttypes.GroupRatioSourceChannelModel,
			wantQuota: 48000, wantBeforeGroup: 60000, wantModelPrice: 0.04, wantUsePrice: true,
		},
		{
			name: "positive override does not charge a zero base price", model: "channel-ratio-free", override: &discount,
			wantGroupRatio: 0.8, wantSource: hosttypes.GroupRatioSourceChannelModel,
			wantQuota: 0, wantBeforeGroup: 0, wantModelPrice: 0, wantUsePrice: true, wantFree: true,
		},
		{
			name: "expression keeps base price and token input", model: "channel-ratio-expression", override: &discount,
			wantGroupRatio: 0.8, wantSource: hosttypes.GroupRatioSourceChannelModel,
			wantQuota: 1800, wantBeforeGroup: 2250, tiered: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			// This is the helper boundary after the token middleware has rewritten
			// client-alias to the billing model, before channel mapping runs.
			common.SetContextKey(ctx, constant.ContextKeyOriginalModel, tt.model)
			common.SetContextKey(ctx, constant.ContextKeyUserGroup, "member")
			common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "initial")
			ctx.Set("auto_group", "selected")
			zero := 0.0
			overrides := map[string]*float64{"client-alias": &zero, "upstream-alias": &zero}
			if tt.override != nil {
				overrides[tt.model] = tt.override
			}
			common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{ModelGroupRatios: overrides})
			request := &dto.GeneralOpenAIRequest{Model: tt.model}
			info := relaycommon.GenRelayInfoOpenAI(ctx, request)
			require.Nil(t, info.ChannelMeta)
			input := &billingexpr.RequestInput{Body: []byte(`{"stream":true}`)}
			info.BillingRequestInput = input

			group := HandleGroupRatio(ctx, info)
			assert.Equal(t, "selected", info.UsingGroup)
			assert.Equal(t, tt.wantGroupRatio, group.GroupRatio)
			assert.Equal(t, tt.wantSource, group.Source)

			staleRatio := 99.0
			info.ChannelMeta = &relaycommon.ChannelMeta{
				UpstreamModelName: tt.model,
				ChannelSetting:    dto.ChannelSettings{ModelGroupRatios: map[string]*float64{tt.model: &staleRatio}},
			}
			mapping, err := common.Marshal(map[string]string{tt.model: "upstream-alias"})
			require.NoError(t, err)
			ctx.Set("model_mapping", string(mapping))
			require.NoError(t, ModelMappedHelper(ctx, info, request))
			assert.Equal(t, tt.model, info.OriginModelName)
			assert.Equal(t, "upstream-alias", info.UpstreamModelName)
			assert.Equal(t, "upstream-alias", request.Model)
			assert.Equal(t, group, HandleGroupRatio(ctx, info), "selected context must win over stale ChannelMeta and upstream alias")

			price, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{
				MaxTokens: 100, BillingRatios: map[string]float64{"n": 3},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.wantQuota, price.QuotaToPreConsume)
			assert.Equal(t, tt.wantBeforeGroup, price.PreConsumeQuotaBeforeGroup)
			assert.Equal(t, tt.wantModelPrice, price.ModelPrice)
			assert.Equal(t, tt.wantModelRatio, price.ModelRatio)
			assert.Equal(t, tt.wantCompletion, price.CompletionRatio)
			assert.Equal(t, tt.wantUsePrice, price.UsePrice)
			assert.Equal(t, tt.wantUsePrice, price.HasOtherRatio("n"))
			assert.Equal(t, tt.wantFree, price.FreeModel)
			assert.Equal(t, group, price.GroupRatioInfo)
			assert.Equal(t, tt.model, price.BillingModelName)
			assert.Equal(t, price, info.PriceData)
			assert.Equal(t, tt.model, info.OriginModelName)
			assert.Equal(t, input.Body, info.BillingRequestInput.Body)

			if tt.tiered {
				require.NotNil(t, info.TieredBillingSnapshot)
				snapshot := info.TieredBillingSnapshot
				assert.Equal(t, billing_setting.BillingModeTieredExpr, snapshot.BillingMode)
				assert.Equal(t, "channel-ratio-expression", snapshot.ModelName)
				assert.Equal(t, `tier("base", p * 3 + c * 15)`, snapshot.ExprString)
				assert.Equal(t, "base", snapshot.EstimatedTier)
				assert.Equal(t, 1000, snapshot.EstimatedPromptTokens)
				assert.Equal(t, 100, snapshot.EstimatedCompletionTokens)
				assert.Equal(t, 2250.0, snapshot.EstimatedQuotaBeforeGroup)
				assert.Equal(t, 1800, snapshot.EstimatedQuotaAfterGroup)
				assert.Equal(t, 0.8, snapshot.GroupRatio)
			} else {
				assert.Nil(t, info.TieredBillingSnapshot)
			}

			if tt.wantGroupRatio == 0 {
				quotaSetting := operation_setting.GetQuotaSetting()
				old := quotaSetting.EnableFreeModelPreConsume
				t.Cleanup(func() { quotaSetting.EnableFreeModelPreConsume = old })
				quotaSetting.EnableFreeModelPreConsume = true
				withPreConsume, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 100})
				require.NoError(t, err)
				assert.False(t, withPreConsume.FreeModel, "free-model bypass is controlled by the quota setting")
				assert.Zero(t, withPreConsume.QuotaToPreConsume, "forcing pre-consume must not turn explicit zero into a charge")
			}
		})
	}
}
