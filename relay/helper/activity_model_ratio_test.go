package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 「指定模型」类型的活动授权必须经由 HandleGroupRatio 生效：命中的模型使用活动倍率
// （可无视分组定价），未命中的模型回落到活动授权继承的分组倍率；活动授权存在时
// 渠道模型专属倍率不参与。
func TestHandleGroupRatioAppliesActivityModelOverride(t *testing.T) {
	savedGroups := ratio_setting.GroupRatio2JSONString()
	savedSpecialGroups := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5,"free-group":0}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))

	newRequestInfo := func(ctx *gin.Context, modelName string) *relaycommon.RelayInfo {
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 7)
		common.SetContextKey(ctx, constant.ContextKeyOriginalModel, modelName)
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, "member")
		common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "activity-group")
		return relaycommon.GenRelayInfoOpenAI(ctx, &dto.GeneralOpenAIRequest{Model: modelName})
	}

	channelRatio := 0.05
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		ModelGroupRatios: map[string]*float64{"gpt-5": &channelRatio},
	})
	// 等价于鉴权阶段已经解析过该用户的活动授权（一次请求只查一次）。
	common.SetContextKey(ctx, constant.ContextKeyActivityGrants, []service.ActivityView{{
		GrantId:             1,
		GroupName:           "activity-group",
		RatioMode:           model.ActivityRatioModeInherit,
		ModelRatioOverrides: map[string]float64{"gpt-5": 0},
	}})

	listed := HandleGroupRatio(ctx, newRequestInfo(ctx, "gpt-5"))
	assert.Equal(t, float64(0), listed.GroupRatio, "列表内模型按活动倍率计费")
	assert.Equal(t, hosttypes.GroupRatioSourceActivity, listed.Source)
	assert.Nil(t, listed.ChannelModelRatio)
	require.NotNil(t, listed.ActivityOriginalRatio)
	assert.Equal(t, 0.05, *listed.ActivityOriginalRatio, "消费日志保留优惠前的渠道价格")

	unlisted := HandleGroupRatio(ctx, newRequestInfo(ctx, "o3"))
	assert.Equal(t, 0.6, unlisted.GroupRatio, "未列出的模型按活动授权继承的分组倍率")
	assert.Equal(t, 0.6, unlisted.GroupSpecialRatio)
	assert.True(t, unlisted.HasSpecialRatio)
	assert.Equal(t, hosttypes.GroupRatioSourceActivity, unlisted.Source)
	assert.Nil(t, unlisted.ChannelModelRatio)
	assert.Nil(t, unlisted.ActivityOriginalRatio)
}

