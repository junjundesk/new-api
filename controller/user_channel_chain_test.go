package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelChainControllerTest(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()
	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
	originalRedis, originalMemory, originalGinMode := common.RedisEnabled, common.MemoryCacheEnabled, gin.Mode()
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitDatabaseColumnNames()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:chain_controller_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.RedisEnabled, common.MemoryCacheEnabled = originalRedis, originalMemory
		common.SetDatabaseTypes(originalMainType, originalLogType)
		model.InitDatabaseColumnNames()
		gin.SetMode(originalGinMode)
		model.InvalidatePricingCache()
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.UserChannelChain{}, &model.Ability{}, &model.Channel{}, &model.Model{}, &model.Vendor{}, &model.Log{}, &model.ActivityGrant{}))
	configureTokenAutoGroupsTest(t, "5", `["default","vip"]`)
	withSelfUseModeEnabled(t)
	user := &model.User{Id: 101, Username: "chain-owner", AffCode: "chain101", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&model.User{Id: 102, Username: "other-owner", AffCode: "chain102", Group: "default", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "model-x", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "model-y", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "unselected", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "model-x", ChannelId: 2, Enabled: true},
		{Group: "vip", Model: "model-y", ChannelId: 2, Enabled: true},
		{Group: "vip", Model: "model-z", ChannelId: 2, Enabled: true},
	}).Error)
	model.InvalidatePricingCache()
	return db, user
}

func createControllerModelChain(t *testing.T, userID int) *model.UserChannelChain {
	t.Helper()
	items := []model.UserChannelChainItem{{Group: "default", Models: []string{"model-x", "model-y"}}, {Group: "vip", Models: []string{"model-x", "model-z"}}}
	chain, err := model.CreateUserChannelChainWithConfig(userID, "Model chain", model.UserChannelChainTypeModels, nil, items)
	require.NoError(t, err)
	return chain
}

