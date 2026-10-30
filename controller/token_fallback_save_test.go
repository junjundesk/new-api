package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedAbility(t *testing.T, db *gorm.DB, group string, modelName string, channelID int) {
	t.Helper()
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
	}).Error)
}

func seedTokenFallbackDraft(t *testing.T, db *gorm.DB, token *model.Token, fallbacks []setting.TokenFallbackModel) {
	t.Helper()
	require.NoError(t, token.SetFallbackModels(fallbacks))
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", token.Id).Update("fallback_models", token.FallbackModels).Error)
}

func updateTokenWithFallbacks(t *testing.T, tokenID int, group string, enabled bool, fallbacks []map[string]any) tokenAPIResponse {
	t.Helper()
	body := map[string]any{
		"id":                   tokenID,
		"name":                 "fallback-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                group,
		"cross_group_retry":    false,
		"fallback_enabled":     enabled,
		"fallback_models":      fallbacks,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	ctx.Set("group", "default")
	UpdateToken(ctx)
	return decodeAPIResponse(t, recorder)
}

func TestUpdateTokenKeepsDisabledFallbackWithUnavailableSourceModel(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	seedAbility(t, db, "vip", "claude-3", 1)

	token := seedToken(t, db, 1, "fallback-token", "draft1234token5678")
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", token.Id).Update("group", "vip").Error)
	seedTokenFallbackDraft(t, db, token, []setting.TokenFallbackModel{
		{SourceModel: "gpt-4", Group: "default", Model: "gpt-4"},
	})

	storedFallback := setting.TokenFallbackModel{SourceModel: "gpt-4", Group: "default", Model: "gpt-4"}

	response := updateTokenWithFallbacks(t, token.Id, "vip", false, []map[string]any{
		{"source_model": "gpt-4", "group": "default", "model": "gpt-4"},
	})
	require.True(t, response.Success, response.Message)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	require.False(t, stored.FallbackEnabled)
	storedFallbacks, err := stored.GetFallbackModels()
	require.NoError(t, err)
	require.Equal(t, []setting.TokenFallbackModel{storedFallback}, storedFallbacks)
}

func TestUpdateTokenKeepsIncompleteDisabledFallbackDraft(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()

	token := seedToken(t, db, 1, "fallback-token", "draft5678token1234")

	response := updateTokenWithFallbacks(t, token.Id, "default", false, []map[string]any{
		{"source_model": "", "group": "", "model": ""},
	})
	require.True(t, response.Success, response.Message)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	storedFallbacks, err := stored.GetFallbackModels()
	require.NoError(t, err)
	require.Equal(t, []setting.TokenFallbackModel{{}}, storedFallbacks)
}

func TestUpdateTokenAllowsGroupSwitchWithUnchangedEnabledFallback(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	seedAbility(t, db, "default", "gpt-4", 1)
	seedAbility(t, db, "vip", "claude-3", 1)

	token := seedToken(t, db, 1, "fallback-token", "switch1234token5678")
	require.NoError(t, token.SetFallbackModels([]setting.TokenFallbackModel{
		{SourceModel: "gpt-4", Group: "default", Model: "gpt-4"},
	}))
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]any{
		"fallback_enabled": true,
		"fallback_models":  token.FallbackModels,
	}).Error)

	// The same enabled fallback list is submitted after the key's group moves to
	// a group that no longer serves its source model. Unrelated edits must not be
	// blocked by the stored configuration.
	response := updateTokenWithFallbacks(t, token.Id, "vip", true, []map[string]any{
		{"source_model": "gpt-4", "group": "default", "model": "gpt-4"},
	})
	require.True(t, response.Success, response.Message)

	var stored model.Token
	require.NoError(t, db.First(&stored, token.Id).Error)
	require.Equal(t, "vip", stored.Group)
	require.True(t, stored.FallbackEnabled)
	storedFallbacks, err := stored.GetFallbackModels()
	require.NoError(t, err)
	require.Equal(t, []setting.TokenFallbackModel{
		{SourceModel: "gpt-4", Group: "default", Model: "gpt-4"},
	}, storedFallbacks)
}

func TestUpdateTokenRejectsChangedEnabledFallbackWithUnavailableSourceModel(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	seedAbility(t, db, "vip", "claude-3", 1)

	token := seedToken(t, db, 1, "fallback-token", "changed1234token567")
	require.NoError(t, token.SetFallbackModels([]setting.TokenFallbackModel{
		{SourceModel: "gpt-4", Group: "default", Model: "gpt-4"},
	}))
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]any{
		"fallback_enabled": true,
		"fallback_models":  token.FallbackModels,
		"group":            "vip",
	}).Error)

	response := updateTokenWithFallbacks(t, token.Id, "vip", true, []map[string]any{
		{"source_model": "gpt-4", "group": "default", "model": "gpt-4"},
		{"source_model": "", "group": "vip", "model": "claude-3"},
	})
	require.False(t, response.Success)
	require.Contains(t, response.Message, "source model gpt-4 is unavailable")
}

func TestUpdateTokenRejectsEnabledFallbackWithUnavailableSourceModel(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	seedAbility(t, db, "vip", "claude-3", 1)

	token := seedToken(t, db, 1, "fallback-token", "enabled1234token56")
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", token.Id).Update("group", "vip").Error)

	response := updateTokenWithFallbacks(t, token.Id, "vip", true, []map[string]any{
		{"source_model": "gpt-4", "group": "vip", "model": "claude-3"},
	})
	require.False(t, response.Success)
	require.Contains(t, response.Message, "source model gpt-4 is unavailable")
}
