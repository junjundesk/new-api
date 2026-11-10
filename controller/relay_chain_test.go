package controller

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupRelayChainTestDB installs a throwaway database so the retry budget reads
// the real per-group enabled model directory instead of degrading when no data
// exists.
func setupRelayChainTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalRetryTimes := common.RetryTimes
	originalMainDBType := common.MainDatabaseType()
	originalLogDBType := common.LogDatabaseType()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// The Ability queries filter on the reserved "group" column, so the
	// dialect-specific column names must match this throwaway SQLite database.
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	model.DB = db

	t.Cleanup(func() {
		model.DB = originalDB
		common.RetryTimes = originalRetryTimes
		common.SetDatabaseTypes(originalMainDBType, originalLogDBType)
		model.InitDatabaseColumnNames()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createRelayChainTestAbility(t *testing.T, db *gorm.DB, channelId int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channelId,
		Enabled:   true,
		Priority:  &priority,
		Weight:    100,
	}).Error)
}

// TestRelayRetryLimitCountsMatchedModelChainRows ensures the retry budget of a
// models-type chain covers exactly the rows that currently provide the request
// model, so an unknown model fails fast instead of burning the whole budget.
func TestRelayRetryLimitCountsMatchedModelChainRows(t *testing.T) {
	db := setupRelayChainTestDB(t)
	common.RetryTimes = 1
	createRelayChainTestAbility(t, db, 91011, "vip", "model-x")
	createRelayChainTestAbility(t, db, 91012, "vip", "model-y")
	createRelayChainTestAbility(t, db, 91013, "default", "model-x")

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	chain := &model.UserChannelChain{
		Id:      91011,
		ChainId: "mdl00002",
		Type:    model.UserChannelChainTypeModels,
	}
	require.NoError(t, chain.SetItems([]model.UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x", "model-y"}},
		{Group: "default", Models: []string{"model-x"}},
	}))
	service.SetRequestChannelChain(ctx, chain)

	// model-x matches both rows: 2 candidates * (RetryTimes + 1) = 4.
	assert.Equal(t, 4, relayRetryLimit(ctx, &relaycommon.RelayInfo{OriginModelName: "model-x"}))
	// model-y matches one row: 1 candidate * (RetryTimes + 1) = 2.
	assert.Equal(t, 2, relayRetryLimit(ctx, &relaycommon.RelayInfo{OriginModelName: "model-y"}))
	// unknown model: resolution fails closed, keep the plain retry budget and
	// let selection return the model-not-available error.
	assert.Equal(t, 1, relayRetryLimit(ctx, &relaycommon.RelayInfo{OriginModelName: "model-unknown"}))
}

// TestRelayRetryLimitKeepsGroupsChainBudget is the regression guard for the
// legacy groups-type chain: its entire group list still supplies one budget per
// group regardless of the request model.
func TestRelayRetryLimitKeepsGroupsChainBudget(t *testing.T) {
	setupRelayChainTestDB(t)
	common.RetryTimes = 0

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	chain := &model.UserChannelChain{
		Id:      91012,
		ChainId: "grp00002",
		Type:    model.UserChannelChainTypeGroups,
	}
	require.NoError(t, chain.SetGroupList([]string{"vip", "default"}))

	service.SetRequestChannelChain(ctx, chain)
	assert.Equal(t, 2, relayRetryLimit(ctx, &relaycommon.RelayInfo{OriginModelName: "any-model"}))
}
