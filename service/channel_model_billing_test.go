package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelModelRetryReservationAndSettlement(t *testing.T) {
	truncate(t)
	const userID, tokenID = 8701, 8701
	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "channel-ratio-retry", 10000)
	setting := operation_setting.GetQuotaSetting()
	original := setting.EnableFreeModelPreConsume
	setting.EnableFreeModelPreConsume = false
	t.Cleanup(func() { setting.EnableFreeModelPreConsume = original })
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: userID, TokenId: tokenID, TokenKey: "channel-ratio-retry", ForcePreConsume: true,
		OriginModelName: "priced-model", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData: types.PriceData{ModelRatio: 1, PreConsumeQuotaBeforeGroup: 120,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0, Source: types.GroupRatioSourceChannelModel}},
	}
	require.Nil(t, PrepareBillingForSelectedChannel(ctx, info))
	require.Nil(t, info.Billing)
	assert.True(t, info.PriceData.FreeModel)
	info.PriceData.GroupRatioInfo.GroupRatio = 0.8
	require.Nil(t, PrepareBillingForSelectedChannel(ctx, info))
	require.NotNil(t, info.Billing)
	assert.Equal(t, 96, info.Billing.GetPreConsumedQuota())
	info.PriceData.GroupRatioInfo.GroupRatio = 2
	require.Nil(t, PrepareBillingForSelectedChannel(ctx, info))
	assert.Equal(t, 240, info.Billing.GetPreConsumedQuota())
	info.PriceData.GroupRatioInfo.GroupRatio = 0.5
	require.Nil(t, PrepareBillingForSelectedChannel(ctx, info))
	assert.Equal(t, 60, info.PriceData.QuotaToPreConsume)
	assert.Equal(t, 240, info.Billing.GetPreConsumedQuota())
	require.NoError(t, info.Billing.Settle(60))
	assert.Equal(t, 9940, getUserQuota(t, userID))
	assert.Equal(t, 9940, getTokenRemainQuota(t, tokenID))
	require.NoError(t, info.Billing.Settle(60))
	assert.Equal(t, 9940, getUserQuota(t, userID), "settlement remains idempotent")
}

func TestChannelModelRetryReservationFailureRollsBackFunding(t *testing.T) {
	truncate(t)
	const userID, tokenID = 8702, 8702
	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "channel-ratio-limited-token", 100)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: userID, TokenId: tokenID, TokenKey: "channel-ratio-limited-token", ForcePreConsume: true,
		UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData:   types.PriceData{ModelRatio: 1, PreConsumeQuotaBeforeGroup: 100, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.5}},
	}
	require.Nil(t, PrepareBillingForSelectedChannel(ctx, info))
	info.PriceData.GroupRatioInfo.GroupRatio = 2
	require.NotNil(t, PrepareBillingForSelectedChannel(ctx, info))
	assert.Equal(t, 50, info.Billing.GetPreConsumedQuota())
	assert.Equal(t, 9950, getUserQuota(t, userID))
	assert.Equal(t, 50, getTokenRemainQuota(t, tokenID))
}

func TestChannelModelTieredRetryUsesFrozenExpression(t *testing.T) {
	const expr = `tier("base", p * 2)`
	billing := &recordingBillingSettler{}
	info := &relaycommon.RelayInfo{
		Billing:   billing,
		PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.8, Source: types.GroupRatioSourceChannelModel}},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr", ExprString: expr, ExprHash: billingexpr.ExprHashString(expr),
			GroupRatio: 0, EstimatedQuotaBeforeGroup: 1000, QuotaPerUnit: 500000,
		},
	}
	require.Nil(t, PrepareBillingForSelectedChannel(nil, info))
	assert.Equal(t, 800, billing.GetPreConsumedQuota())
	assert.Equal(t, 0.8, info.TieredBillingSnapshot.GroupRatio)
	ok, quota, result := TryTieredSettle(info, billingexpr.TokenParams{P: 1000})
	require.True(t, ok)
	require.NotNil(t, result)
	assert.Equal(t, 800, quota)
	assert.Equal(t, expr, info.TieredBillingSnapshot.ExprString)
	info.PriceData.GroupRatioInfo.GroupRatio = 0
	require.Nil(t, PrepareBillingForSelectedChannel(nil, info))
	ok, quota, _ = TryTieredSettle(info, billingexpr.TokenParams{P: 1000})
	require.True(t, ok)
	assert.Zero(t, quota)
	assert.Equal(t, 800, billing.GetPreConsumedQuota(), "free retry refunds only at settlement")
}

