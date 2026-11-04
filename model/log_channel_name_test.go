package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The usage-log channel column renders "name #id" for admins and is hidden for
// everyone else. Admins viewing their own scope take the GetUserLogs path, so
// that query must resolve channel names exactly like the all-users listing.
func TestGetUserLogsResolvesChannelNamesForAdminScope(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&Channel{Id: 11, Name: "openai-primary", Key: "k"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 12, Name: "backup", Key: "k"}).Error)

	seedConsumeLogs(t,
		Log{UserId: 7, Type: LogTypeConsume, ChannelId: 11, Quota: 10},
		Log{UserId: 7, Type: LogTypeConsume, ChannelId: 12, Quota: 10},
		Log{UserId: 7, Type: LogTypeConsume, ChannelId: 0, Quota: 10},
	)

	logs, total, err := GetUserLogs(7, LogTypeUnknown, "usage", 0, 0, "", "", 0, 10, "", "", "", true)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, logs, 3)

	names := make(map[int]string, len(logs))
	for _, log := range logs {
		names[log.ChannelId] = log.ChannelName
	}
	assert.Equal(t, "openai-primary", names[11])
	assert.Equal(t, "backup", names[12])
	assert.Equal(t, "", names[0], "logs without a channel must keep an empty name")
}

func TestGetUserLogsKeepsChannelNamesHiddenFromNonAdminScope(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&Channel{Id: 11, Name: "openai-primary", Key: "k"}).Error)
	seedConsumeLogs(t, Log{UserId: 7, Type: LogTypeConsume, ChannelId: 11, Quota: 10})

	logs, _, err := GetUserLogs(7, LogTypeUnknown, "usage", 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Empty(t, logs[0].ChannelName)
}
