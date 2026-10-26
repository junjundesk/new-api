package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoIncludesFallbackSummaryAndAdminAttempts(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Set("use_channel", []string{"11", "42"})
	start := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	state := &relaycommon.FallbackState{
		OriginalModel: "mapped-model",
		OriginalGroup: "default",
		Attempts: []relaycommon.FallbackAttempt{
			{Index: 1, Group: "vip", Model: "unavailable-backup", ChannelId: 21, StatusCode: 503, Error: "upstream unavailable"},
			{Index: 2, Group: "azure", Model: "working-backup", ChannelId: 42, StatusCode: 200},
		},
	}
	state.Used = &state.Attempts[1]
	info := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(25 * time.Millisecond),
		RelayFormat:       types.RelayFormatOpenAI,
		ChannelMeta:       &relaycommon.ChannelMeta{},
		Fallback:          state,
	}

	other := GenerateTextOtherInfo(c, info, 2, 1.5, 1.2, 3, 0.5, 0, 0.8)
	fallback, ok := other["fallback"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, map[string]interface{}{
		"from_model": "mapped-model",
		"from_group": "default",
		"to_model":   "working-backup",
		"to_group":   "azure",
		"attempt":    2,
	}, fallback)
	assert.NotContains(t, fallback, "error", "upstream diagnostics must stay admin-only")
	assert.NotContains(t, fallback, "channel_id")

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []string{"11", "42"}, adminInfo["use_channel"])
	attempts, ok := adminInfo["fallback_attempts"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, attempts, 2)
	assert.Equal(t, map[string]interface{}{
		"index":       1,
		"group":       "vip",
		"model":       "unavailable-backup",
		"channel_id":  21,
		"status_code": 503,
		"error":       "upstream unavailable",
	}, attempts[0])
	assert.Equal(t, map[string]interface{}{
		"index":       2,
		"group":       "azure",
		"model":       "working-backup",
		"channel_id":  42,
		"status_code": 200,
	}, attempts[1], "successful attempts must not acquire a synthetic error")
	assert.NotContains(t, other, "fallback_attempts", "audit details must not be duplicated into the user-visible map")
}

func TestAppendFallbackInfoMarksExhaustedFallbacks(t *testing.T) {
	other := map[string]interface{}{}
	adminInfo := map[string]interface{}{"request_ip": "192.0.2.10"}
	state := &relaycommon.FallbackState{
		OriginalModel: "gpt-5",
		OriginalGroup: "default",
		Attempts: []relaycommon.FallbackAttempt{
			{Index: 1, Group: "vip", Model: "backup-a", StatusCode: 0, Error: "model no longer allowed"},
			{Index: 2, Group: "azure", Model: "backup-b", ChannelId: 42, StatusCode: 502, Error: "bad gateway"},
		},
	}

	AppendFallbackInfo(other, adminInfo, state)
	assert.Equal(t, map[string]interface{}{
		"from_model": "gpt-5",
		"from_group": "default",
		"failed":     true,
		"attempts":   2,
	}, other["fallback"])
	assert.Equal(t, "192.0.2.10", adminInfo["request_ip"], "fallback logging must preserve unrelated admin metadata")
	assert.Contains(t, adminInfo, "fallback_attempts")
}

func TestAppendFallbackInfoDoesNotMarkUntriedConfiguration(t *testing.T) {
	other := map[string]interface{}{"model_ratio": 2.0}
	adminInfo := map[string]interface{}{"request_ip": "192.0.2.10"}
	AppendFallbackInfo(other, adminInfo, &relaycommon.FallbackState{
		OriginalModel: "gpt-5",
		OriginalGroup: "default",
		Configured:    []relaycommon.FallbackTarget{{Group: "vip", Model: "backup"}},
	})
	assert.NotContains(t, other, "fallback")
	assert.Equal(t, map[string]interface{}{"request_ip": "192.0.2.10"}, adminInfo)
	AppendFallbackInfo(other, adminInfo, nil)
	assert.NotContains(t, other, "fallback")
}

func TestFallbackLogVisibilityThroughSerializedUserAndAdminViews(t *testing.T) {
	const userID = 940101
	previousLogDB := model.LOG_DB
	previousMainDB := model.DB
	previousLogType := common.LogDatabaseType()
	previousMemoryCache := common.MemoryCacheEnabled
	db := model.LOG_DB
	require.NotNil(t, db, "the package test fixture must explicitly initialize its in-memory log database")
	require.NotNil(t, model.DB)
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		require.NoError(t, db.Where("user_id = ?", userID).Delete(&model.Log{}).Error)
		model.LOG_DB = previousLogDB
		model.DB = previousMainDB
		common.SetLogDatabaseType(previousLogType)
		common.MemoryCacheEnabled = previousMemoryCache
	})

	state := &relaycommon.FallbackState{
		OriginalModel: "gpt-5",
		OriginalGroup: "default",
		Attempts:      []relaycommon.FallbackAttempt{{Index: 1, Group: "vip", Model: "backup-model", ChannelId: 42, StatusCode: 503, Error: "private upstream diagnostic"}},
	}
	state.Used = &state.Attempts[0]
	other := map[string]interface{}{}
	adminInfo := map[string]interface{}{}
	AppendFallbackInfo(other, adminInfo, state)
	other["admin_info"] = adminInfo
	otherJSON := common.MapToJsonStr(other)
	require.NotEmpty(t, otherJSON)
	row := &model.Log{
		UserId:    userID,
		CreatedAt: 1793000000,
		Type:      model.LogTypeConsume,
		ModelName: "backup-model",
		Other:     otherJSON,
	}
	require.NoError(t, db.Create(row).Error)

	userLogs, total, err := model.GetUserLogs(userID, model.LogTypeConsume, "", 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, userLogs, 1)
	userOther, err := common.StrToMap(userLogs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, userOther, "admin_info")
	assert.NotContains(t, userOther, "fallback_attempts")
	assert.NotContains(t, userLogs[0].Other, "private upstream diagnostic")
	assert.Equal(t, map[string]interface{}{
		"from_model": "gpt-5",
		"from_group": "default",
		"to_model":   "backup-model",
		"to_group":   "vip",
		"attempt":    float64(1),
	}, userOther["fallback"])

	adminLogs, total, err := model.GetUserLogs(userID, model.LogTypeConsume, "", 0, 0, "", "", 0, 10, "", "", "", true)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, adminLogs, 1)
	adminOther, err := common.StrToMap(adminLogs[0].Other)
	require.NoError(t, err)
	visibleAdminInfo, ok := adminOther["admin_info"].(map[string]interface{})
	require.True(t, ok)
	visibleAttempts, ok := visibleAdminInfo["fallback_attempts"].([]interface{})
	require.True(t, ok)
	require.Len(t, visibleAttempts, 1)
	assert.Equal(t, map[string]interface{}{
		"index":       float64(1),
		"group":       "vip",
		"model":       "backup-model",
		"channel_id":  float64(42),
		"status_code": float64(503),
		"error":       "private upstream diagnostic",
	}, visibleAttempts[0])
}
