package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newModelChainTokenContext(t *testing.T, chain *model.UserChannelChain) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	service.SetRequestChannelChain(ctx, chain)
	return ctx
}

func newModelsTypeChain(t *testing.T, userId int, chainId string) *model.UserChannelChain {
	t.Helper()
	chain := &model.UserChannelChain{
		Id:      91021,
		UserId:  userId,
		ChainId: chainId,
		Name:    "models-chain",
		Type:    model.UserChannelChainTypeModels,
	}
	require.NoError(t, chain.SetItems([]model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x"}},
	}))
	return chain
}

// TestSetupContextForTokenDisablesFallbackOnModelsChain is the runtime gate for
// the cross-model fallback contract: a token carrying fallback configuration
// must not get it activated while bound to a models-type chain.
func TestSetupContextForTokenDisablesFallbackOnModelsChain(t *testing.T) {
	require.NoError(t, i18n.Init())
	chain := newModelsTypeChain(t, 97001, "mdl00101")
	ctx := newModelChainTokenContext(t, chain)
	token := &model.Token{
		Id:              96001,
		UserId:          97001,
		Key:             "sk-test",
		Name:            "chain-token",
		UnlimitedQuota:  true,
		Group:           "chain:" + chain.ChainId,
		FallbackEnabled: true,
		FallbackModels:  `[{"group":"vip","model":"fallback-model"}]`,
	}

	require.NoError(t, SetupContextForToken(ctx, token))
	fallbacks, ok := common.GetContextKeyType[[]setting.TokenFallbackModel](ctx, constant.ContextKeyTokenFallbackModels)
	require.True(t, ok)
	assert.Empty(t, fallbacks)

	// A specific channel would bypass the chain's per-model group choice.
	ctxSpecific := newModelChainTokenContext(t, chain)
	err := SetupContextForToken(ctxSpecific, token, "1", "2")
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, ctxSpecific.Writer.Status())
}

// TestSetupContextForTokenKeepsFallbackOnGroupsChain guards the compatibility
// path: a legacy groups-type chain must keep activating the token fallback.
func TestSetupContextForTokenKeepsFallbackOnGroupsChain(t *testing.T) {
	chain := &model.UserChannelChain{
		Id:      91022,
		UserId:  97002,
		ChainId: "grp00101",
		Name:    "groups-chain",
		Type:    model.UserChannelChainTypeGroups,
	}
	require.NoError(t, chain.SetGroupList([]string{"vip", "default"}))
	ctx := newModelChainTokenContext(t, chain)
	token := &model.Token{
		Id:              96002,
		UserId:          97002,
		Key:             "sk-test-2",
		Name:            "groups-chain-token",
		UnlimitedQuota:  true,
		Group:           "chain:" + chain.ChainId,
		FallbackEnabled: true,
		FallbackModels:  `[{"group":"vip","model":"fallback-model"}]`,
	}

	require.NoError(t, SetupContextForToken(ctx, token))
	fallbacks, ok := common.GetContextKeyType[[]setting.TokenFallbackModel](ctx, constant.ContextKeyTokenFallbackModels)
	require.True(t, ok)
	require.Len(t, fallbacks, 1)
	assert.Equal(t, "fallback-model", fallbacks[0].Model)
}

func newPlaygroundChainRequestContext(t *testing.T, chain *model.UserChannelChain, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	service.SetRequestChannelChain(ctx, chain)
	return ctx
}

// TestGetModelRequestRejectsPlaygroundGroupOnModelsChain pins the distributor
// guard: a playground-supplied group would override the per-model group chosen
// by the chain, so it must be rejected before selection.
func TestGetModelRequestRejectsPlaygroundGroupOnModelsChain(t *testing.T) {
	chain := newModelsTypeChain(t, 97003, "mdl00102")
	ctx := newPlaygroundChainRequestContext(t, chain, "{\"model\":\"model-x\",\"group\":\"vip\"}")

	_, _, err := getModelRequest(ctx)
	require.ErrorIs(t, err, errPlaygroundGroupOnModelChain)
}

// TestGetModelRequestKeepsPlaygroundGroupOnGroupsChain guards the legacy
// behavior: groups-type chains still accept the playground group override.
func TestGetModelRequestKeepsPlaygroundGroupOnGroupsChain(t *testing.T) {
	chain := &model.UserChannelChain{
		Id:      91023,
		UserId:  97004,
		ChainId: "grp00102",
		Type:    model.UserChannelChainTypeGroups,
	}
	require.NoError(t, chain.SetGroupList([]string{"vip"}))
	ctx := newPlaygroundChainRequestContext(t, chain, "{\"model\":\"model-x\",\"group\":\"vip\"}")

	request, shouldSelect, err := getModelRequest(ctx)
	require.NoError(t, err)
	require.True(t, shouldSelect)
	require.NotNil(t, request)
	assert.Equal(t, "vip", request.Group)
	assert.Equal(t, "model-x", request.Model)
}

