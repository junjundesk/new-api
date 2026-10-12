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

type pricingBillingResponse struct {
	Success    bool               `json:"success"`
	Data       []model.Pricing    `json:"data"`
	GroupRatio map[string]float64 `json:"group_ratio"`
	Usable     map[string]string  `json:"usable_group"`
	AutoGroups []string           `json:"auto_groups"`
}

func TestGetPricingBillingRatiosAreUserScoped(t *testing.T) {
	oldDB := model.DB
	oldRedis := common.RedisEnabled
	oldMemory := common.MemoryCacheEnabled
	oldGroups := setting.UserUsableGroups2JSONString()
	oldAuto := setting.AutoGroups2JsonString()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldSpecials := ratio_setting.GroupGroupRatio2JSONString()
	specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	oldSpecialGroups := specialGroups.ReadAll()
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
		common.MemoryCacheEnabled = oldMemory
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAuto))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldSpecials))
		specialGroups.Clear()
		specialGroups.AddAll(oldSpecialGroups)
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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip","hidden"]`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"member":1,"vip":3,"hidden":9}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.5,"vip":1}}`))
	specialGroups.Clear()
	specialGroups.Set("member", map[string]string{"+:vip": "VIP"})
	require.NoError(t, db.Create(&model.User{Id: 5101, Username: "pricing-member", Group: "member", Status: common.UserStatusEnabled}).Error)

	for i, fixture := range []struct {
		name, group, setting string
	}{
		{"scoped-model", "default", ""},
		{"scoped-model", "default", `{"model_group_ratios":{"scoped-model":0}}`},
		{"scoped-model", "vip", `{"model_group_ratios":{"scoped-model":1}}`},
		{"scoped-model", "hidden", `{"model_group_ratios":{"scoped-model":99}}`},
		{"hidden-model", "hidden", ""},
		{"all-model", "all", `{"model_group_ratios":{"all-model":100}}`},
	} {
		channel := &model.Channel{
			Id: i + 1, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled,
			Name: "private-channel-name", Key: "private-api-key", Group: fixture.group,
			Models: fixture.name, Setting: common.GetPointer(fixture.setting),
		}
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			ChannelId: channel.Id, Group: fixture.group, Model: fixture.name, Enabled: true,
		}).Error)
	}
	model.InvalidatePricingCache()

	for _, test := range []struct {
		name   string
		userID int
		want   map[string]model.BillingRatioRange
	}{
		{"member", 5101, map[string]model.BillingRatioRange{
			"default": {Min: 0, Max: 0.5, Source: "mixed"},
			"vip":     {Min: 1, Max: 1, Source: "channel_model"},
		}},
		{"guest-after-member", 0, map[string]model.BillingRatioRange{
			"default": {Min: 0, Max: 2, Source: "mixed"},
		}},
		{"member-after-guest", 5101, map[string]model.BillingRatioRange{
			"default": {Min: 0, Max: 0.5, Source: "mixed"},
			"vip":     {Min: 1, Max: 1, Source: "channel_model"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
			if test.userID != 0 {
				ctx.Set("id", test.userID)
			}
			GetPricing(ctx)
			require.Equal(t, http.StatusOK, recorder.Code)
			var response pricingBillingResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			byName := pricingByModelName(response.Data)
			require.Len(t, byName, 2)
			require.Contains(t, byName, "scoped-model")
			assert.Equal(t, test.want, byName["scoped-model"].BillingRatios)
			assert.NotContains(t, byName["scoped-model"].EnableGroup, "hidden")
			assert.Equal(t, []string{"all"}, byName["all-model"].EnableGroup)
			assert.Empty(t, byName["all-model"].BillingRatios)
			if test.userID == 0 {
				assert.Equal(t, map[string]float64{"default": 2}, response.GroupRatio)
				assert.Equal(t, []string{"default"}, byName["scoped-model"].EnableGroup)
				assert.Equal(t, []string{"default"}, response.AutoGroups)
				assert.NotContains(t, response.Usable, "vip")
			} else {
				assert.Equal(t, map[string]float64{"default": 0.5, "member": 1, "vip": 1}, response.GroupRatio)
				assert.ElementsMatch(t, []string{"default", "vip"}, byName["scoped-model"].EnableGroup)
				assert.Equal(t, []string{"default", "vip"}, response.AutoGroups)
			}
			for _, secret := range []string{"channel_id", "private-channel-name", "private-api-key", "model_group_ratios", "billingCandidates", "hidden", "\"setting\"", "\"settings\""} {
				assert.NotContains(t, recorder.Body.String(), secret)
			}
		})
	}
	for _, price := range model.GetPricing() {
		assert.Nil(t, price.BillingRatios)
	}
}
