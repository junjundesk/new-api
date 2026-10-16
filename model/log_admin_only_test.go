package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// seedSupersededAttemptLogs 写入三条同一请求链路的日志：一条最终成功的消费日志、
// 一条已被后续尝试取代的中间失败日志（admin_only），以及一条 admin_only 列为
// NULL 的历史错误日志（新增列之前的写入）。
func seedSupersededAttemptLogs(t *testing.T, userId int, tokenId int) {
	t.Helper()
	rows := []*Log{
		{UserId: userId, Type: LogTypeConsume, ModelName: "deepseek-flash", TokenId: tokenId, Content: "consume", Quota: 1},
		{UserId: userId, Type: LogTypeError, ModelName: "deepseek-flash", TokenId: tokenId, Content: "attempt-cn-failed", AdminOnly: true},
		{UserId: userId, Type: LogTypeError, ModelName: "deepseek-flash", TokenId: tokenId, Content: "attempt-legacy"},
	}
	for _, row := range rows {
		require.NoError(t, LOG_DB.Create(row).Error)
	}
	require.NoError(t, LOG_DB.Exec("UPDATE logs SET admin_only = NULL WHERE content = ?", "attempt-legacy").Error)
	t.Cleanup(func() {
		LOG_DB.Where("user_id = ?", userId).Delete(&Log{})
	})
}

// TestGetUserLogsHidesSupersededAttempts 保证用户侧日志列表隐藏被后续尝试取代
// 的中间失败记录，同时保留历史行（admin_only 为 NULL）与正常日志。
func TestGetUserLogsHidesSupersededAttempts(t *testing.T) {
	userId := 930001
	seedSupersededAttemptLogs(t, userId, 0)

	logs, total, err := GetUserLogs(userId, LogTypeUnknown, "", 0, 0, "", "", 0, 20, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, logs, 2)
	for _, log := range logs {
		assert.False(t, log.AdminOnly, "user view must not contain admin-only rows")
		assert.NotEqual(t, "attempt-cn-failed", log.Content)
	}
}

// TestGetUserLogsKeepsSupersededAttemptsForAdmin 保证管理员查询仍能看到全部
// 尝试记录，包括被取代的中间失败。
func TestGetUserLogsKeepsSupersededAttemptsForAdmin(t *testing.T) {
	userId := 930002
	seedSupersededAttemptLogs(t, userId, 0)

	logs, total, err := GetUserLogs(userId, LogTypeUnknown, "", 0, 0, "", "", 0, 20, "", "", "", true)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, logs, 3)

	contents := make([]string, 0, len(logs))
	for _, log := range logs {
		contents = append(contents, log.Content)
	}
	assert.Contains(t, contents, "attempt-cn-failed")
}

// TestGetLogByTokenIdHidesSupersededAttempts 保证令牌维度的日志接口同样不向
// 用户暴露被取代的中间失败记录。
func TestGetLogByTokenIdHidesSupersededAttempts(t *testing.T) {
	userId := 930003
	tokenId := 9300031
	seedSupersededAttemptLogs(t, userId, tokenId)

	logs, err := GetLogByTokenId(tokenId)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	for _, log := range logs {
		assert.False(t, log.AdminOnly)
		assert.NotEqual(t, "attempt-cn-failed", log.Content)
	}
}

// TestApplyUserVisibleLogFilterBranchesByLogDatabase 保证过滤条件适配两类日志库：
// ClickHouse 的 admin_only 列不可为空，直接比较；SQL 库为兼容历史 NULL 行使用
// COALESCE。SQLite/MySQL/PostgreSQL 共用后者。
func TestApplyUserVisibleLogFilterBranchesByLogDatabase(t *testing.T) {
	dummyDB, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)
	originalLogType := common.LogDatabaseType()
	t.Cleanup(func() { common.SetLogDatabaseType(originalLogType) })

	buildSQL := func() string {
		var rows []Log
		return applyUserVisibleLogFilter(dummyDB.Model(&Log{})).Find(&rows).Statement.SQL.String()
	}

	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	assert.True(t, strings.Contains(buildSQL(), "admin_only = 0"), "ClickHouse filter should compare the non-nullable column directly")

	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	assert.Contains(t, buildSQL(), "admin_only = ? OR admin_only IS NULL")
}
