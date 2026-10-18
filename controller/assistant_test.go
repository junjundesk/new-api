package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/assistant_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAssistantOptionsTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	common.OptionMapRWMutex.Lock()
	original := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	setting := assistant_setting.Get()
	originalSetting := *setting
	setting.UserDailyTokenLimit = 0
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = original
		common.OptionMapRWMutex.Unlock()
		*setting = originalSetting
		common.RedisEnabled = previousRedisEnabled
	})
}

func TestGetAssistantConfigDisabledWhenNotConfigured(t *testing.T) {
	setupAssistantOptionsTest(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	GetAssistantConfig(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Enabled bool `json:"enabled"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.False(t, payload.Data.Enabled)
}

func TestGetOptionsHidesAssistantAPIKeysAndExposesConfiguredFlags(t *testing.T) {
	setupAssistantOptionsTest(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[assistant_setting.OptionKeyUserAPIKey] = "sk-user-secret"
	common.OptionMap[assistant_setting.OptionKeyAdminAPIKey] = "sk-admin-secret"
	common.OptionMap[assistant_setting.OptionKeyUserEnabled] = "true"
	common.OptionMapRWMutex.Unlock()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	GetOptions(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	values := map[string]string{}
	for _, option := range payload.Data {
		values[option.Key] = option.Value
	}
	assert.NotContains(t, values, assistant_setting.OptionKeyUserAPIKey)
	assert.NotContains(t, values, assistant_setting.OptionKeyAdminAPIKey)
	assert.Equal(t, "true", values[assistant_setting.OptionKeyUserAPIKeyConfigured])
	assert.Equal(t, "true", values[assistant_setting.OptionKeyAdminAPIKeyConfigured])
}

func TestUpdateOptionAssistantValidation(t *testing.T) {
	setupAssistantOptionsTest(t)
	setting := assistant_setting.Get()
	setting.UserEnabled = false
	setting.UserSource = assistant_setting.SourceDirect
	setting.UserBaseURL = "https://api.openai.com/v1"
	setting.UserAPIKey = "sk-existing"
	setting.UserModel = "gpt-test"

	postOption := func(t *testing.T, key string, value any) map[string]any {
		t.Helper()
		body, err := common.Marshal(map[string]any{"key": key, "value": value})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("id", 1)
		ctx.Set("username", "root")
		ctx.Set("role", common.RoleRootUser)
		UpdateOption(ctx)
		require.Equal(t, http.StatusOK, recorder.Code)
		payload := map[string]any{}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload
	}

	payload := postOption(t, assistant_setting.OptionKeyUserAPIKey, "")
	assert.Equal(t, true, payload["success"])
	assert.Equal(t, "sk-existing", assistant_setting.Get().UserAPIKey)

	payload = postOption(t, assistant_setting.OptionKeyUserBaseURL, "ftp://bad")
	assert.Equal(t, false, payload["success"])

	setting.UserAPIKey = ""
	payload = postOption(t, assistant_setting.OptionKeyUserEnabled, true)
	assert.Equal(t, false, payload["success"])
}

func TestGetStatusIncludesAssistantEnabled(t *testing.T) {
	setupAssistantOptionsTest(t)
	setting := assistant_setting.Get()
	setting.UserEnabled = true
	setting.UserSource = assistant_setting.SourceDirect
	setting.UserBaseURL = "https://api.openai.com/v1"
	setting.UserAPIKey = "sk-test"
	setting.UserModel = "gpt-test"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	GetStatus(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			AssistantEnabled bool `json:"assistant_enabled"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.True(t, payload.Data.AssistantEnabled)

	setting.Enabled = false
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	GetStatus(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	payload = struct {
		Success bool `json:"success"`
		Data    struct {
			AssistantEnabled bool `json:"assistant_enabled"`
		} `json:"data"`
	}{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.False(t, payload.Data.AssistantEnabled)
}

func TestUpdateOptionAssistantKeyThenEnable(t *testing.T) {
	setupAssistantOptionsTest(t)
	originalDB := model.DB
	dsn := fmt.Sprintf("file:assistant_option_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	originalLogDB := model.LOG_DB
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.User{}, &model.Log{}))
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})

	setting := assistant_setting.Get()
	setting.UserAPIKey = ""
	setting.UserEnabled = false
	setting.UserSource = assistant_setting.SourceDirect
	setting.UserBaseURL = "https://api.openai.com/v1"
	setting.UserModel = "gpt-test"

	postOption := func(t *testing.T, key string, value any) map[string]any {
		t.Helper()
		body, err := common.Marshal(map[string]any{"key": key, "value": value})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("id", 1)
		ctx.Set("username", "root")
		ctx.Set("role", common.RoleRootUser)
		UpdateOption(ctx)
		require.Equal(t, http.StatusOK, recorder.Code)
		payload := map[string]any{}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		require.Equal(t, true, payload["success"], payload["message"])
		return payload
	}

	postOption(t, assistant_setting.OptionKeyUserAPIKey, "sk-test")
	assert.Equal(t, "sk-test", assistant_setting.Get().UserAPIKey)
	postOption(t, assistant_setting.OptionKeyUserEnabled, true)
	assert.True(t, assistant_setting.Get().UserEnabled)
	assert.True(t, assistant_setting.IsAvailable(assistant_setting.ProfileUser))
}

func TestUpdateOptionAssistantSafetySwitches(t *testing.T) {
	setupAssistantOptionsTest(t)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	dsn := fmt.Sprintf("file:assistant_safety_option_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Log{}, &model.User{}))
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	postOption := func(t *testing.T, key string, value any) map[string]any {
		t.Helper()
		body, err := common.Marshal(map[string]any{"key": key, "value": value})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("id", 1)
		ctx.Set("username", "root")
		ctx.Set("role", common.RoleRootUser)
		UpdateOption(ctx)
		require.Equal(t, http.StatusOK, recorder.Code)
		payload := map[string]any{}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload
	}
	payload := postOption(t, assistant_setting.OptionKeyConfirmWrites, false)
	assert.Equal(t, true, payload["success"])
	payload = postOption(t, assistant_setting.OptionKeyAdminDBToolsEnabled, false)
	assert.Equal(t, true, payload["success"])
	assert.Equal(t, "false", common.OptionMap[assistant_setting.OptionKeyConfirmWrites])
	assert.Equal(t, "false", common.OptionMap[assistant_setting.OptionKeyAdminDBToolsEnabled])
}
