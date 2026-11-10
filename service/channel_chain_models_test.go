package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelChainModelsTest(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Ability{}, &model.UserChannelChain{}))
	model.DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitDatabaseColumnNames()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString("{\"default\":\"Default\",\"vip\":\"VIP\"}"))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString("{\"default\":1,\"vip\":2}"))

	t.Cleanup(func() {
		model.DB = originalDB
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		model.InitDatabaseColumnNames()
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	return db
}

func seedChannelChainModelsAbility(t *testing.T, db *gorm.DB, group string, modelNames ...string) {
	t.Helper()
	for _, modelName := range modelNames {
		require.NoError(t, db.Create(&model.Ability{Group: group, Model: modelName, ChannelId: 1, Enabled: true}).Error)
	}
}

func newChannelChainModelsForTest(t *testing.T, items []model.UserChannelChainItem) *model.UserChannelChain {
	t.Helper()
	chain := &model.UserChannelChain{Type: model.UserChannelChainTypeModels}
	require.NoError(t, chain.SetItems(items))
	return chain
}

func TestGetUserChannelChainModelsSortsGroupsChainCatalog(t *testing.T) {
	db := setupChannelChainModelsTest(t)
	seedChannelChainModelsAbility(t, db, "vip", "model-z", "model-a")
	seedChannelChainModelsAbility(t, db, "default", "model-b", "model-z")

	chain := &model.UserChannelChain{}
	require.NoError(t, chain.SetGroupList([]string{"vip", "default"}))
	models, err := GetUserChannelChainModels(chain)
	require.NoError(t, err)
	assert.Equal(t, []string{"model-a", "model-b", "model-z"}, models)

	empty := &model.UserChannelChain{}
	models, err = GetUserChannelChainModels(empty)
	require.NoError(t, err)
	assert.Empty(t, models)
	assert.NotNil(t, models)
}

func TestGetUserChannelChainModelsIntersectsEachRowWithEnabledCatalog(t *testing.T) {
	db := setupChannelChainModelsTest(t)
	seedChannelChainModelsAbility(t, db, "vip", "model-x", "model-y", "model-unselected")
	seedChannelChainModelsAbility(t, db, "default", "model-x", "model-z")
	require.NoError(t, db.Model(&model.Ability{}).
		Where(&model.Ability{Group: "default", Model: "model-z"}).
		Update("enabled", false).Error)

	chain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x", "model-y"}},
		{Group: "default", Models: []string{"model-x", "model-z"}},
	})
	models, err := GetUserChannelChainModels(chain)
	require.NoError(t, err)
	assert.Equal(t, []string{"model-x", "model-y"}, models)

	// A model selected on two rows but enabled only in the later group appears
	// once, and is not borrowed from a group that does not enable it.
	seedChannelChainModelsAbility(t, db, "default", "model-w")
	rowChain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-w"}},
		{Group: "default", Models: []string{"model-w"}},
	})
	models, err = GetUserChannelChainModels(rowChain)
	require.NoError(t, err)
	assert.Equal(t, []string{"model-w"}, models)
}

func TestGetUserChannelChainModelsReturnsEmptyCatalogWithoutEnabledModels(t *testing.T) {
	setupChannelChainModelsTest(t)
	chain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{{Group: "vip", Models: []string{"model-x"}}})
	models, err := GetUserChannelChainModels(chain)
	require.NoError(t, err)
	assert.Equal(t, []string{}, models)
	assert.NotNil(t, models)

	groups := &model.UserChannelChain{}
	require.NoError(t, groups.SetGroupList([]string{"vip"}))
	models, err = GetUserChannelChainModels(groups)
	require.NoError(t, err)
	assert.Equal(t, []string{}, models)
	assert.NotNil(t, models)
}

func TestGetUserChannelChainModelsRequiresDatabase(t *testing.T) {
	setupChannelChainModelsTest(t)
	chain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{{Group: "vip", Models: []string{"model-x"}}})
	groups := &model.UserChannelChain{}
	require.NoError(t, groups.SetGroupList([]string{"vip"}))

	_, err := GetUserChannelChainModels(nil)
	require.Error(t, err)
	_, err = GetUserChannelChainModelGroups(nil, "model-x")
	require.Error(t, err)
	require.Error(t, ValidateUserChannelChain(nil, "default", false))

	model.DB = nil
	_, err = GetUserChannelChainModels(chain)
	require.ErrorContains(t, err, "database is not initialized")
	_, err = GetUserChannelChainModels(groups)
	require.ErrorContains(t, err, "database is not initialized")
	_, err = GetUserChannelChainModelGroups(chain, "model-x")
	require.ErrorContains(t, err, "database is not initialized")
}

