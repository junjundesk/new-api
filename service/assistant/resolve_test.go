package assistant

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/assistant_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupResolveTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	model.InitDatabaseColumnNames()
	originalDB := model.DB
	dsn := fmt.Sprintf("file:assistant_resolve_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	setting := assistant_setting.Get()
	original := *setting
	t.Cleanup(func() { *setting = original })
	return db
}

func TestResolveConfigFromDisabledChannel(t *testing.T) {
	db := setupResolveTestDB(t)
	baseURL := "http://upstream.test"
	channel := &model.Channel{
		Id:      42,
		Type:    constant.ChannelTypeOpenAIResponses,
		Key:     "sk-channel",
		Name:    "disabled-channel",
		Status:  common.ChannelStatusManuallyDisabled,
		BaseURL: &baseURL,
		Models:  "gpt-test,gpt-other",
		Group:   "default",
	}
	require.NoError(t, db.Create(channel).Error)

	cfg, err := ResolveConfig(assistant_setting.Profile{
		Name:         assistant_setting.ProfileUser,
		Source:       assistant_setting.SourceChannel,
		ChannelID:    42,
		ChannelModel: "gpt-test",
	})
	require.NoError(t, err)
	assert.Equal(t, "http://upstream.test/v1", cfg.BaseURL)
	assert.Equal(t, "sk-channel", cfg.APIKey)
	assert.Equal(t, "gpt-test", cfg.Model)

	overridden, err := ResolveConfig(assistant_setting.Profile{
		Name:         assistant_setting.ProfileUser,
		Source:       assistant_setting.SourceChannel,
		ChannelID:    42,
		ChannelModel: "gpt-other",
		APIKey:       "sk-profile-override",
	})
	require.NoError(t, err)
	assert.Equal(t, "sk-profile-override", overridden.APIKey)
	assert.Equal(t, "gpt-other", overridden.Model)
}

func TestResolveConfigDirectProfile(t *testing.T) {
	setupResolveTestDB(t)
	cfg, err := ResolveConfig(assistant_setting.Profile{
		Name:            assistant_setting.ProfileAdmin,
		Source:          assistant_setting.SourceDirect,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "sk-admin",
		Model:           "gpt-admin",
		ReasoningEffort: "xhigh",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1", cfg.BaseURL)
	assert.Equal(t, "sk-admin", cfg.APIKey)
	assert.Equal(t, "gpt-admin", cfg.Model)
	assert.Equal(t, "xhigh", cfg.ReasoningEffort)

	defaulted, err := ResolveConfig(assistant_setting.Profile{
		Source:  assistant_setting.SourceDirect,
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "sk-admin",
		Model:   "gpt-admin",
	})
	require.NoError(t, err)
	assert.Equal(t, "high", defaulted.ReasoningEffort)

	_, err = ResolveConfig(assistant_setting.Profile{
		Source:          assistant_setting.SourceDirect,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "sk-admin",
		Model:           "gpt-admin",
		ReasoningEffort: "ultra",
	})
	require.Error(t, err)
}

func TestResolveConfigRejectsMissingChannelAndAzure(t *testing.T) {
	db := setupResolveTestDB(t)
	_, err := ResolveConfig(assistant_setting.Profile{
		Source:       assistant_setting.SourceChannel,
		ChannelID:    999,
		ChannelModel: "gpt-test",
	})
	require.Error(t, err)

	azureBase := "https://example.openai.azure.com"
	azure := &model.Channel{
		Id:      43,
		Type:    constant.ChannelTypeAzure,
		Key:     "sk-azure",
		Name:    "azure",
		Status:  common.ChannelStatusEnabled,
		BaseURL: &azureBase,
		Models:  "gpt-test",
	}
	require.NoError(t, db.Create(azure).Error)
	_, err = ResolveConfig(assistant_setting.Profile{
		Source:       assistant_setting.SourceChannel,
		ChannelID:    43,
		ChannelModel: "gpt-test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Azure")
}

func TestResolveConfigAppliesChannelModelMapping(t *testing.T) {
	db := setupResolveTestDB(t)
	baseURL := "https://api.openai.com"
	mapping := "{\"gpt-client\":[\"gpt-upstream\"]}"
	channel := &model.Channel{
		Id:           44,
		Type:         constant.ChannelTypeOpenAI,
		Key:          "sk-channel",
		Name:         "mapped",
		Status:       common.ChannelStatusEnabled,
		BaseURL:      &baseURL,
		Models:       "gpt-client",
		ModelMapping: &mapping,
	}
	require.NoError(t, db.Create(channel).Error)
	cfg, err := ResolveConfig(assistant_setting.Profile{
		Source:       assistant_setting.SourceChannel,
		ChannelID:    44,
		ChannelModel: "gpt-client",
	})
	require.NoError(t, err)
	assert.Equal(t, "gpt-upstream", cfg.Model)
}