func TestChannelModelTaskSnapshotAndLegacyFallback(t *testing.T) {
	oldModels, oldGroups, oldSpecial := ratio_setting.ModelRatio2JSONString(), ratio_setting.GroupRatio2JSONString(), ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldModels))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldSpecial))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"test-model":5}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":3}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))
	for _, tc := range []struct {
		name      string
		version   int
		ratio     float64
		wantQuota int
	}{
		{"frozen after config change and reload", 1, 0.8, 2400},
		{"explicit zero remains free", 1, 0, 0},
		{"legacy uses current globals", 0, 0.8, 22500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			const userID, tokenID, channelID = 8703, 8703, 8703
			seedUser(t, userID, 100000)
			seedToken(t, tokenID, userID, "task-snapshot-token", 100000)
			seedChannel(t, channelID)
			seedChargedAccounting(t, userID, channelID, tokenID, 1000, 1)
			task := makeTask(userID, channelID, 1000, tokenID, BillingSourceWallet, 0)
			task.PrivateData.BillingContext = &model.TaskBillingContext{
				Version: tc.version, ModelRatio: 2, GroupRatio: tc.ratio, OriginModelName: "test-model",
				GroupRatioSource: types.GroupRatioSourceChannelModel, ChannelModelRatio: common.GetPointer(tc.ratio),
				OtherRatios: map[string]float64{"duration": 1.5},
			}
			require.NoError(t, model.DB.Create(task).Error)
			var reloaded model.Task
			require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
			RecalculateTaskQuotaByTokens(context.Background(), &reloaded, 1000)
			assert.Equal(t, tc.wantQuota, reloaded.Quota)
			assert.Equal(t, 101000-tc.wantQuota, getUserQuota(t, userID))
			assert.Equal(t, 101000-tc.wantQuota, getTokenRemainQuota(t, tokenID))
			other := taskBillingOther(&reloaded)
			assert.Equal(t, tc.ratio, other["group_ratio"])
			assert.Equal(t, types.GroupRatioSourceChannelModel, other["billing_ratio_source"])
			assert.NotContains(t, other, "billing_channel_id")
			require.IsType(t, map[string]interface{}{}, other["admin_info"])
			assert.Equal(t, channelID, other["admin_info"].(map[string]interface{})["billing_channel_id"])
		})
	}
}

func TestChannelModelTieredErrorFallbackUsesFinalAttempt(t *testing.T) {
	for _, tc := range []struct {
		ratio          float64
		estimate, want int
	}{{0, 0, 0}, {0.2, 200, 200}} {
		info := &relaycommon.RelayInfo{FinalPreConsumedQuota: 1000,
			TieredBillingSnapshot: &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: "invalid +-+ expr", GroupRatio: tc.ratio, EstimatedQuotaAfterGroup: tc.estimate}}
		ok, quota, _ := TryTieredSettle(info, billingexpr.TokenParams{P: 100})
		require.True(t, ok)
		assert.Equal(t, tc.want, quota)
	}
}

func TestChannelModelRealtimeTrustedLimits(t *testing.T) {
	for _, tc := range []struct {
		name          string
		wallet, token int
	}{{"wallet exhausted", 100, 1000}, {"token exhausted", 1000, 100}} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			const userID, tokenID = 8705, 8705
			seedUser(t, userID, tc.wallet)
			seedToken(t, tokenID, userID, "trusted-realtime-token", tc.token)
			info := &relaycommon.RelayInfo{UserId: userID, TokenId: tokenID, TokenKey: "trusted-realtime-token"}
			session := &BillingSession{relayInfo: info, funding: &WalletFunding{userId: userID}, trusted: true}
			info.Billing = session
			require.NoError(t, session.ReserveRealtime(80))
			assert.Equal(t, 80, session.GetPreConsumedQuota(), "trusted connection must still reserve usage")
			require.Error(t, session.ReserveRealtime(120))
			assert.Equal(t, tc.wallet-80, getUserQuota(t, userID))
			assert.Equal(t, tc.token-80, getTokenRemainQuota(t, tokenID))
			require.NoError(t, session.Settle(80))
			assert.Equal(t, tc.wallet-80, getUserQuota(t, userID))
		})
	}
}

func TestChannelModelRealtimeCumulativeReservationSettlesOnce(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 8704, 8704, 8704
	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "realtime-snapshot-token", 10000)
	seedChannel(t, channelID)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{
		UserId: userID, TokenId: tokenID, TokenKey: "realtime-snapshot-token", ForcePreConsume: true,
		OriginModelName: "gpt-realtime-test", UsingGroup: "default", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: channelID},
		PriceData: types.PriceData{BillingModelName: "gpt-realtime-test", ModelRatio: 1, CompletionRatio: 2,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.8, Source: types.GroupRatioSourceChannelModel}},
	}
	usage := &dto.RealtimeUsage{InputTokens: 100, TotalTokens: 100}
	usage.InputTokenDetails.TextTokens = 100
	require.NoError(t, PreWssConsumeQuota(ctx, info, usage))
	assert.Equal(t, 80, info.Billing.GetPreConsumedQuota())
	usage.InputTokens, usage.TotalTokens, usage.InputTokenDetails.TextTokens = 300, 300, 300
	require.NoError(t, PreWssConsumeQuota(ctx, info, usage))
	assert.Equal(t, 240, info.Billing.GetPreConsumedQuota())
	PostWssConsumeQuota(ctx, info, info.OriginModelName, usage, "")
	assert.Equal(t, 9760, getUserQuota(t, userID))
	assert.Equal(t, 9760, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(240), getChannelUsedQuota(t, channelID))
	other := GenerateTextOtherInfo(ctx, info, 1, 0.8, 2, 0, 0, 0, 0.5)
	assert.NotContains(t, other, "user_group_ratio")
	assert.Equal(t, 0.8, other["channel_model_ratio"])
	assert.Equal(t, types.GroupRatioSourceChannelModel, other["billing_ratio_source"])
}
