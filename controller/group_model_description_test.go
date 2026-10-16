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

type groupModelDescriptionsPricingResponse struct {
	Success                bool                         `json:"success"`
	Usable                 map[string]string            `json:"usable_group"`
	GroupModelDescriptions map[string]map[string]string `json:"group_model_descriptions"`
}

type groupModelMembersResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    []groupModelMember `json:"data"`
}

// setupGroupModelDescriptionFixture builds three groups (default, vip, hidden)
// where only the first two are user-visible, and configures plaza descriptions
// for all of them so visibility scoping can be asserted.
func setupGroupModelDescriptionFixture(t *testing.T) {
	t.Helper()

	oldDB := model.DB
	oldRedis := common.RedisEnabled
	oldMemory := common.MemoryCacheEnabled
	oldGroups := setting.UserUsableGroups2JSONString()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	descriptions := ratio_setting.GetGroupRatioSetting().GroupModelDescriptions
	require.NotNil(t, descriptions)
	oldDescriptions := descriptions.ReadAll()
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
		common.MemoryCacheEnabled = oldMemory
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		descriptions.Clear()
		descriptions.AddAll(oldDescriptions)
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
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"hidden":1}`))

	for i, fixture := range []struct {
		model string
		group string
	}{
		{"shared-model", "default"},
		{"vip-only-model", "vip"},
		{"hidden-model", "hidden"},
	} {
		channel := &model.Channel{
			Id: i + 1, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled,
			Name: "group-members-channel-" + fixture.group, Key: "test-channel-key",
			Group: fixture.group, Models: fixture.model,
		}
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{
			ChannelId: channel.Id, Group: fixture.group, Model: fixture.model, Enabled: true,
		}).Error)
	}
	require.NoError(t, db.Create(&model.Model{
		ModelName: "vip-only-model", Description: "全局描述", Status: 1,
	}).Error)
	model.InvalidatePricingCache()

	descriptions.Set("default", map[string]string{"shared-model": "默认分组描述"})
	descriptions.Set("vip", map[string]string{"vip-only-model": "VIP 分组描述"})
	descriptions.Set("hidden", map[string]string{"hidden-model": "隐藏分组描述"})
}

func TestGetPricingExposesGroupModelDescriptionsForVisibleGroupsOnly(t *testing.T) {
	setupGroupModelDescriptionFixture(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	GetPricing(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response groupModelDescriptionsPricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, map[string]map[string]string{
		"default": {"shared-model": "默认分组描述"},
		"vip":     {"vip-only-model": "VIP 分组描述"},
	}, response.GroupModelDescriptions)
	assert.NotContains(t, recorder.Body.String(), "隐藏分组描述")
}

func TestGetGroupModelMembersReturnsGroupModelsWithGroupDescription(t *testing.T) {
	setupGroupModelDescriptionFixture(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/models/group_members?group=vip", nil)
	GetGroupModelMembers(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response groupModelMembersResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	assert.Equal(t, groupModelMember{
		ModelName:        "vip-only-model",
		Description:      "全局描述",
		GroupDescription: "VIP 分组描述",
	}, response.Data[0])
}

func TestGetGroupModelMembersRequiresGroupName(t *testing.T) {
	setupGroupModelDescriptionFixture(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/models/group_members", nil)
	GetGroupModelMembers(ctx)

	var response groupModelMembersResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "缺少分组名称")
}
