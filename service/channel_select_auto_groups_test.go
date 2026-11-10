package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
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

func setupChannelSelectAutoGroupsTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalMaxTokenAutoGroups := setting.GetMaxTokenAutoGroups()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.UserChannelChain{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	common.RetryTimes = 0

	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", originalMaxTokenAutoGroups)))

		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	return db
}

func TestCacheGetRandomSatisfiedChannelResolvesChannelChain(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "channel-chain-runtime-model"
	createChannelSelectAutoGroupsChannel(t, db, 2301, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2302, "default", modelName)
	model.InitChannelCache()

	chain, err := model.CreateUserChannelChain(92002, "channel-chain", []string{"vip", "default"})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserId, 92002)
	ctx.Set("id", 92002)
	common.SetContextKey(ctx, constant.ContextKeyChannelChain, chain.GetGroupList())

	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "chain:" + chain.ChainId,
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2301, first.Id)
	assert.Equal(t, "vip", selectedGroup)

	param.IncreaseRetry()
	second, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2302, second.Id)
	assert.Equal(t, "default", selectedGroup)
}

func TestChannelChainSelectsChannelsByGroupOrder(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "channel-chain-group-order-model"
	createChannelSelectAutoGroupsChannel(t, db, 2401, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2402, "default", modelName)
	model.InitChannelCache()

	chain, err := model.CreateUserChannelChain(92003, "group-order-chain", []string{"vip", "default"})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyChannelChain, chain.GetGroupList())

	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "chain:" + chain.ChainId,
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2401, first.Id)
	assert.Equal(t, "vip", selectedGroup)
}

func createChannelSelectAutoGroupsChannel(t *testing.T, db *gorm.DB, id int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Weight:   &weight,
		Models:   modelName,
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func createChannelChainModelsChannel(t *testing.T, db *gorm.DB, id int, group string, models ...string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Weight:   &weight,
		Models:   strings.Join(models, ","),
		Group:    group,
		Priority: &priority,
	}).Error)
	for _, modelName := range models {
		require.NoError(t, db.Create(&model.Ability{
			Group:     group,
			Model:     modelName,
			ChannelId: id,
			Enabled:   true,
			Priority:  &priority,
			Weight:    weight,
		}).Error)
	}
}

func newModelsChainRetryParam(t *testing.T, chain *model.UserChannelChain, userId int, requestModel string) (*gin.Context, *RetryParam) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserId, userId)
	ctx.Set("id", userId)
	SetRequestChannelChain(ctx, chain)
	retry := 0
	return ctx, &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "chain:" + chain.ChainId,
		ModelName:   requestModel,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}
}

// TestChannelChainModelsFiltersCandidateGroupsByRequestModel covers the models
// chain contract: A[model-x,model-y] + B[model-x,model-z] must route model-x to
// A then B, model-y only to A and model-z only to B, without rewriting the
// request model or falling back to an ordinary group.
func TestChannelChainModelsFiltersCandidateGroupsByRequestModel(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	createChannelChainModelsChannel(t, db, 2601, "vip", "model-x", "model-y")
	createChannelChainModelsChannel(t, db, 2602, "default", "model-x", "model-z")
	model.InitChannelCache()

	chain, err := model.CreateUserChannelChainWithConfig(92004, "models-chain", model.UserChannelChainTypeModels, nil, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x", "model-y"}},
		{Group: "default", Models: []string{"model-x", "model-z"}},
	})
	require.NoError(t, err)
	require.Equal(t, model.UserChannelChainTypeModels, chain.GetType())

	groupCases := []struct {
		model string
		want  []string
	}{
		{model: "model-x", want: []string{"vip", "default"}},
		{model: "model-y", want: []string{"vip"}},
		{model: "model-z", want: []string{"default"}},
	}
	for _, tc := range groupCases {
		ctx, _ := newModelsChainRetryParam(t, chain, 92004, tc.model)
		groups, isChain, groupErr := GetRequestChannelChainGroups(ctx, tc.model)
		require.NoError(t, groupErr)
		require.True(t, isChain)
		assert.Equal(t, tc.want, groups)
	}

	// model-x walks the two rows in order.
	_, param := newModelsChainRetryParam(t, chain, 92004, "model-x")
	first, firstGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2601, first.Id)
	assert.Equal(t, "vip", firstGroup)
	param.IncreaseRetry()
	second, secondGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2602, second.Id)
	assert.Equal(t, "default", secondGroup)

	// model-y only exists on the first row: exhausting it returns no channel
	// and never falls through to the second row's ordinary group.
	_, paramY := newModelsChainRetryParam(t, chain, 92004, "model-y")
	firstY, groupY, err := CacheGetRandomSatisfiedChannel(paramY)
	require.NoError(t, err)
	require.NotNil(t, firstY)
	assert.Equal(t, 2601, firstY.Id)
	assert.Equal(t, "vip", groupY)
	paramY.IncreaseRetry()
	none, selectedGroup, err := CacheGetRandomSatisfiedChannel(paramY)
	require.NoError(t, err)
	assert.Nil(t, none)
	assert.Equal(t, "chain:"+chain.ChainId, selectedGroup)
}