// 活动码授权最高优先：非 0 活动倍率、继承免费分组、继承付费分组都不让渠道专属倍率参与。
func TestHandleGroupRatioActivityOutranksChannelModelRatio(t *testing.T) {
	savedGroups := ratio_setting.GroupRatio2JSONString()
	savedSpecialGroups := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5,"free-group":0}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))

	channelRatio := 0.05
	newContext := func(userGroup, usingGroup string, view service.ActivityView) (*gin.Context, *relaycommon.RelayInfo) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 7)
		common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-5")
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, userGroup)
		common.SetContextKey(ctx, constant.ContextKeyUsingGroup, usingGroup)
		common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{
			ModelGroupRatios: map[string]*float64{"gpt-5": &channelRatio},
		})
		common.SetContextKey(ctx, constant.ContextKeyActivityGrants, []service.ActivityView{view})
		return ctx, relaycommon.GenRelayInfoOpenAI(ctx, &dto.GeneralOpenAIRequest{Model: "gpt-5"})
	}

	t.Run("a non-zero activity override wins", func(t *testing.T) {
		ctx, info := newContext("member", "activity-group", service.ActivityView{
			GroupName: "activity-group", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0.2,
		})
		got := HandleGroupRatio(ctx, info)
		assert.Equal(t, 0.2, got.GroupRatio)
		assert.Equal(t, hosttypes.GroupRatioSourceActivity, got.Source)
		assert.Nil(t, got.ChannelModelRatio)
		require.NotNil(t, got.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *got.ActivityOriginalRatio)
	})

	t.Run("an inherit grant on a free group wins", func(t *testing.T) {
		ctx, info := newContext("other", "free-group", service.ActivityView{
			GroupName: "free-group", RatioMode: model.ActivityRatioModeInherit,
		})
		got := HandleGroupRatio(ctx, info)
		assert.Equal(t, float64(0), got.GroupRatio)
		assert.Equal(t, hosttypes.GroupRatioSourceActivity, got.Source)
		assert.Nil(t, got.ChannelModelRatio)
		require.NotNil(t, got.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *got.ActivityOriginalRatio)
	})

	t.Run("an inherit grant on a paid group uses the group ratio", func(t *testing.T) {
		ctx, info := newContext("other", "activity-group", service.ActivityView{
			GroupName: "activity-group", RatioMode: model.ActivityRatioModeInherit,
		})
		got := HandleGroupRatio(ctx, info)
		assert.Equal(t, 1.5, got.GroupRatio)
		assert.Equal(t, hosttypes.GroupRatioSourceActivity, got.Source)
		assert.Nil(t, got.ChannelModelRatio)
		require.NotNil(t, got.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *got.ActivityOriginalRatio)
	})
}

// 完全免费的活动授权必须跳过预扣费；即使运营设置开启了免费模型预扣费也不查余额。
func TestActivityFreePerCallPriceSkipsPreConsume(t *testing.T) {
	savedGroups := ratio_setting.GroupRatio2JSONString()
	savedSpecialGroups := ratio_setting.GroupGroupRatio2JSONString()
	savedModelPrices := ratio_setting.ModelPrice2JSONString()
	savedFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrices))
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = savedFreePreConsume
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"price-model":0.04}`))
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = true

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserId, 7)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "price-model")
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "member")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "activity-group")
	common.SetContextKey(ctx, constant.ContextKeyActivityGrants, []service.ActivityView{{
		GroupName: "activity-group", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0,
	}})

	info := relaycommon.GenRelayInfoOpenAI(ctx, &dto.GeneralOpenAIRequest{Model: "price-model"})
	price, err := ModelPriceHelperPerCall(ctx, info)
	require.NoError(t, err)
	assert.True(t, price.FreeModel)
	assert.Zero(t, price.Quota)
	assert.Equal(t, float64(0), price.GroupRatioInfo.GroupRatio)
}

func TestActivityFreeSkipsPreConsumeRegardlessOfSetting(t *testing.T) {
	savedGroups := ratio_setting.GroupRatio2JSONString()
	savedSpecialGroups := ratio_setting.GroupGroupRatio2JSONString()
	savedModelRatios := ratio_setting.ModelRatio2JSONString()
	savedFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedModelRatios))
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = savedFreePreConsume
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-5":2}`))
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = true

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserId, 7)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-5")
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "member")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "activity-group")
	channelRatio := 0.05
	common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		ModelGroupRatios: map[string]*float64{"gpt-5": &channelRatio},
	})
	common.SetContextKey(ctx, constant.ContextKeyActivityGrants, []service.ActivityView{{
		GroupName: "activity-group", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0,
	}})

	info := relaycommon.GenRelayInfoOpenAI(ctx, &dto.GeneralOpenAIRequest{Model: "gpt-5"})
	price, err := ModelPriceHelper(ctx, info, 1000, &relaytypes.TokenCountMeta{MaxTokens: 100})
	require.NoError(t, err)
	assert.True(t, price.FreeModel)
	assert.Zero(t, price.QuotaToPreConsume)
	assert.Equal(t, float64(0), price.GroupRatioInfo.GroupRatio)
	assert.Equal(t, hosttypes.GroupRatioSourceActivity, price.GroupRatioInfo.Source)
}
