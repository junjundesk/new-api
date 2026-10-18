package assistant

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupToolTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.LogConsumeEnabled = true
	common.QuotaPerUnit = 500000
	model.InitDatabaseColumnNames()

	dsn := fmt.Sprintf("file:assistant_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedAssistantUser(t *testing.T, db *gorm.DB, id int, username string) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		Id:           id,
		Username:     username,
		Password:     "password",
		DisplayName:  username,
		Role:         common.RoleCommonUser,
		Status:       common.UserStatusEnabled,
		Group:        "default",
		AffCode:      fmt.Sprintf("aff-%d", id),
		Quota:        1000,
		UsedQuota:    200,
		RequestCount: 5,
	}).Error)
}

func seedAssistantLog(t *testing.T, db *gorm.DB, userID int, modelName, tokenName, content string, quota int) {
	t.Helper()
	username, err := model.GetUsernameById(userID, false)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Log{
		UserId:           userID,
		Username:         username,
		CreatedAt:        time.Now().Unix(),
		Type:             model.LogTypeConsume,
		Content:          content,
		ModelName:        modelName,
		TokenName:        tokenName,
		Group:            "default",
		Quota:            quota,
		PromptTokens:     10,
		CompletionTokens: 20,
		UseTime:          1,
	}).Error)
}

func TestCreateAPIKeyToolReturnsMaskedKeyOnly(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	executor := NewDefaultToolExecutor(false)
	result := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "create_api_key",
		Arguments: "{\"name\":\"assistant-key\"}",
	}, nil)
	require.True(t, result.OK, result.Error)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	id, ok := data["id"].(int)
	require.True(t, ok)
	masked, ok := data["masked_key"].(string)
	require.True(t, ok)
	require.NotEmpty(t, masked)

	token, err := model.GetTokenByIds(id, 1)
	require.NoError(t, err)
	assert.Equal(t, "assistant-key", token.Name)
	assert.Equal(t, 1, token.UserId)
	assert.True(t, token.UnlimitedQuota)
	assert.NotEqual(t, token.Key, masked)
	assert.False(t, strings.Contains(masked, token.Key))
	payload, err := common.Marshal(result.Data)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), token.Key)
}

func TestUsageToolsAreScopedToCurrentUser(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	seedAssistantLog(t, db, 1, "model-a", "key-a", "alice consume", 100)
	seedAssistantLog(t, db, 2, "model-b-secret", "key-b", "bob consume", 999)

	executor := NewDefaultToolExecutor(false)
	summary := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "get_usage_summary",
		Arguments: "{\"days\":7,\"group_by\":\"model\"}",
	}, nil)
	require.True(t, summary.OK, summary.Error)
	payload, err := common.Marshal(summary.Data)
	require.NoError(t, err)
	assert.Contains(t, string(payload), "model-a")
	assert.NotContains(t, string(payload), "model-b-secret")

	logs := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "list_usage_logs",
		Arguments: "{\"days\":7}",
	}, nil)
	require.True(t, logs.OK, logs.Error)
	logPayload, err := common.Marshal(logs.Data)
	require.NoError(t, err)
	assert.Contains(t, string(logPayload), "alice consume")
	assert.NotContains(t, string(logPayload), "bob consume")

	require.NoError(t, db.Create(&model.Token{UserId: 1, Key: "sk-alice", Name: "alice-key", UnlimitedQuota: true, ExpiredTime: -1}).Error)
	require.NoError(t, db.Create(&model.Token{UserId: 2, Key: "sk-bob", Name: "bob-key", UnlimitedQuota: true, ExpiredTime: -1}).Error)
	keys := executor.Execute(context.Background(), 1, ToolCall{Name: "list_api_keys", Arguments: "{}"}, nil)
	require.True(t, keys.OK, keys.Error)
	keysPayload, err := common.Marshal(keys.Data)
	require.NoError(t, err)
	assert.Contains(t, string(keysPayload), "alice-key")
	assert.NotContains(t, string(keysPayload), "bob-key")
	assert.NotContains(t, string(keysPayload), "sk-alice")
	assert.NotContains(t, string(keysPayload), "sk-bob")
}

