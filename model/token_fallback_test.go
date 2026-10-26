package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenFallbackModelsJSONRoundTrip(t *testing.T) {
	want := []setting.TokenFallbackModel{
		{SourceModel: "gpt-5", Group: "vip", Model: "claude-sonnet-4-5"},
		{Group: "default", Model: "gpt-4.1-mini"},
	}
	token := &Token{}
	require.NoError(t, token.SetFallbackModels(want))
	assert.NotEmpty(t, token.FallbackModels)

	got, err := token.GetFallbackModels()
	require.NoError(t, err)
	assert.Equal(t, want, got)

	require.NoError(t, token.SetFallbackModels(nil))
	assert.Empty(t, token.FallbackModels, "clearing a key's fallbacks must remove the persisted JSON")
	got, err = token.GetFallbackModels()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestTokenGetFallbackModelsRejectsMalformedJSON(t *testing.T) {
	token := &Token{FallbackModels: `[{"group":"vip","model":`}
	got, err := token.GetFallbackModels()
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestTokenUpdatePersistsFallbackFieldsIncludingDisabled(t *testing.T) {
	previousDB := DB
	previousMainType := common.MainDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	initCol()
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousMainType)
		common.RedisEnabled = previousRedisEnabled
		initCol()
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	require.NoError(t, db.AutoMigrate(&Token{}))

	token := &Token{
		Key:             "fallback-update-token",
		Name:            "fallback update",
		Status:          common.TokenStatusEnabled,
		ExpiredTime:     -1,
		RemainQuota:     100,
		FallbackEnabled: true,
	}
	require.NoError(t, token.SetFallbackModels([]setting.TokenFallbackModel{
		{SourceModel: "gpt-5", Group: "vip", Model: "claude-sonnet-4-5"},
	}))
	require.NoError(t, token.Insert())

	token.FallbackEnabled = false
	require.NoError(t, token.SetFallbackModels(nil))
	require.NoError(t, token.Update())

	var stored Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	assert.False(t, stored.FallbackEnabled, "Update must persist disabling the feature")
	assert.Empty(t, stored.FallbackModels, "Update must persist clearing the configured list")
}

func TestTokenFallbackFieldsSurviveRedisCacheRoundTrip(t *testing.T) {
	useUserCacheMiniRedis(t)
	want := []setting.TokenFallbackModel{
		{SourceModel: "gpt-5", Group: "vip", Model: "claude-sonnet-4-5"},
		{Group: "default", Model: "gpt-4.1-mini"},
	}
	token := Token{Id: 9201, UserId: 92, Key: "fallback-cache-token", Name: "fallback cache", FallbackEnabled: true}
	require.NoError(t, token.SetFallbackModels(want))
	require.NoError(t, cacheSetToken(token))

	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.True(t, cached.FallbackEnabled)
	got, err := cached.GetFallbackModels()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