// TestChannelChainModelsFailsClosedWhenModelNotConfigured pins the fail-closed
// contract: a request model that no row selects must produce the sentinel error
// instead of degrading to the token/ordinary group.
func TestChannelChainModelsFailsClosedWhenModelNotConfigured(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	createChannelChainModelsChannel(t, db, 2651, "vip", "known-model")
	model.InitChannelCache()

	chain, err := model.CreateUserChannelChainWithConfig(92005, "models-chain-miss", model.UserChannelChainTypeModels, nil, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"known-model"}},
	})
	require.NoError(t, err)

	ctx, param := newModelsChainRetryParam(t, chain, 92005, "unknown-model")
	groups, isChain, groupErr := GetRequestChannelChainGroups(ctx, "unknown-model")
	require.True(t, isChain)
	require.ErrorIs(t, groupErr, ErrChannelChainModelNotAvailable)
	assert.Nil(t, groups)

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.ErrorIs(t, err, ErrChannelChainModelNotAvailable)
	assert.Nil(t, channel)
	assert.Equal(t, "chain:"+chain.ChainId, selectedGroup)
}

// TestChannelChainModelsPinnedChannelKeepsBillingGroup ensures a pinned chain
// attempt resolves the real pricing group and that a pin without a prior chain
// selection cannot bypass the chain.
func TestChannelChainModelsPinnedChannelKeepsBillingGroup(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	createChannelChainModelsChannel(t, db, 2701, "vip", "model-pin")
	model.InitChannelCache()

	chain, err := model.CreateUserChannelChainWithConfig(92006, "models-chain-pin", model.UserChannelChainTypeModels, nil, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-pin"}},
	})
	require.NoError(t, err)

	_, param := newModelsChainRetryParam(t, chain, 92006, "model-pin")
	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "vip", selectedGroup)

	// Pin the channel selected through the chain: the next attempt must keep
	// the real group instead of returning "chain:<id>".
	param.SetPinnedChannel(selected)
	pinned, pinnedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, pinned)
	assert.Equal(t, selected.Id, pinned.Id)
	assert.Equal(t, "vip", pinnedGroup)

	// A pin in a context without any chain selection must be dropped and the
	// chain must select normally (never select the pinned channel directly).
	ctxNoSelection, paramNoSelection := newModelsChainRetryParam(t, chain, 92006, "model-pin")
	paramNoSelection.SetPinnedChannel(selected)
	reselected, reselectedGroup, err := CacheGetRandomSatisfiedChannel(paramNoSelection)
	require.NoError(t, err)
	require.NotNil(t, reselected)
	assert.Equal(t, "vip", reselectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctxNoSelection, constant.ContextKeyAutoGroup))
}

