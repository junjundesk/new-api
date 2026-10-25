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

type userGroupTagResponse struct {
	Success bool `json:"success"`
	Data    map[string]struct {
		Ratio                 float64 `json:"ratio"`
		Desc                  string  `json:"desc"`
		HasModelSpecificRatio bool    `json:"has_model_specific_ratio"`
	} `json:"data"`
}

func TestGetUserGroupsMarksGroupsWithModelSpecificRatio(t *testing.T) {
	oldDB := model.DB
	oldRedis := common.RedisEnabled
	oldMemory := common.MemoryCacheEnabled
	oldUsableGroups := setting.UserUsableGroups2JSONString()
	oldGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
		common.MemoryCacheEnabled = oldMemory
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatios))
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
		&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{},
		&model.Vendor{}, &model.UserChannelChain{}, &model.ActivityGrant{},
	))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","empty":"普通分组"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"empty":1}`))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "group-tag-user", Group: "default"}).Error)

	exclusiveSetting := `{"model_group_ratios":{"exclusive-model":0.8}}`
	exclusiveChannel := &model.Channel{
		Type: constant.ChannelTypeOpenAI, Key: "test-key", Name: "exclusive-channel",
		Status: common.ChannelStatusEnabled, Models: "exclusive-model", Group: "default",
		Setting: &exclusiveSetting,
	}
	require.NoError(t, db.Create(exclusiveChannel).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: exclusiveChannel.Id, Group: "default", Model: "exclusive-model", Enabled: true}).Error)

	plainChannel := &model.Channel{
		Type: constant.ChannelTypeOpenAI, Key: "test-key", Name: "plain-channel",
		Status: common.ChannelStatusEnabled, Models: "plain-model", Group: "empty",
	}
	require.NoError(t, db.Create(plainChannel).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: plainChannel.Id, Group: "empty", Model: "plain-model", Enabled: true}).Error)

	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
	ctx.Set("id", 1)
	GetUserGroups(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response userGroupTagResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Contains(t, response.Data, "default")
	require.Contains(t, response.Data, "empty")
	assert.True(t, response.Data["default"].HasModelSpecificRatio)
	assert.False(t, response.Data["empty"].HasModelSpecificRatio)
}