func TestChannelChainControllerRejectsInvalidConfigAndTypeChanges(t *testing.T) {
	db, user := setupChannelChainControllerTest(t)
	chain := createControllerModelChain(t, user.Id)
	tests := []struct {
		name string
		body map[string]any
	}{
		{"missing type", map[string]any{"name": "changed", "groups": []string{"default"}}},
		{"unknown type", map[string]any{"name": "changed", "type": "other", "groups": []string{"default"}}},
		{"empty rows", map[string]any{"name": "changed", "type": "models", "items": []any{}}},
		{"mismatched groups", map[string]any{"name": "changed", "type": "models", "groups": []string{"vip"}, "items": []model.UserChannelChainItem{{Group: "default", Models: []string{"model-x"}}}}},
		{"model outside group", map[string]any{"name": "changed", "type": "models", "items": []model.UserChannelChainItem{{Group: "default", Models: []string{"model-z"}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self/channel_chains/"+chain.ChainId, test.body, user.Id)
			ctx.Params = gin.Params{{Key: "chain_id", Value: chain.ChainId}}
			UpdateUserChannelChain(ctx)
			assert.False(t, decodeAPIResponse(t, recorder).Success)
			var stored model.UserChannelChain
			require.NoError(t, db.First(&stored, chain.Id).Error)
			assert.Equal(t, chain.Name, stored.Name)
			assert.Equal(t, chain.Type, stored.Type)
			assert.Equal(t, chain.Items, stored.Items)
		})
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self/channel_chains/"+chain.ChainId, map[string]any{"name": "not owned", "type": "models", "items": []model.UserChannelChainItem{{Group: "default", Models: []string{"model-x"}}}}, 102)
	ctx.Params = gin.Params{{Key: "chain_id", Value: chain.ChainId}}
	UpdateUserChannelChain(ctx)
	assert.False(t, decodeAPIResponse(t, recorder).Success)
}

func TestModelChainCatalogsIntersectEachRowAndNeverExpandEmptyResults(t *testing.T) {
	db, user := setupChannelChainControllerTest(t)
	chain := createControllerModelChain(t, user.Id)
	chainGroup := "chain:" + chain.ChainId
	// Y remains enabled in VIP but was selected only in the default row.
	require.NoError(t, db.Model(&model.Ability{}).Where(&model.Ability{Group: "default", Model: "model-y"}).Update("enabled", false).Error)
	for _, empty := range []bool{false, true} {
		if empty {
			require.NoError(t, db.Model(&model.Ability{}).Where("model IN ?", []string{"model-x", "model-z"}).Update("enabled", false).Error)
		}
		expected := []string{"model-x", "model-z"}
		if empty {
			expected = []string{}
		}
		ctx, recorder := newTokenAutoGroupsAuthenticatedContext(t, http.MethodGet, "/api/user/models?group="+chainGroup, nil, user.Id)
		GetUserModels(ctx)
		assert.ElementsMatch(t, expected, decodeUserModelsResponse(t, recorder))
		tokenModels, err := getTokenGroupModels(ctx, chainGroup, nil)
		require.NoError(t, err)
		assert.ElementsMatch(t, expected, tokenModels)
		ctx, recorder = newTokenAutoGroupsAuthenticatedContext(t, http.MethodGet, "/v1/models", nil, user.Id)
		common.SetContextKey(ctx, constant.ContextKeyTokenGroup, chainGroup)
		service.SetRequestChannelChain(ctx, chain)
		common.SetContextKey(ctx, constant.ContextKeyTokenModelMappings, []setting.TokenModelMapping{{SourceModel: "alias-x", TargetModel: "model-x"}})
		ListModels(ctx, constant.ChannelTypeOpenAI)
		listed := decodeListModelsResponse(t, recorder)
		if empty {
			assert.Empty(t, listed)
		} else {
			assert.Equal(t, map[string]struct{}{"alias-x": {}, "model-z": {}}, listed)
		}
	}
}

func TestModelChainCatalogRejectsInvalidChainAndMismatchedSnapshots(t *testing.T) {
	_, user := setupChannelChainControllerTest(t)
	chain, err := model.CreateUserChannelChainWithConfig(user.Id, "Catalog target", model.UserChannelChainTypeModels, nil, []model.UserChannelChainItem{{Group: "default", Models: []string{"model-x"}}})
	require.NoError(t, err)
	otherChain, err := model.CreateUserChannelChainWithConfig(user.Id, "Another chain", model.UserChannelChainTypeModels, nil, []model.UserChannelChainItem{{Group: "vip", Models: []string{"model-z"}}})
	require.NoError(t, err)
	otherOwner := *otherChain
	otherOwner.ChainId = chain.ChainId
	otherOwner.UserId = 102
	for _, snapshot := range []*model.UserChannelChain{otherChain, &otherOwner} {
		ctx, recorder := newTokenAutoGroupsAuthenticatedContext(t, http.MethodGet, "/v1/models", nil, user.Id)
		common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "chain:"+chain.ChainId)
		service.SetRequestChannelChain(ctx, snapshot)
		ListModels(ctx, constant.ChannelTypeOpenAI)
		assert.Equal(t, map[string]struct{}{"model-x": {}}, decodeListModelsResponse(t, recorder))
	}
	ctx, recorder := newTokenAutoGroupsAuthenticatedContext(t, http.MethodGet, "/v1/models", nil, user.Id)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "chain:")
	ListModels(ctx, constant.ChannelTypeOpenAI)
	assert.False(t, decodeAPIResponse(t, recorder).Success)
	ctx, recorder = newTokenAutoGroupsAuthenticatedContext(t, http.MethodGet, "/api/user/models?group=chain:", nil, user.Id)
	GetUserModels(ctx)
	assert.False(t, decodeAPIResponse(t, recorder).Success)
}

func TestModelChainMappingTargetsRespectEffectiveCatalog(t *testing.T) {
	db, user := setupChannelChainControllerTest(t)
	chain := createControllerModelChain(t, user.Id)
	// A model enabled elsewhere must not restore a disabled selection in this row.
	require.NoError(t, db.Model(&model.Ability{}).Where(&model.Ability{Group: "default", Model: "model-y"}).Update("enabled", false).Error)
	tests := []struct {
		target  string
		allowed bool
	}{
		{target: "unselected"},
		{target: "model-y"},
		{target: "model-x", allowed: true},
	}
	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			request := baseAutoTokenRequest("mapping-" + test.target)
			request["group"] = "chain:" + chain.ChainId
			request["model_mappings"] = []setting.TokenModelMapping{{SourceModel: "alias-x", TargetModel: test.target}}
			ctx, recorder := newTokenAutoGroupsAuthenticatedContext(t, http.MethodPost, "/api/token/", request, user.Id)
			AddToken(ctx)
			response := decodeAPIResponse(t, recorder)
			require.Equal(t, test.allowed, response.Success, response.Message)
			var stored []model.Token
			require.NoError(t, db.Where("name = ?", "mapping-"+test.target).Find(&stored).Error)
			if !test.allowed {
				assert.Empty(t, stored)
				return
			}
			require.Len(t, stored, 1)
			mappings, err := stored[0].GetModelMappings()
			require.NoError(t, err)
			assert.Equal(t, []setting.TokenModelMapping{{SourceModel: "alias-x", TargetModel: test.target}}, mappings)
		})
	}
}