func TestSetAPIKeyStatusRejectsOtherUsersToken(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	require.NoError(t, db.Create(&model.Token{UserId: 2, Key: "sk-bob", Name: "bob-key", UnlimitedQuota: true, ExpiredTime: -1, Status: common.TokenStatusEnabled}).Error)
	bobToken, err := model.GetTokenByKey("sk-bob", true)
	require.NoError(t, err)

	executor := NewDefaultToolExecutor(false)
	result := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "set_api_key_status",
		Arguments: fmt.Sprintf("{\"id\":%d,\"enabled\":false}", bobToken.Id),
	}, nil)
	assert.False(t, result.OK)

	require.NoError(t, db.Create(&model.Token{UserId: 1, Key: "sk-alice", Name: "alice-key", UnlimitedQuota: true, ExpiredTime: -1, Status: common.TokenStatusEnabled}).Error)
	aliceToken, err := model.GetTokenByKey("sk-alice", true)
	require.NoError(t, err)
	result = executor.Execute(context.Background(), 1, ToolCall{
		Name:      "set_api_key_status",
		Arguments: fmt.Sprintf("{\"id\":%d,\"enabled\":false}", aliceToken.Id),
	}, nil)
	require.True(t, result.OK, result.Error)
	updated, err := model.GetTokenByIds(aliceToken.Id, 1)
	require.NoError(t, err)
	assert.Equal(t, common.TokenStatusDisabled, updated.Status)
}

func TestToolArgumentBounds(t *testing.T) {
	setupToolTestDB(t)
	executor := NewDefaultToolExecutor(false)
	tests := []struct {
		name      string
		arguments string
	}{
		{name: "days zero", arguments: "{\"days\":0}"},
		{name: "days too large", arguments: "{\"days\":32}"},
		{name: "limit too large", arguments: "{\"limit\":21}"},
		{name: "bad group by", arguments: "{\"group_by\":\"unknown\"}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.Execute(context.Background(), 1, ToolCall{Name: "get_usage_summary", Arguments: tt.arguments}, nil)
			assert.False(t, result.OK)
		})
	}
	missingEnabled := executor.Execute(context.Background(), 1, ToolCall{Name: "set_api_key_status", Arguments: "{\"id\":1}"}, nil)
	assert.False(t, missingEnabled.OK)
	emptyName := executor.Execute(context.Background(), 1, ToolCall{Name: "create_api_key", Arguments: "{\"name\":\"\"}"}, nil)
	assert.False(t, emptyName.OK)
}

func TestGetAccountSummaryIncludesQuota(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantLog(t, db, 1, "model-a", "key-a", "consume", 100)
	executor := NewDefaultToolExecutor(false)
	result := executor.Execute(context.Background(), 1, ToolCall{Name: "get_account_summary"}, nil)
	require.True(t, result.OK, result.Error)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1000, data["quota"])
	assert.Equal(t, 200, data["used_quota"])
	payload, err := common.Marshal(result.Data)
	require.NoError(t, err)
	assert.Contains(t, string(payload), "\"count\":1")
}

func TestTokenToolsAreNotCappedPerTurn(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	executor := NewDefaultToolExecutor(false)
	for index := 0; index < 2; index++ {
		result := executor.Execute(context.Background(), 1, ToolCall{
			Name:      "create_api_key",
			Arguments: fmt.Sprintf("{\"name\":\"key-%d\"}", index),
		}, nil)
		require.True(t, result.OK, result.Error)
	}
	for index := 0; index < 4; index++ {
		require.NoError(t, db.Create(&model.Token{
			UserId:         1,
			Key:            fmt.Sprintf("sk-status-%d", index),
			Name:           fmt.Sprintf("status-key-%d", index),
			UnlimitedQuota: true,
			ExpiredTime:    -1,
			Status:         common.TokenStatusEnabled,
		}).Error)
	}
	tokens, err := model.GetAllUserTokens(1, 0, 20)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tokens), 4)
	for _, token := range tokens {
		result := executor.Execute(context.Background(), 1, ToolCall{
			Name:      "set_api_key_status",
			Arguments: fmt.Sprintf("{\"id\":%d,\"enabled\":false}", token.Id),
		}, nil)
		require.True(t, result.OK, result.Error)
	}
}

