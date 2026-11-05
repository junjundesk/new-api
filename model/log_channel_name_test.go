package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

func TestGetUserLogsKeepsChannelInfoHiddenFromNonAdminScope(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&Channel{Id: 11, Name: "openai-primary", Key: "k", Group: "vip"}).Error)
	seedConsumeLogs(t, Log{UserId: 7, Type: LogTypeConsume, ChannelId: 11, Quota: 10})

	logs, _, err := GetUserLogs(7, LogTypeUnknown, "usage", 0, 0, "", "", 0, 10, "", "", "", false)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Empty(t, logs[0].ChannelName)
	assert.Empty(t, logs[0].ChannelGroups)
}

// The channel column links to a group's channel list, so admin reads must
// return the channel's own current groups (not the log's billing group)
// whichever lookup path serves the page.
func TestGetUserLogsResolvesChannelGroupsForAdminScope(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory cache %v", memoryCacheEnabled), func(t *testing.T) {
			originalMemoryCacheEnabled := common.MemoryCacheEnabled
			// Registered before truncateTables so it runs after the tables are
			// cleared and the cache reloads empty.
			t.Cleanup(func() {
				InitChannelCache()
				common.MemoryCacheEnabled = originalMemoryCacheEnabled
			})
			common.MemoryCacheEnabled = memoryCacheEnabled
			truncateTables(t)
			channel := &Channel{Id: 11, Name: "openai-primary", Key: "k", Status: common.ChannelStatusEnabled, Group: "default,vip", Models: "gpt-4o"}
			require.NoError(t, channel.Insert())
			InitChannelCache()
			seedConsumeLogs(t,
				Log{UserId: 7, Type: LogTypeError, ChannelId: 11, Group: "auto"},
				Log{UserId: 7, Type: LogTypeConsume, ChannelId: 0, Group: "vip", Quota: 10},
			)

			logs, _, err := GetUserLogs(7, LogTypeUnknown, "usage", 0, 0, "", "", 0, 10, "", "", "", true)
			require.NoError(t, err)
			require.Len(t, logs, 2)

			groups := make(map[int][]string, len(logs))
			for _, log := range logs {
				groups[log.ChannelId] = log.ChannelGroups
			}
			assert.Equal(t, []string{"default", "vip"}, groups[11])
			assert.Empty(t, groups[0], "logs without a channel must carry no channel groups")
		})
	}
}