// TestChannelChainRuntimeDBLoadValidatesGroupAccess covers the non-auth path:
// when no request snapshot exists the selector loads the chain itself, and that
// row must pass the same structure/group authorization checks as TokenAuth
// before it can drive routing.
func TestChannelChainRuntimeDBLoadValidatesGroupAccess(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)

	chain, err := model.CreateUserChannelChain(92007, "db-load-chain", []string{"vip"})
	require.NoError(t, err)
	require.NoError(t, chain.SetGroupList([]string{"vip", "forbidden-group"}))
	require.NoError(t, db.Save(chain).Error)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserId, 92007)
	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "chain:" + chain.ChainId,
		ModelName:   "any-model",
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	channel, _, err := CacheGetRandomSatisfiedChannel(param)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unavailable")
	assert.Nil(t, channel)
	_, cached := GetRequestChannelChain(ctx)
	assert.False(t, cached, "an unauthorized chain must not be cached as a routing snapshot")
}

// TestRequestChainGroupsRejectsUnknownTypeAndEmptyGroups keeps the resolver
// fail-closed: an unknown persisted type is not a groups chain, and a groups
// chain with no groups is invalid instead of an empty candidate list.
func TestRequestChainGroupsRejectsUnknownTypeAndEmptyGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unknownCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetRequestChannelChain(unknownCtx, &model.UserChannelChain{ChainId: "weird001", Type: "weird"})
	groups, isChain, err := GetRequestChannelChainGroups(unknownCtx, "model-x")
	require.True(t, isChain)
	require.Error(t, err)
	assert.Nil(t, groups)
	require.NotErrorIs(t, err, ErrChannelChainModelNotAvailable)

	emptyCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetRequestChannelChain(emptyCtx, &model.UserChannelChain{ChainId: "empty001", Type: model.UserChannelChainTypeGroups})
	groups, isChain, err = GetRequestChannelChainGroups(emptyCtx, "model-x")
	require.True(t, isChain)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no groups")
	assert.Nil(t, groups)
}

// TestChannelChainModelsPinnedChannelRejectsNonCandidateGroup guards the pin
// boundary: a group that serves the model but is not one of this request's
// filtered chain rows must not be reused as the billing group.
func TestChannelChainModelsPinnedChannelRejectsNonCandidateGroup(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	createChannelChainModelsChannel(t, db, 2721, "vip", "model-pin")
	createChannelChainModelsChannel(t, db, 2722, "default", "model-pin")
	model.InitChannelCache()

	chain, err := model.CreateUserChannelChainWithConfig(92008, "models-chain-pin-guard", model.UserChannelChainTypeModels, nil, []model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-pin"}},
		{Group: "default", Models: []string{"model-other"}},
	})
	require.NoError(t, err)

	ctx, param := newModelsChainRetryParam(t, chain, 92008, "model-pin")
	// Simulate a stale pin that was selected in a group outside the candidate
	// rows for this model (default serves model-pin, but no row selects it).
	common.SetContextKey(ctx, constant.ContextKeyAutoGroup, "default")
	nonCandidate, err := model.CacheGetChannel(2722)
	require.NoError(t, err)
	require.NotNil(t, nonCandidate)
	param.SetPinnedChannel(nonCandidate)

	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 2721, selected.Id)
	assert.Equal(t, "vip", selectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
}

// TestSetRequestChannelChainResetsDerivedContext pins the snapshot boundary:
// loading a chain must drop the previous snapshot's derived candidate cache and
// legacy groups key so stale groups cannot survive a re-load.
func TestSetRequestChannelChainResetsDerivedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyChannelChain, []string{"stale-group"})
	common.SetContextKey(ctx, constant.ContextKeyChannelChainGroups, map[string][]string{"stale-model": {"stale-group"}})

	chain := &model.UserChannelChain{ChainId: "mdlreset", Type: model.UserChannelChainTypeModels}
	require.NoError(t, chain.SetItems([]model.UserChannelChainItem{{Group: "vip", Models: []string{"model-x"}}}))
	SetRequestChannelChain(ctx, chain)

	assert.Empty(t, common.GetContextKeyStringSlice(ctx, constant.ContextKeyChannelChain))
	_, staleHit := cachedRequestChannelChainGroups(ctx, "stale-model")
	assert.False(t, staleHit)
	_, ok := GetRequestChannelChain(ctx)
	assert.True(t, ok)

	// A typed nil snapshot must be ignored without panicking or clearing state.
	var nilChain *model.UserChannelChain
	SetRequestChannelChain(ctx, nilChain)
	GetRequestChannelChain(ctx)
	assert.True(t, IsRequestModelChannelChain(ctx))
}

