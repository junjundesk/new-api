package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAllQuotaDatesExcludesRequestedUsers(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&QuotaData{
		UserID: 1, Username: "alice", ModelName: "gpt-a", CreatedAt: 3600,
		Count: 2, Quota: 100, TokenUsed: 40,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID: 10, Username: "admin", ModelName: "gpt-a", CreatedAt: 3600,
		Count: 1, Quota: 70, TokenUsed: 30,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID: 2, Username: "bob", ModelName: "gpt-b", CreatedAt: 3600,
		Count: 1, Quota: 25, TokenUsed: 10,
	}).Error)

	rows, err := GetAllQuotaDates(0, 7200, "", []int{10})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// alice and the admin share gpt-a, so the exclusion has to be reflected in
	// the aggregate: 100 stays, the admin's 70 is dropped.
	quotaByModel := make(map[string]int, len(rows))
	countByModel := make(map[string]int, len(rows))
	for _, row := range rows {
		quotaByModel[row.ModelName] = row.Quota
		countByModel[row.ModelName] = row.Count
	}
	require.Equal(t, 100, quotaByModel["gpt-a"])
	require.Equal(t, 2, countByModel["gpt-a"])
	require.Equal(t, 25, quotaByModel["gpt-b"])
	require.Equal(t, 1, countByModel["gpt-b"])
}

func TestGetAllQuotaDatesCombinesUsernameAndExclusion(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&QuotaData{
		UserID: 1, Username: "alice", ModelName: "gpt-a", CreatedAt: 3600,
		Count: 1, Quota: 100, TokenUsed: 40,
	}).Error)

	rows, err := GetAllQuotaDates(0, 7200, "alice", []int{1})
	require.NoError(t, err)
	require.Empty(t, rows)

	rows, err = GetAllQuotaDates(0, 7200, "alice", nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 100, rows[0].Quota)
}