func TestUserScopeCannotTargetOtherUsers(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	executor := NewDefaultToolExecutor(false)
	result := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "get_account_summary",
		Arguments: "{\"user_id\":2}",
	}, nil)
	require.True(t, result.OK, result.Error)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", data["username"])
	users := executor.Execute(context.Background(), 1, ToolCall{Name: "list_users", Arguments: "{}"}, nil)
	assert.False(t, users.OK)
}

func TestAdminScopeCanQueryAndManageAllUsers(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	seedAssistantLog(t, db, 1, "model-a", "key-a", "alice consume", 100)
	seedAssistantLog(t, db, 2, "model-b", "key-b", "bob consume", 200)
	require.NoError(t, db.Create(&model.Token{UserId: 2, Key: "sk-bob", Name: "bob-key", UnlimitedQuota: true, ExpiredTime: -1, Status: common.TokenStatusEnabled}).Error)
	bobToken, err := model.GetTokenByKey("sk-bob", true)
	require.NoError(t, err)

	executor := NewDefaultToolExecutor(true)

	summary := executor.Execute(context.Background(), 1, ToolCall{Name: "get_account_summary", Arguments: "{\"user_id\":2}"}, nil)
	require.True(t, summary.OK, summary.Error)
	summaryData := summary.Data.(map[string]any)
	assert.Equal(t, "bob", summaryData["username"])

	platform := executor.Execute(context.Background(), 1, ToolCall{Name: "get_account_summary", Arguments: "{}"}, nil)
	require.True(t, platform.OK, platform.Error)
	platformData := platform.Data.(map[string]any)
	assert.EqualValues(t, 2, platformData["user_count"])

	usage := executor.Execute(context.Background(), 1, ToolCall{Name: "get_usage_summary", Arguments: "{\"group_by\":\"user\"}"}, nil)
	require.True(t, usage.OK, usage.Error)
	usageData := usage.Data.(map[string]any)
	assert.Equal(t, "all_users", usageData["scope"])
	assert.Len(t, usageData["rows"], 2)

	allLogs := executor.Execute(context.Background(), 1, ToolCall{Name: "list_usage_logs", Arguments: "{}"}, nil)
	require.True(t, allLogs.OK, allLogs.Error)
	assert.Len(t, allLogs.Data.(map[string]any)["items"], 2)

	users := executor.Execute(context.Background(), 1, ToolCall{Name: "list_users", Arguments: "{\"keyword\":\"bob\"}"}, nil)
	require.True(t, users.OK, users.Error)
	userItems := users.Data.(map[string]any)["items"].([]map[string]any)
	require.Len(t, userItems, 1)
	assert.Equal(t, "bob", userItems[0]["username"])

	logs := executor.Execute(context.Background(), 1, ToolCall{Name: "list_usage_logs", Arguments: "{\"user_id\":2}"}, nil)
	require.True(t, logs.OK, logs.Error)
	logData := logs.Data.(map[string]any)
	logItems := logData["items"].([]map[string]any)
	require.Len(t, logItems, 1)
	assert.Equal(t, "bob consume", logItems[0]["content"])

	keys := executor.Execute(context.Background(), 1, ToolCall{Name: "list_api_keys", Arguments: "{\"user_id\":2}"}, nil)
	require.True(t, keys.OK, keys.Error)
	keyItems := keys.Data.(map[string]any)["items"].([]map[string]any)
	require.Len(t, keyItems, 1)
	assert.Equal(t, "bob-key", keyItems[0]["name"])

	created := executor.Execute(context.Background(), 1, ToolCall{Name: "create_api_key", Arguments: "{\"user_id\":2,\"name\":\"admin-created\"}"}, nil)
	require.True(t, created.OK, created.Error)
	createdData := created.Data.(map[string]any)
	tokenID, ok := createdData["id"].(int)
	require.True(t, ok)
	token, err := model.GetTokenById(tokenID)
	require.NoError(t, err)
	assert.Equal(t, 2, token.UserId)
	assert.Equal(t, "admin-created", token.Name)

	disable := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "set_api_key_status",
		Arguments: fmt.Sprintf("{\"id\":%d,\"enabled\":false}", bobToken.Id),
	}, nil)
	require.True(t, disable.OK, disable.Error)
	updated, err := model.GetTokenById(bobToken.Id)
	require.NoError(t, err)
	assert.Equal(t, common.TokenStatusDisabled, updated.Status)
}
