package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/assistant"
	"github.com/QuantumNous/new-api/setting/assistant_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAssistantChatStreamsTextOverSSE(t *testing.T) {
	setupAssistantOptionsTest(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/responses", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"id\":\"resp_1\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}],\"usage\":{\"input_tokens\":42,\"output_tokens\":5,\"total_tokens\":47}}")
	}))
	defer upstream.Close()

	setting := assistant_setting.Get()
	setting.UserEnabled = true
	setting.UserSource = assistant_setting.SourceDirect
	setting.UserBaseURL = upstream.URL
	setting.UserAPIKey = "sk-test"
	setting.UserModel = "gpt-test"
	setting.UserProtocol = assistant_setting.ProtocolResponses
	setting.RequestTimeoutSeconds = 5
	setting.UserContextWindow = 1000000
	setting.UserMaxOutputTokens = 100

	body, err := common.Marshal(map[string]any{"message": "hi", "history": []any{}})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assistant/chat", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 9999)

	AssistantChat(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")

	events := make([]assistant.Event, 0)
	for _, block := range strings.Split(recorder.Body.String(), "\n\n") {
		block = strings.TrimSpace(block)
		if !strings.HasPrefix(block, "data: ") {
			continue
		}
		event := assistant.Event{}
		require.NoError(t, common.Unmarshal([]byte(strings.TrimPrefix(block, "data: ")), &event))
		events = append(events, event)
	}
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	assert.Contains(t, types, "meta")
	assert.Contains(t, types, "delta")
	assert.Contains(t, types, "done")
	for _, event := range events {
		if event.Type == "delta" {
			assert.Equal(t, "Hello", event.Text)
		}
		if event.Type == "done" {
			assert.Equal(t, 42, event.ContextTokens)
			assert.Equal(t, 1000000, event.ContextWindow)
			require.NotNil(t, event.Usage)
			assert.Equal(t, 47, event.Usage.TotalTokens)
		}
	}
}

func TestAssistantChatErrorEventUsesMessageField(t *testing.T) {
	setupAssistantOptionsTest(t)
	setting := assistant_setting.Get()
	setting.UserEnabled = false
	setting.UserSource = assistant_setting.SourceDirect
	setting.UserProtocol = assistant_setting.ProtocolResponses
	setting.UserBaseURL = "https://api.openai.com/v1"
	setting.UserAPIKey = "sk-test"
	setting.UserModel = "gpt-test"
	setting.UserContextWindow = 1000000
	setting.UserMaxOutputTokens = 100

	body, err := common.Marshal(map[string]any{"message": "hi", "history": []any{}})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assistant/chat", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 9998)

	AssistantChat(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	found := false
	for _, block := range strings.Split(recorder.Body.String(), "\n\n") {
		block = strings.TrimSpace(block)
		if !strings.HasPrefix(block, "data: ") {
			continue
		}
		payload := map[string]any{}
		require.NoError(t, common.Unmarshal([]byte(strings.TrimPrefix(block, "data: ")), &payload))
		if payload["type"] != "error" {
			continue
		}
		found = true
		assert.NotEmpty(t, payload["message"])
		assert.NotContains(t, payload, "error")
	}
	assert.True(t, found, "expected an SSE error event")
}

func TestAdminAssistantChatStreamsOverSSE(t *testing.T) {
	setupAssistantOptionsTest(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/responses", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"id\":\"resp_admin\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"AdminHello\"}]}]}")
	}))
	defer upstream.Close()

	setting := assistant_setting.Get()
	setting.AdminEnabled = true
	setting.AdminSource = assistant_setting.SourceDirect
	setting.AdminBaseURL = upstream.URL
	setting.AdminAPIKey = "sk-admin"
	setting.AdminModel = "gpt-admin"
	setting.AdminProtocol = assistant_setting.ProtocolResponses
	setting.RequestTimeoutSeconds = 5
	setting.AdminContextWindow = 1000000
	setting.AdminMaxOutputTokens = 100

	body, err := common.Marshal(map[string]any{"message": "hi", "history": []any{}})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assistant/admin/chat", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 9997)

	AdminAssistantChat(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "AdminHello")
}

func TestAssistantChatRejectsWhenDailyTokenLimitReached(t *testing.T) {
	setupAssistantOptionsTest(t)
	originalDB := model.DB
	dsn := fmt.Sprintf("file:assistant_chat_quota_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.AssistantUsageDaily{}))
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})

	setting := assistant_setting.Get()
	setting.UserEnabled = true
	setting.UserSource = assistant_setting.SourceDirect
	setting.UserBaseURL = "https://api.openai.com/v1"
	setting.UserAPIKey = "sk-test"
	setting.UserModel = "gpt-test"
	setting.UserProtocol = assistant_setting.ProtocolResponses
	setting.UserContextWindow = 1000000
	setting.UserMaxOutputTokens = 100
	setting.UserDailyTokenLimit = 10

	now := time.Now()
	day := now.Year()*10000 + int(now.Month())*100 + now.Day()
	require.NoError(t, model.AddAssistantDailyUsage(9996, day, 10, 0, 10))

	body, err := common.Marshal(map[string]any{"message": "hi"})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assistant/chat", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 9996)

	AssistantChat(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "assistant_daily_token_limit")
	assert.Contains(t, recorder.Body.String(), "retry_after")
}
