package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedConsumeLogs(t *testing.T, logs ...Log) {
	t.Helper()
	for _, row := range logs {
		entry := row
		require.NoError(t, DB.Create(&entry).Error)
	}
}

func TestGetUserConsumeTotalsScopesToUserAndVisibleConsumeLogs(t *testing.T) {
	truncateTables(t)
	seedConsumeLogs(t,
		Log{UserId: 1, Type: LogTypeConsume, Quota: 100, PromptTokens: 10, CompletionTokens: 5},
		Log{UserId: 1, Type: LogTypeConsume, Quota: 50, PromptTokens: 4, CompletionTokens: 1},
		Log{UserId: 2, Type: LogTypeConsume, Quota: 999, PromptTokens: 100, CompletionTokens: 100},
		// Error logs never contribute to consumption.
		Log{UserId: 1, Type: LogTypeError, Quota: 0},
		// Admin-only rows are hidden from user-facing aggregates.
		Log{UserId: 1, Type: LogTypeConsume, Quota: 700, AdminOnly: true},
	)

	totals, err := GetUserConsumeTotals(1, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, UserConsumeTotals{Quota: 150, Count: 2, Tokens: 20}, totals)

	other, err := GetUserConsumeTotals(2, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, UserConsumeTotals{Quota: 999, Count: 1, Tokens: 200}, other)
}

func TestGetUserConsumeTotalsAppliesInclusiveTimeBounds(t *testing.T) {
	truncateTables(t)
	seedConsumeLogs(t,
		Log{UserId: 1, Type: LogTypeConsume, CreatedAt: 1000, Quota: 10, PromptTokens: 1},
		Log{UserId: 1, Type: LogTypeConsume, CreatedAt: 2000, Quota: 20, PromptTokens: 2},
		Log{UserId: 1, Type: LogTypeConsume, CreatedAt: 3000, Quota: 30, PromptTokens: 3},
	)

	window, err := GetUserConsumeTotals(1, 1500, 2000)
	require.NoError(t, err)
	assert.Equal(t, UserConsumeTotals{Quota: 20, Count: 1, Tokens: 2}, window)

	upTo, err := GetUserConsumeTotals(1, 0, 2000)
	require.NoError(t, err)
	assert.Equal(t, UserConsumeTotals{Quota: 30, Count: 2, Tokens: 3}, upTo)

	from, err := GetUserConsumeTotals(1, 2000, 0)
	require.NoError(t, err)
	assert.Equal(t, UserConsumeTotals{Quota: 50, Count: 2, Tokens: 5}, from)
}

func TestGetUserConsumeTotalsReturnsZeroWithoutConsumeLogs(t *testing.T) {
	truncateTables(t)
	seedConsumeLogs(t, Log{UserId: 1, Type: LogTypeError, Quota: 80})

	totals, err := GetUserConsumeTotals(1, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, UserConsumeTotals{}, totals)
}

func TestGetUserConsumeBreakdownSupportsAllowlistedDimensions(t *testing.T) {
	truncateTables(t)
	seedConsumeLogs(t,
		Log{UserId: 1, Type: LogTypeConsume, ModelName: "gpt-a", Group: "vip", TokenName: "token-1", Quota: 100, PromptTokens: 10, CompletionTokens: 5},
		Log{UserId: 1, Type: LogTypeConsume, ModelName: "gpt-a", Group: "default", TokenName: "token-2", Quota: 50, PromptTokens: 4, CompletionTokens: 1},
		Log{UserId: 1, Type: LogTypeConsume, ModelName: "gpt-b", Group: "vip", TokenName: "token-2", Quota: 200, PromptTokens: 20, CompletionTokens: 10},
		Log{UserId: 2, Type: LogTypeConsume, ModelName: "gpt-c", Group: "vip", TokenName: "token-3", Quota: 500},
	)

	cases := []struct {
		dimension string
		want      []*UserConsumeBreakdownRow
	}{
		{
			dimension: "model",
			want: []*UserConsumeBreakdownRow{
				{Dimension: "gpt-b", Quota: 200, Count: 1, Tokens: 30},
				{Dimension: "gpt-a", Quota: 150, Count: 2, Tokens: 20},
			},
		},
		{
			dimension: "group",
			want: []*UserConsumeBreakdownRow{
				{Dimension: "vip", Quota: 300, Count: 2, Tokens: 45},
				{Dimension: "default", Quota: 50, Count: 1, Tokens: 5},
			},
		},
		{
			dimension: "token",
			want: []*UserConsumeBreakdownRow{
				{Dimension: "token-2", Quota: 250, Count: 2, Tokens: 35},
				{Dimension: "token-1", Quota: 100, Count: 1, Tokens: 15},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.dimension, func(t *testing.T) {
			rows, err := GetUserConsumeBreakdown(1, 0, 0, tc.dimension, 10)
			require.NoError(t, err)
			assert.Equal(t, tc.want, rows)
		})
	}
}

func TestGetUserConsumeBreakdownRejectsUnknownDimension(t *testing.T) {
	truncateTables(t)
	seedConsumeLogs(t, Log{UserId: 1, Type: LogTypeConsume, ModelName: "gpt-a", Quota: 100})

	for _, dimension := range []string{"", "user_id", "quota", "model_name", "model_name; DROP TABLE logs"} {
		rows, err := GetUserConsumeBreakdown(1, 0, 0, dimension, 10)
		require.Error(t, err, "dimension %q must be rejected", dimension)
		assert.Nil(t, rows, "dimension %q must not query", dimension)
	}
}

func TestGetUserConsumeBreakdownClampsLimit(t *testing.T) {
	truncateTables(t)
	// Distinct descending quotas keep the expected row order unambiguous.
	for i := 0; i < userConsumeBreakdownMaxLimit+10; i++ {
		seedConsumeLogs(t, Log{
			UserId:    1,
			Type:      LogTypeConsume,
			ModelName: fmt.Sprintf("model-%02d", i),
			Quota:     1000 - i,
		})
	}

	capped, err := GetUserConsumeBreakdown(1, 0, 0, "model", 100)
	require.NoError(t, err)
	require.Len(t, capped, userConsumeBreakdownMaxLimit)
	assert.Equal(t, "model-00", capped[0].Dimension)

	def, err := GetUserConsumeBreakdown(1, 0, 0, "model", 0)
	require.NoError(t, err)
	require.Len(t, def, userConsumeBreakdownDefaultLimit)

	top, err := GetUserConsumeBreakdown(1, 0, 0, "model", 2)
	require.NoError(t, err)
	require.Len(t, top, 2)
	assert.Equal(t, int64(1000), top[0].Quota)
	assert.Equal(t, int64(999), top[1].Quota)
}

func TestGetUserConsumeBreakdownReturnsEmptyForNoConsumeLogs(t *testing.T) {
	truncateTables(t)
	seedConsumeLogs(t, Log{UserId: 1, Type: LogTypeError, ModelName: "gpt-a", Quota: 80})

	rows, err := GetUserConsumeBreakdown(1, 0, 0, "model", 10)
	require.NoError(t, err)
	assert.NotNil(t, rows)
	assert.Empty(t, rows)
}
