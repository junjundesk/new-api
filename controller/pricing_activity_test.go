package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 活动码授权只改变实际扣费，模型广场必须继续展示标准分组价格与原有可选分组。
func TestGetPricingIgnoresActivityGrants(t *testing.T) {
	oldDB := model.DB
	oldRedis := common.RedisEnabled
	oldMemory := common.MemoryCacheEnabled
	oldUsable := setting.UserUsableGroups2JSONString()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldSpecials := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
		common.MemoryCacheEnabled = oldMemory
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldSpecials))
		model.InvalidatePricingCache()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	model.DB = db
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.ActivityGrant{},
	))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"promo":3}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.5}}`))
	require.NoError(t, db.Create(&model.User{
		Id: 5201, Username: "pricing-activity", Group: "member", Status: common.UserStatusEnabled,
	}).Error)

	for _, fixture := range []struct {
		group, modelName string
	}{
		{"default", "plaza-model"},
		{"promo", "promo-model"},
	} {
		channel := &model.Channel{
			Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled,
			Name: "activity-pricing-channel", Key: "activity-pricing-key", Group: fixture.group, Models: fixture.modelName,
		}
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			ChannelId: channel.Id, Group: fixture.group, Model: fixture.modelName, Enabled: true,
		}).Error)
	}
	for _, grant := range []model.ActivityGrant{
		{UserId: 5201, CodeId: 1, Code: "free4u", CodeName: "Free", GroupName: "default", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0, Status: model.ActivityGrantStatusActive},
		{UserId: 5201, CodeId: 2, Code: "promo", CodeName: "Promo", GroupName: "promo", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0, Status: model.ActivityGrantStatusActive},
	} {
		require.NoError(t, db.Create(&grant).Error)
	}
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 5201)
	GetPricing(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response pricingBillingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)

	byName := pricingByModelName(response.Data)
	require.Contains(t, byName, "plaza-model")
	assert.Equal(t, map[string]model.BillingRatioRange{
		"default": {Min: 0.5, Max: 0.5, Source: "user_group"},
	}, byName["plaza-model"].BillingRatios)
	assert.Equal(t, map[string]float64{"default": 0.5}, response.GroupRatio)
	assert.Contains(t, response.Usable, "default")
	assert.NotContains(t, response.Usable, "promo")
	assert.NotContains(t, byName, "promo-model")
}