func TestModelChainTokenSavesRejectEnabledFallbackBeforeShortcuts(t *testing.T) {
	tests := []struct {
		name             string
		create           bool
		groupOnly        bool
		includeFallbacks bool
	}{
		{name: "create without fallback list", create: true},
		{name: "create with fallback list", create: true, includeFallbacks: true},
		{name: "full update without fallback list"},
		{name: "full update unchanged fallback list", includeFallbacks: true},
		{name: "group only retains stored enabled flag", groupOnly: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, user := setupChannelChainControllerTest(t)
			chain := createControllerModelChain(t, user.Id)
			fallbacks := []setting.TokenFallbackModel{{SourceModel: "model-x", Group: "vip", Model: "model-z"}}
			request := baseAutoTokenRequest("fallback-conflict")
			request["group"] = "chain:" + chain.ChainId
			request["fallback_enabled"] = true
			var token *model.Token
			if !test.create {
				token = seedToken(t, db, user.Id, "existing-fallback", "existing-fallback-key")
				token.FallbackEnabled = true
				require.NoError(t, token.SetFallbackModels(fallbacks))
				require.NoError(t, db.Save(token).Error)
				request["id"] = token.Id
			}
			if test.includeFallbacks {
				request["fallback_models"] = fallbacks
			}
			target, method := "/api/token/", http.MethodPut
			if test.create {
				method = http.MethodPost
			}
			if test.groupOnly {
				target += "?group_only=true"
				// group_only does not change the stored flag, even if the client sends false.
				request["fallback_enabled"] = false
			}
			ctx, recorder := newTokenAutoGroupsAuthenticatedContext(t, method, target, request, user.Id)
			if test.create {
				AddToken(ctx)
			} else {
				UpdateToken(ctx)
			}
			require.Equal(t, http.StatusOK, recorder.Code)
			assert.False(t, decodeAPIResponse(t, recorder).Success)
			if test.create {
				var count int64
				require.NoError(t, db.Model(&model.Token{}).Count(&count).Error)
				assert.Zero(t, count)
			} else {
				var stored model.Token
				require.NoError(t, db.First(&stored, token.Id).Error)
				assert.Equal(t, token.Group, stored.Group)
				assert.True(t, stored.FallbackEnabled)
				assert.Equal(t, token.FallbackModels, stored.FallbackModels)
			}
		})
	}
}

