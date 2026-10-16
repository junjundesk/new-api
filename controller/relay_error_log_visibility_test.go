package controller

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupProcessChannelErrorDB 提供 processChannelError 依赖的最小环境：日志表
// （含 admin_only 列）、用户表与错误日志总开关。ErrorLogEnabled 是包级变量，
// 测试结束后恢复，避免影响其它用例。
func setupProcessChannelErrorDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousEnabled := constant.ErrorLogEnabled
	previousRedisEnabled := common.RedisEnabled

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.User{}))
	model.DB = db
	model.LOG_DB = db
	constant.ErrorLogEnabled = true
	common.RedisEnabled = false

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		constant.ErrorLogEnabled = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// newChannelErrorTestContext 构造 processChannelError 需要的请求上下文，字段
// 与 relay 路径上中间件写入的 context key 保持一致。
func newChannelErrorTestContext(userId int, channelId int, channelName string, requestPath string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("id", userId)
	c.Set("token_name", "test-token")
	c.Set("original_model", "deepseek-flash")
	c.Set("token_id", 1)
	c.Set("group", "default")
	c.Set("channel_id", channelId)
	c.Set("channel_name", channelName)
	c.Request = httptest.NewRequest("POST", requestPath, nil)
	return c
}

// TestProcessChannelErrorMarksSupersededAttemptsAdminOnly 验证同一次请求中已被
// 后续尝试取代的中间失败只写管理员可见日志，而最终失败对用户可见。
func TestProcessChannelErrorMarksSupersededAttemptsAdminOnly(t *testing.T) {
	db := setupProcessChannelErrorDB(t)
	const userId = 940001

	intermediate := newChannelErrorTestContext(userId, 11, "cn-channel", "/v1/chat/completions")
	intermediateErr := types.NewOpenAIError(fmt.Errorf("model unavailable"), types.ErrorCodeBadResponseStatusCode, 404)
	processChannelError(intermediate, *types.NewChannelError(11, 1, "cn-channel", false, "", true), intermediateErr, true)

	final := newChannelErrorTestContext(userId, 12, "global-channel", "/v1/chat/completions")
	finalErr := types.NewOpenAIError(fmt.Errorf("model unavailable"), types.ErrorCodeBadResponseStatusCode, 404)
	processChannelError(final, *types.NewChannelError(12, 1, "global-channel", false, "", true), finalErr, false)

	var rows []model.Log
	require.NoError(t, db.Where("user_id = ?", userId).Order("id asc").Find(&rows).Error)
	require.Len(t, rows, 2)

	assert.True(t, rows[0].AdminOnly, "superseded intermediate attempt must be admin-only")
	assert.Equal(t, 11, rows[0].ChannelId)
	assert.False(t, rows[1].AdminOnly, "the final failed attempt stays visible to the user")
	assert.Equal(t, 12, rows[1].ChannelId)

	// 用户侧查询只能看到最终失败；管理员侧两条都在。
	userLogs, total, err := model.GetUserLogs(userId, model.LogTypeUnknown, "", 0, 0, "", "", 0, 20, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, userLogs, 1)
	assert.Equal(t, 12, userLogs[0].ChannelId)

	adminLogs, total, err := model.GetUserLogs(userId, model.LogTypeUnknown, "", 0, 0, "", "", 0, 20, "", "", "", true)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, adminLogs, 2)
}
