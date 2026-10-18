package controller

import (
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

func openAssistantProfileTestDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	dsn := fmt.Sprintf("file:assistant_profile_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(models...))
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetAdminAssistantConfig(t *testing.T) {
	setupAssistantOptionsTest(t)
	setting := assistant_setting.Get()
	setting.AdminEnabled = true
	setting.AdminSource = assistant_setting.SourceDirect
	setting.AdminBaseURL = "https://api.openai.com/v1"
	setting.AdminAPIKey = "sk-admin-secret"
	setting.AdminModel = "gpt-admin"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	GetAdminAssistantConfig(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Enabled bool   `json:"enabled"`
			Model   string `json:"model"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.True(t, payload.Data.Enabled)
	assert.Equal(t, "gpt-admin", payload.Data.Model)
	assert.NotContains(t, recorder.Body.String(), "sk-admin-secret")
}

func TestListAssistantChannelsIncludesDisabledChannelsAndNoKeys(t *testing.T) {
	setupAssistantOptionsTest(t)
	db := openAssistantProfileTestDB(t, &model.Channel{})
	baseURL := "https://api.openai.com"
	channel := &model.Channel{
		Id:      77,
		Type:    1,
		Key:     "sk-channel-secret",
		Name:    "disabled-channel",
		Status:  common.ChannelStatusManuallyDisabled,
		BaseURL: &baseURL,
		Models:  "gpt-test, gpt-other",
		Group:   "default",
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ListAssistantChannels(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ID     int      `json:"id"`
			Name   string   `json:"name"`
			Status int      `json:"status"`
			Models []string `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data, 1)
	assert.Equal(t, 77, payload.Data[0].ID)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, payload.Data[0].Status)
	assert.Equal(t, []string{"gpt-test", "gpt-other"}, payload.Data[0].Models)
	assert.NotContains(t, recorder.Body.String(), "sk-channel-secret")
}

func TestClearAssistantKeyOnlyAffectsSelectedProfile(t *testing.T) {
	setupAssistantOptionsTest(t)
	db := openAssistantProfileTestDB(t, &model.Option{}, &model.User{}, &model.Log{})
	_ = db
	setting := assistant_setting.Get()
	setting.UserEnabled = true
	setting.UserAPIKey = "sk-user"
	setting.AdminEnabled = true
	setting.AdminAPIKey = "sk-admin"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/assistant/admin/key?profile=admin", nil)
	ctx.Set("id", 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)
	ClearAssistantKey(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.Equal(t, "", assistant_setting.Get().AdminAPIKey)
	assert.False(t, assistant_setting.Get().AdminEnabled)
	assert.Equal(t, "sk-user", assistant_setting.Get().UserAPIKey)
	assert.True(t, assistant_setting.Get().UserEnabled)
}