// TestGetModelRequestPreservesTokenGroupWhenPlaygroundGroupEmpty guards the
// no-override path: an empty playground group must not wipe the token group
// (for a models-type chain that group is "chain:<id>").
func TestGetModelRequestPreservesTokenGroupWhenPlaygroundGroupEmpty(t *testing.T) {
	chain := newModelsTypeChain(t, 97005, "mdl00103")
	ctx := newPlaygroundChainRequestContext(t, chain, `{"model":"model-x"}`)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "chain:"+chain.ChainId)

	request, _, err := getModelRequest(ctx)
	require.NoError(t, err)
	require.NotNil(t, request)
	assert.Empty(t, request.Group)
	assert.Equal(t, "chain:"+chain.ChainId, common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))

	plain, _ := gin.CreateTestContext(httptest.NewRecorder())
	plain.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"model-x"}`))
	plain.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(plain, constant.ContextKeyTokenGroup, "default")

	request, _, err = getModelRequest(plain)
	require.NoError(t, err)
	require.NotNil(t, request)
	assert.Empty(t, request.Group)
	assert.Equal(t, "default", common.GetContextKeyString(plain, constant.ContextKeyTokenGroup))
}

// TestSetupContextForTokenRejectsInvalidSelfLoadedChain covers standalone
// callers that do not have an auth-cached snapshot: the DB row must pass the
// same structure/group authorization checks before it becomes routing state.
func TestSetupContextForTokenRejectsInvalidSelfLoadedChain(t *testing.T) {
	require.NoError(t, i18n.Init())
	db := setupMiddlewareChannelChainDB(t)
	chain := model.UserChannelChain{
		UserId:  97101,
		ChainId: "mdlbad01",
		Name:    "bad-chain",
		Type:    model.UserChannelChainTypeGroups,
	}
	require.NoError(t, chain.SetGroupList([]string{"vip", "forbidden-group"}))
	require.NoError(t, db.Create(&chain).Error)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserId, 97101)
	token := &model.Token{
		Id:             96101,
		UserId:         97101,
		Key:            "sk-test-bad-chain",
		Name:           "bad-chain-token",
		UnlimitedQuota: true,
		Group:          "chain:" + chain.ChainId,
	}

	err := SetupContextForToken(ctx, token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unavailable")
	_, cached := service.GetRequestChannelChain(ctx)
	assert.False(t, cached, "an unauthorized chain must not become the request snapshot")
}

// TestSetupContextForTokenReloadsMismatchedSnapshot guards the snapshot reuse
// boundary: a cached chain that belongs to another chain id or user must be
// reloaded from the token's own binding instead of being trusted.
func TestSetupContextForTokenReloadsMismatchedSnapshot(t *testing.T) {
	require.NoError(t, i18n.Init())
	db := setupMiddlewareChannelChainDB(t)
	target := model.UserChannelChain{UserId: 97102, ChainId: "mdlreload1", Name: "reload", Type: model.UserChannelChainTypeGroups}
	require.NoError(t, target.SetGroupList([]string{"vip"}))
	require.NoError(t, db.Create(&target).Error)

	cases := []struct {
		name  string
		chain *model.UserChannelChain
	}{
		{
			name: "cached chain id mismatch",
			chain: func() *model.UserChannelChain {
				stale := &model.UserChannelChain{Id: 91031, UserId: 97102, ChainId: "stale001", Type: model.UserChannelChainTypeGroups}
				require.NoError(t, stale.SetGroupList([]string{"vip"}))
				return stale
			}(),
		},
		{
			name: "cached user id mismatch",
			chain: func() *model.UserChannelChain {
				stale := &model.UserChannelChain{Id: 91032, UserId: 99999, ChainId: target.ChainId, Type: model.UserChannelChainTypeGroups}
				require.NoError(t, stale.SetGroupList([]string{"vip"}))
				return stale
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newModelChainTokenContext(t, tc.chain)
			common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(ctx, constant.ContextKeyUserId, 97102)
			token := &model.Token{Id: 96102, UserId: 97102, Key: "sk-reload", Name: "reload-token", UnlimitedQuota: true, Group: "chain:" + target.ChainId}

			require.NoError(t, SetupContextForToken(ctx, token))
			cached, ok := service.GetRequestChannelChain(ctx)
			require.True(t, ok)
			assert.Equal(t, target.ChainId, cached.ChainId)
			assert.Equal(t, 97102, cached.UserId)
		})
	}
}

func setupMiddlewareChannelChainDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.UserChannelChain{}))
	model.DB = db
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))

	t.Cleanup(func() {
		model.DB = originalDB
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	return db
}

// setupDistributeChainTestDB installs a throwaway database with channels and
// abilities so Distribute performs real channel-cache selection instead of a
// private parser call.
func setupDistributeChainTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalMainDBType := common.MainDatabaseType()
	originalLogDBType := common.LogDatabaseType()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// Channel/Ability queries filter on the reserved "group" column, so the
	// dialect-specific column names must match this throwaway SQLite database.
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.SetDatabaseTypes(originalMainDBType, originalLogDBType)
		model.InitDatabaseColumnNames()
		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createDistributeChainChannel(t *testing.T, db *gorm.DB, id int, group string, models ...string) {
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

type distributeChainCapture struct {
	group     string
	channelId int
}

func runDistributeModelChainRequest(t *testing.T, chain *model.UserChannelChain, mappings []setting.TokenModelMapping, body string) (*httptest.ResponseRecorder, *distributeChainCapture) {
	t.Helper()
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	capture := &distributeChainCapture{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "chain:"+chain.ChainId)
		common.SetContextKey(c, constant.ContextKeyTokenModelMappings, mappings)
		service.SetRequestChannelChain(c, chain)
	})
	router.POST("/v1/chat/completions", Distribute(), func(c *gin.Context) {
		capture.group = common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
		if channelId, ok := common.GetContextKey(c, constant.ContextKeyChannelId); ok {
			capture.channelId, _ = channelId.(int)
		}
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder, capture
}

// TestDistributeModelChainFiltersGroupsAfterTokenModelMapping is the end-to-end
// guard for the models-chain contract: a key-level model mapping rewrites the
// requested alias before selection, and only the chain rows that explicitly
// carry the mapped model are candidates.
func TestDistributeModelChainFiltersGroupsAfterTokenModelMapping(t *testing.T) {
	db := setupDistributeChainTestDB(t)
	createDistributeChainChannel(t, db, 2801, "vip", "model-x")
	createDistributeChainChannel(t, db, 2802, "default", "model-x")
	createDistributeChainChannel(t, db, 2803, "vip", "model-z")
	createDistributeChainChannel(t, db, 2804, "default", "model-z")
	model.InitChannelCache()

	chain := &model.UserChannelChain{Id: 91041, UserId: 97201, ChainId: "mdldist01", Name: "distribute-models", Type: model.UserChannelChainTypeModels}
	require.NoError(t, chain.SetItems([]model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x"}},
		{Group: "default", Models: []string{"model-x"}},
	}))
	mappings := []setting.TokenModelMapping{{SourceModel: "client-alias", TargetModel: "model-x"}}

	recorder, capture := runDistributeModelChainRequest(t, chain, mappings, `{"model":"client-alias","messages":[]}`)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "vip", capture.group)
	assert.Equal(t, 2801, capture.channelId)
}

// TestDistributeModelChainRejectsMappedModelOutsideRows guards the other half:
// after mapping, a model that no row selects fails closed with 503 even when an
// ordinary group has channels for it, instead of silently escaping the chain.
func TestDistributeModelChainRejectsMappedModelOutsideRows(t *testing.T) {
	db := setupDistributeChainTestDB(t)
	createDistributeChainChannel(t, db, 2811, "vip", "model-x", "model-z")
	createDistributeChainChannel(t, db, 2812, "default", "model-z")
	model.InitChannelCache()

	chain := &model.UserChannelChain{Id: 91042, UserId: 97202, ChainId: "mdldist02", Name: "distribute-models", Type: model.UserChannelChainTypeModels}
	require.NoError(t, chain.SetItems([]model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x"}},
	}))
	mappings := []setting.TokenModelMapping{{SourceModel: "client-unselected", TargetModel: "model-z"}}

	recorder, capture := runDistributeModelChainRequest(t, chain, mappings, `{"model":"client-unselected","messages":[]}`)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), "model_not_found")
	// No channel may be selected when the mapped model has no chain row.
	assert.Zero(t, capture.channelId)
	assert.Empty(t, capture.group)
}
