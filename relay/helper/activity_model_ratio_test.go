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
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 「指定模型」类型的活动授权必须经由 HandleGroupRatio 生效：命中的模型使用活动倍率
// （可无视分组定价），未命中的模型继续走用户分组矩阵。
func TestHandleGroupRatioAppliesActivityModelOverride(t *testing.T) {
	savedGroups := ratio_setting.GroupRatio2JSONString()
	savedSpecialGroups := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))

	newRequestInfo := func(ctx *gin.Context, modelName string) *relaycommon.RelayInfo {
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 7)
		common.SetContextKey(ctx, constant.ContextKeyOriginalModel, modelName)
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, "member")
		common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "activity-group")
		return relaycommon.GenRelayInfoOpenAI(ctx, &dto.GeneralOpenAIRequest{Model: modelName})
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
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
	require.NotNil(t, listed.ActivityOriginalRatio)
	assert.Equal(t, 0.6, *listed.ActivityOriginalRatio, "消费日志保留优惠前价格")

	unlisted := HandleGroupRatio(ctx, newRequestInfo(ctx, "o3"))
	assert.Equal(t, 0.6, unlisted.GroupRatio, "未列出的模型仍走用户分组矩阵")
	assert.Equal(t, hosttypes.GroupRatioSourceUserGroup, unlisted.Source)
	assert.Nil(t, unlisted.ActivityOriginalRatio)
}