func TestModelChainTokenSavesPreserveDisabledDraft(t *testing.T) {
	db, user := setupChannelChainControllerTest(t)
	chain := createControllerModelChain(t, user.Id)
	drafts := []setting.TokenFallbackModel{{SourceModel: "removed-model", Group: "", Model: ""}}
	request := baseAutoTokenRequest("disabled-draft")
	request["group"] = "chain:" + chain.ChainId
	request["fallback_enabled"] = false
	request["fallback_models"] = drafts
	ctx, recorder := newTokenAutoGroupsAuthenticatedContext(t, http.MethodPost, "/api/token/", request, user.Id)
	AddToken(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var token model.Token
	require.NoError(t, db.Where("name = ?", "disabled-draft").First(&token).Error)
	assert.False(t, token.FallbackEnabled)
	savedDraft, err := token.GetFallbackModels()
	require.NoError(t, err)
	assert.Equal(t, drafts, savedDraft)
	for _, groupOnly := range []bool{false, true} {
		target := "/api/token/"
		request["id"] = token.Id
		delete(request, "fallback_models")
		if groupOnly {
			target += "?group_only=true"
		}
		ctx, recorder = newTokenAutoGroupsAuthenticatedContext(t, http.MethodPut, target, request, user.Id)
		UpdateToken(ctx)
		response = decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		var stored model.Token
		require.NoError(t, db.First(&stored, token.Id).Error)
		assert.False(t, stored.FallbackEnabled)
		assert.Equal(t, token.FallbackModels, stored.FallbackModels)
		assert.False(t, stored.CrossGroupRetry)
		assert.Empty(t, stored.AutoGroups)
	}
}

func TestGroupChainStillAllowsEnabledFallback(t *testing.T) {
	db, user := setupChannelChainControllerTest(t)
	chain, err := model.CreateUserChannelChain(user.Id, "Legacy", []string{"default", "vip"})
	require.NoError(t, err)
	token := seedToken(t, db, user.Id, "legacy-fallback", "legacy-fallback-key")
	token.FallbackEnabled = true
	require.NoError(t, token.SetFallbackModels([]setting.TokenFallbackModel{{Group: "vip", Model: "model-z"}}))
	require.NoError(t, db.Save(token).Error)
	request := map[string]any{"id": token.Id, "group": "chain:" + chain.ChainId, "unlimited_quota": true}
	ctx, recorder := newTokenAutoGroupsAuthenticatedContext(t, http.MethodPut, "/api/token/?group_only=true", request, user.Id)
	UpdateToken(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	assert.Equal(t, "chain:"+chain.ChainId, stored.Group)
	assert.True(t, stored.FallbackEnabled)
	assert.Equal(t, token.FallbackModels, stored.FallbackModels)
}

func TestUserGroupsExposeModelChainMetadata(t *testing.T) {
	_, user := setupChannelChainControllerTest(t)
	chain := createControllerModelChain(t, user.Id)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self/groups", nil, user.Id)
	GetUserGroups(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var groups map[string]map[string]any
	require.NoError(t, common.Unmarshal(response.Data, &groups))
	assert.Equal(t, true, groups["default"]["chain_selectable"])
	assert.Equal(t, true, groups["vip"]["chain_selectable"])
	metadata := groups["chain:"+chain.ChainId]
	assert.Equal(t, false, metadata["chain_selectable"])
	assert.Equal(t, "models", metadata["chain_type"])
	assert.Equal(t, []any{"default", "vip"}, metadata["chain_groups"])
	require.Len(t, metadata["chain_items"], 2)
}

func TestChannelChainControllerCRUDCompatibility(t *testing.T) {
	db, user := setupChannelChainControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/self/channel_chains", map[string]any{"name": "  Legacy chain  ", "groups": []string{"default", "vip"}}, user.Id)
	CreateUserChannelChain(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var legacy model.UserChannelChain
	require.NoError(t, common.Unmarshal(response.Data, &legacy))
	require.NotEmpty(t, legacy.ChainId)
	assert.Equal(t, "Legacy chain", legacy.Name)
	items := []model.UserChannelChainItem{{Group: "default", Models: []string{"model-x", "model-y"}}, {Group: "vip", Models: []string{"model-z"}}}
	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/user/self/channel_chains", map[string]any{"name": "Models", "type": "models", "items": items}, user.Id)
	CreateUserChannelChain(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var modelChain model.UserChannelChain
	require.NoError(t, common.Unmarshal(response.Data, &modelChain))
	stored, err := model.GetUserChannelChain(user.Id, modelChain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, []string{"default", "vip"}, stored.GetGroupList())
	storedItems, err := stored.GetItems()
	require.NoError(t, err)
	assert.Equal(t, items, storedItems)
	require.NoError(t, db.Create(&model.Token{UserId: user.Id, Key: "chain-bound", Group: "chain:" + modelChain.ChainId}).Error)
	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/user/self/channel_chains", nil, user.Id)
	GetUserChannelChains(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var listed map[string]any
	require.NoError(t, common.Unmarshal(response.Data, &listed))
	chains := listed["chains"].([]any)
	require.Len(t, chains, 2)
	assert.Equal(t, "groups", chains[0].(map[string]any)["type"])
	assert.Equal(t, []any{}, chains[0].(map[string]any)["items"])
	assert.Equal(t, "models", chains[1].(map[string]any)["type"])
	assert.EqualValues(t, 100, listed["max_models_per_item"])
	assert.EqualValues(t, 60*1024, listed["max_items_bytes"])
	assert.EqualValues(t, 1, listed["token_usage"].(map[string]any)["chain:"+modelChain.ChainId])
	ctx, recorder = newAuthenticatedContext(t, http.MethodPut, "/api/user/self/channel_chains/"+legacy.ChainId, map[string]any{"name": "Legacy updated", "groups": []string{"vip"}}, user.Id)
	ctx.Params = gin.Params{{Key: "chain_id", Value: legacy.ChainId}}
	UpdateUserChannelChain(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var updated map[string]string
	require.NoError(t, common.Unmarshal(response.Data, &updated))
	assert.Equal(t, legacy.ChainId, updated["chain_id"])
	for _, owner := range []int{102, user.Id} {
		ctx, recorder = newAuthenticatedContext(t, http.MethodDelete, "/api/user/self/channel_chains/"+modelChain.ChainId, nil, owner)
		ctx.Params = gin.Params{{Key: "chain_id", Value: modelChain.ChainId}}
		DeleteUserChannelChain(ctx)
		response = decodeAPIResponse(t, recorder)
		assert.Equal(t, owner == user.Id, response.Success)
		if owner == user.Id {
			var deleted map[string]int
			require.NoError(t, common.Unmarshal(response.Data, &deleted))
			assert.Equal(t, 1, deleted["reset_tokens"])
		}
	}
}