// TestChannelChainTokenGroupWithEmptyIdFailsClosed pins the malformed token
// group boundary: "chain:" without an id must stay a chain request and error,
// not silently select an ordinary group.
func TestChannelChainTokenGroupWithEmptyIdFailsClosed(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	createChannelSelectAutoGroupsChannel(t, db, 2731, "default", "empty-chain-id-model")
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserId, 92010)
	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "chain:",
		ModelName:   "empty-chain-id-model",
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	channel, _, err := CacheGetRandomSatisfiedChannel(param)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel chain token group")
	assert.Nil(t, channel, "a malformed chain token group must not fall back to an ordinary channel")
}

// TestChannelChainRuntimeDBLoadReadsUserGroupFromDB covers the non-auth path
// where the request context has no user group: authorization must use the
// owner's persisted group instead of validating against an empty group.
func TestChannelChainRuntimeDBLoadReadsUserGroupFromDB(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	originalMainDBType := common.MainDatabaseType()
	originalLogDBType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitDatabaseColumnNames()
	t.Cleanup(func() {
		common.SetDatabaseTypes(originalMainDBType, originalLogDBType)
		model.InitDatabaseColumnNames()
	})
	require.NoError(t, db.Create(&model.User{Id: 92011, Username: "chain-user-92011", Group: "default"}).Error)

	createChannelSelectAutoGroupsChannel(t, db, 2741, "vip", "db-group-chain-model")
	model.InitChannelCache()
	chain, err := model.CreateUserChannelChain(92011, "db-group-chain", []string{"vip"})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserId, 92011)
	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "chain:" + chain.ChainId,
		ModelName:   "db-group-chain-model",
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 2741, channel.Id)
	assert.Equal(t, "vip", selectedGroup)
}

func TestCacheGetRandomSatisfiedChannelUsesTokenAutoGroupsWhenGlobalAutoIsEmpty(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "auto-groups-runtime-model"
	createChannelSelectAutoGroupsChannel(t, db, 2101, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2102, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)

	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "auto",
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2101, first.Id)
	assert.Equal(t, "vip", selectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
	assert.Empty(t, setting.GetAutoGroups(), "the selection must not depend on the global Auto list")

	param.IncreaseRetry()
	second, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2102, second.Id)
	assert.Equal(t, "default", selectedGroup)
	assert.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
}

// TestCacheGetRandomSatisfiedChannelKeepsPinnedChannel covers the model
// candidate failover path: the relay pins the current channel so the next
// attempt tries another upstream model of the same channel instead of letting
// the group selection pick a different one.
func TestCacheGetRandomSatisfiedChannelKeepsPinnedChannel(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "pinned-channel-runtime-model"
	baseURL := "http://upstream.example.com"
	mapping := `{"alias-model":["candidate-a","candidate-b"]}`
	createChannelSelectAutoGroupsChannel(t, db, 2501, "default", modelName)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 2501).Updates(map[string]any{
		"base_url":      baseURL,
		"model_mapping": mapping,
	}).Error)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")

	retry := 3
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "default",
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	// 请求上下文里的渠道只有身份字段：pin 必须解析出带地址与映射的完整渠道，
	// 否则重试时会用空的 base URL 发往上游。
	param.SetPinnedChannel(&model.Channel{
		Id:     2501,
		Type:   constant.ChannelTypeOpenAI,
		Name:   "context-synthesized-channel",
		Status: common.ChannelStatusEnabled,
	})
	require.NotNil(t, param.PinnedChannel)
	assert.Equal(t, baseURL, param.PinnedChannel.GetBaseURL())
	assert.Equal(t, mapping, param.PinnedChannel.GetModelMapping())

	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 2501, selected.Id)
	assert.Equal(t, baseURL, selected.GetBaseURL())
	assert.Equal(t, "default", selectedGroup)

	param.ClearPinnedChannel()
	selected, _, err = CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 2501, selected.Id)
}
