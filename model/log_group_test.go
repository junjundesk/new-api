package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedLogGroupRows 写入两组日志各四条，覆盖使用日志与用户日志的全部分组类型。
func seedLogGroupRows(t *testing.T, userId int) {
	t.Helper()
	rows := []*Log{
		{UserId: userId, Type: LogTypeConsume, Content: "consume"},
		{UserId: userId, Type: LogTypeError, Content: "error"},
		{UserId: userId, Type: LogTypeRefund, Content: "refund"},
		{UserId: userId, Type: LogTypeUnknown, Content: "unknown"},
		{UserId: userId, Type: LogTypeTopup, Content: "topup"},
		{UserId: userId, Type: LogTypeManage, Content: "manage"},
		{UserId: userId, Type: LogTypeSystem, Content: "system"},
		{UserId: userId, Type: LogTypeLogin, Content: "login"},
	}
	for _, row := range rows {
		require.NoError(t, LOG_DB.Create(row).Error)
	}
	t.Cleanup(func() {
		LOG_DB.Where("user_id = ?", userId).Delete(&Log{})
	})
}

// TestGetUserLogsSplitsUsageAndAccountGroups 保证使用日志与用户日志两个页面各自
// 只查到自己那一组类型，显式指定的 type 优先于分组，不指定分组时保持全部类型语义。
func TestGetUserLogsSplitsUsageAndAccountGroups(t *testing.T) {
	userId := 930101
	seedLogGroupRows(t, userId)

	usage, total, err := GetUserLogs(userId, LogTypeUnknown, LogGroupUsage, 0, 0, "", "", 0, 20, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	for _, log := range usage {
		assert.Contains(t, []int{LogTypeUnknown, LogTypeConsume, LogTypeError, LogTypeRefund}, log.Type)
	}

	account, total, err := GetUserLogs(userId, LogTypeUnknown, LogGroupAccount, 0, 0, "", "", 0, 20, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	for _, log := range account {
		assert.Contains(t, []int{LogTypeTopup, LogTypeManage, LogTypeSystem, LogTypeLogin}, log.Type)
	}

	// 分组与显式 type 取交集：旧链接把登录日志带进使用日志页时必须查不到。
	crossGroup, total, err := GetUserLogs(userId, LogTypeLogin, LogGroupUsage, 0, 0, "", "", 0, 20, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, crossGroup)

	inGroup, total, err := GetUserLogs(userId, LogTypeLogin, LogGroupAccount, 0, 0, "", "", 0, 20, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, inGroup, 1)
	assert.Equal(t, LogTypeLogin, inGroup[0].Type)

	all, total, err := GetUserLogs(userId, LogTypeUnknown, "", 0, 0, "", "", 0, 20, "", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, int64(8), total)
	assert.Len(t, all, 8)
}

// TestGetAllLogsFiltersByLogGroup 保证管理端列表同样按分组过滤。
func TestGetAllLogsFiltersByLogGroup(t *testing.T) {
	userId := 930102
	seedLogGroupRows(t, userId)

	account, total, err := GetAllLogs(LogTypeUnknown, LogGroupAccount, 0, 0, "", "", "", 0, 20, 0, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	require.Len(t, account, 4)
	for _, log := range account {
		assert.Contains(t, []int{LogTypeTopup, LogTypeManage, LogTypeSystem, LogTypeLogin}, log.Type)
	}
}
