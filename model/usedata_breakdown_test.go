package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func seedBreakdownQuotaData(t *testing.T, rows ...QuotaData) {
	t.Helper()
	for _, row := range rows {
		quotaData := row
		require.NoError(t, DB.Create(&quotaData).Error)
	}
}

func TestGetUserQuotaBreakdownAggregatesByUserGroupAndModel(t *testing.T) {
	truncateTables(t)
	seedBreakdownQuotaData(t,
		QuotaData{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a", CreatedAt: 3600, Count: 2, Quota: 100, TokenUsed: 40},
		QuotaData{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a", CreatedAt: 7200, Count: 1, Quota: 50, TokenUsed: 20},
		QuotaData{UserID: 1, Username: "alice", UseGroup: "default", ModelName: "gpt-a", CreatedAt: 3600, Count: 4, Quota: 400, TokenUsed: 80},
		QuotaData{UserID: 2, Username: "bob", UseGroup: "vip", ModelName: "gpt-b", CreatedAt: 3600, Count: 1, Quota: 25, TokenUsed: 10},
	)

	result, err := GetUserQuotaBreakdown(0, 0, "", 0, common.RoleRootUser, nil)
	require.NoError(t, err)
	require.False(t, result.Truncated)
	require.Len(t, result.Rows, 3)
	// Rows are ordered by quota descending, and the two alice/vip/gpt-a rows
	// collapse into a single group+model line.
	require.Equal(t, UserQuotaBreakdownRow{
		UserID: 1, Username: "alice", UseGroup: "default", ModelName: "gpt-a",
		Count: 4, Quota: 400, TokenUsed: 80,
	}, *result.Rows[0])
	require.Equal(t, UserQuotaBreakdownRow{
		UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a",
		Count: 3, Quota: 150, TokenUsed: 60,
	}, *result.Rows[1])
	require.Equal(t, UserQuotaBreakdownRow{
		UserID: 2, Username: "bob", UseGroup: "vip", ModelName: "gpt-b",
		Count: 1, Quota: 25, TokenUsed: 10,
	}, *result.Rows[2])
}

func TestGetUserQuotaBreakdownScopesByRole(t *testing.T) {
	truncateTables(t)
	seedBreakdownQuotaData(t,
		QuotaData{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a", CreatedAt: 3600, Count: 1, Quota: 100, TokenUsed: 40},
		QuotaData{UserID: 2, Username: "bob", UseGroup: "default", ModelName: "gpt-b", CreatedAt: 3600, Count: 1, Quota: 25, TokenUsed: 10},
	)

	rootRows, err := GetUserQuotaBreakdown(0, 0, "", 0, common.RoleRootUser, nil)
	require.NoError(t, err)
	require.Len(t, rootRows.Rows, 2)

	adminRows, err := GetUserQuotaBreakdown(0, 0, "alice", 0, common.RoleAdminUser, nil)
	require.NoError(t, err)
	require.Len(t, adminRows.Rows, 1)
	require.Equal(t, "alice", adminRows.Rows[0].Username)

	// A common user cannot widen the scope by passing another user's username.
	selfRows, err := GetUserQuotaBreakdown(0, 0, "bob", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	require.Len(t, selfRows.Rows, 1)
	require.Equal(t, 1, selfRows.Rows[0].UserID)
	require.Equal(t, "alice", selfRows.Rows[0].Username)
}

func TestGetUserQuotaBreakdownTreatsZeroBoundsAsUnbounded(t *testing.T) {
	truncateTables(t)
	seedBreakdownQuotaData(t,
		QuotaData{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a", CreatedAt: 1000, Count: 1, Quota: 10, TokenUsed: 1},
		QuotaData{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-b", CreatedAt: 5000, Count: 1, Quota: 20, TokenUsed: 2},
	)

	allRows, err := GetUserQuotaBreakdown(0, 0, "", 0, common.RoleRootUser, nil)
	require.NoError(t, err)
	require.Len(t, allRows.Rows, 2)

	startBoundedRows, err := GetUserQuotaBreakdown(2000, 0, "", 0, common.RoleRootUser, nil)
	require.NoError(t, err)
	require.Len(t, startBoundedRows.Rows, 1)
	require.Equal(t, int64(20), startBoundedRows.Rows[0].Quota)

	endBoundedRows, err := GetUserQuotaBreakdown(0, 2000, "", 0, common.RoleRootUser, nil)
	require.NoError(t, err)
	require.Len(t, endBoundedRows.Rows, 1)
	require.Equal(t, int64(10), endBoundedRows.Rows[0].Quota)
}

func TestGetUserQuotaBreakdownExcludesRequestedUsers(t *testing.T) {
	truncateTables(t)
	seedBreakdownQuotaData(t,
		QuotaData{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a", CreatedAt: 3600, Count: 1, Quota: 100, TokenUsed: 40},
		QuotaData{UserID: 10, Username: "admin", UseGroup: "default", ModelName: "gpt-a", CreatedAt: 3600, Count: 1, Quota: 70, TokenUsed: 30},
		QuotaData{UserID: 2, Username: "bob", UseGroup: "default", ModelName: "gpt-b", CreatedAt: 3600, Count: 1, Quota: 25, TokenUsed: 10},
	)

	result, err := GetUserQuotaBreakdown(0, 0, "", 0, common.RoleAdminUser, []int{10})
	require.NoError(t, err)
	require.Len(t, result.Rows, 2)
	require.Equal(t, "alice", result.Rows[0].Username)
	require.Equal(t, "bob", result.Rows[1].Username)

	// Excluding every spender leaves an empty breakdown rather than falling
	// back to the unfiltered list.
	emptyRows, err := GetUserQuotaBreakdown(0, 0, "", 0, common.RoleAdminUser, []int{1, 2, 10})
	require.NoError(t, err)
	require.Empty(t, emptyRows.Rows)
}

func TestGetUserQuotaBreakdownIgnoresExclusionsForSelfScope(t *testing.T) {
	truncateTables(t)
	seedBreakdownQuotaData(t,
		QuotaData{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a", CreatedAt: 3600, Count: 1, Quota: 100, TokenUsed: 40},
		QuotaData{UserID: 2, Username: "bob", UseGroup: "default", ModelName: "gpt-b", CreatedAt: 3600, Count: 1, Quota: 25, TokenUsed: 10},
	)

	selfRows, err := GetUserQuotaBreakdown(0, 0, "", 1, common.RoleCommonUser, []int{1})
	require.NoError(t, err)
	require.Len(t, selfRows.Rows, 1)
	require.Equal(t, 1, selfRows.Rows[0].UserID)
}