func TestGetUserChannelChainModelGroupsFiltersBySelectedEnabledModel(t *testing.T) {
	db := setupChannelChainModelsTest(t)
	seedChannelChainModelsAbility(t, db, "vip", "model-x", "model-y")
	seedChannelChainModelsAbility(t, db, "default", "model-x")

	chain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x", "model-y"}},
		{Group: "default", Models: []string{"model-x"}},
	})
	tests := []struct {
		name      string
		modelName string
		want      []string
	}{
		{name: "selected on both rows", modelName: "model-x", want: []string{"vip", "default"}},
		{name: "selected on first row only", modelName: "model-y", want: []string{"vip"}},
		{name: "not selected anywhere", modelName: "model-z", want: []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups, err := GetUserChannelChainModelGroups(chain, test.modelName)
			require.NoError(t, err)
			assert.Equal(t, test.want, groups)
			assert.NotNil(t, groups)
		})
	}

	// A blank model cannot resolve to any group; it must stay empty instead of
	// falling back to every configured row.
	blankGroups, err := GetUserChannelChainModelGroups(chain, "   ")
	require.NoError(t, err)
	assert.Nil(t, blankGroups)

	// Selecting a model is not enough: it must be enabled in that group now.
	require.NoError(t, db.Model(&model.Ability{}).Where(&model.Ability{Model: "model-y"}).Update("enabled", false).Error)
	groups, err := GetUserChannelChainModelGroups(chain, "model-y")
	require.NoError(t, err)
	assert.Equal(t, []string{}, groups)
	assert.NotNil(t, groups)

	// A groups chain exposes every group, independent of model catalogs.
	groupsChain := &model.UserChannelChain{}
	require.NoError(t, groupsChain.SetGroupList([]string{"vip", "default"}))
	groups, err = GetUserChannelChainModelGroups(groupsChain, "model-not-in-any-catalog")
	require.NoError(t, err)
	assert.Equal(t, []string{"vip", "default"}, groups)
}

func TestGetUserChannelChainModelGroupsMatchesExplicitNormalizedModel(t *testing.T) {
	db := setupChannelChainModelsTest(t)
	seedChannelChainModelsAbility(t, db, "vip", "gpt-4o", "gpt-4-gizmo-*", "gemini-2.5-flash-thinking-*")
	seedChannelChainModelsAbility(t, db, "default", "gpt-4o-2024-08-06")

	chain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"gpt-4o", "gpt-4-gizmo-*", "gemini-2.5-flash-thinking-*"}},
		{Group: "default", Models: []string{"gpt-4o-2024-08-06"}},
	})
	tests := []struct {
		name      string
		modelName string
		want      []string
	}{
		{name: "exact row model", modelName: "gpt-4o", want: []string{"vip"}},
		{name: "date suffix does not expand to base model", modelName: "gpt-4o-2024-08-06", want: []string{"default"}},
		{name: "gizmo normalized identifier explicitly selected", modelName: "gpt-4-gizmo-custom", want: []string{"vip"}},
		{name: "gizmo normalized identifier not selected", modelName: "gpt-4o-gizmo-custom", want: []string{}},
		{name: "thinking normalized identifier explicitly selected", modelName: "gemini-2.5-flash-thinking-8192", want: []string{"vip"}},
		{name: "non thinking gemini is not expanded", modelName: "gemini-2.5-flash-8192", want: []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups, err := GetUserChannelChainModelGroups(chain, test.modelName)
			require.NoError(t, err)
			assert.Equal(t, test.want, groups)
			assert.NotNil(t, groups)
		})
	}
}

func TestGetUserChannelChainModelGroupsRequiresNormalizedModelEnabled(t *testing.T) {
	db := setupChannelChainModelsTest(t)
	seedChannelChainModelsAbility(t, db, "vip", "gpt-4-gizmo-*")
	require.NoError(t, db.Model(&model.Ability{}).Where(&model.Ability{Model: "gpt-4-gizmo-*"}).Update("enabled", false).Error)

	chain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{{Group: "vip", Models: []string{"gpt-4-gizmo-*"}}})
	groups, err := GetUserChannelChainModelGroups(chain, "gpt-4-gizmo-custom")
	require.NoError(t, err)
	assert.Equal(t, []string{}, groups)
	assert.NotNil(t, groups)
}

func TestValidateUserChannelChainStructureAndModelCatalog(t *testing.T) {
	db := setupChannelChainModelsTest(t)
	seedChannelChainModelsAbility(t, db, "vip", "model-x")
	seedChannelChainModelsAbility(t, db, "default", "model-x")

	chain := newChannelChainModelsForTest(t, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x"}},
		{Group: "default", Models: []string{"model-x"}},
	})
	require.NoError(t, ValidateUserChannelChain(chain, "default", false))
	require.NoError(t, ValidateUserChannelChain(chain, "default", true))

	// Runtime validation ignores a model that is temporarily disabled.
	require.NoError(t, db.Model(&model.Ability{}).Where(&model.Ability{Group: "default", Model: "model-x"}).Update("enabled", false).Error)
	require.NoError(t, ValidateUserChannelChain(chain, "default", false))
	require.ErrorContains(t, ValidateUserChannelChain(chain, "default", true), "not available")

	broken := &model.UserChannelChain{Type: model.UserChannelChainTypeModels, Items: "[{\"group\":\"vip\",\"models\":[]}]"}
	require.Error(t, ValidateUserChannelChain(broken, "default", false))

	nested := &model.UserChannelChain{}
	require.NoError(t, nested.SetGroupList([]string{"chain:abc"}))
	require.ErrorContains(t, ValidateUserChannelChain(nested, "default", false), "nested")

	unknown := &model.UserChannelChain{Type: "custom"}
	require.ErrorIs(t, ValidateUserChannelChain(unknown, "default", false), model.ErrInvalidChannelChainType)

	originalDB := model.DB
	model.DB = nil
	require.NoError(t, ValidateUserChannelChain(chain, "default", false))
	require.ErrorContains(t, ValidateUserChannelChain(chain, "default", true), "database is not initialized")
	model.DB = originalDB
}
