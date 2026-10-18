package assistant

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAssistantUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:assistant_usage_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.AssistantUsageDaily{}))
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestDailyTokenQuotaTracksUsageAndRejectsWhenReached(t *testing.T) {
	setupAssistantUsageTestDB(t)

	exceeded, retryAfter, err := CheckDailyTokenQuota(1, 0)
	require.NoError(t, err)
	assert.False(t, exceeded)
	assert.Zero(t, retryAfter)

	exceeded, _, err = CheckDailyTokenQuota(1, 100)
	require.NoError(t, err)
	assert.False(t, exceeded)

	require.NoError(t, RecordTokenUsage(1, Usage{InputTokens: 60, OutputTokens: 50}))
	exceeded, retryAfter, err = CheckDailyTokenQuota(1, 100)
	require.NoError(t, err)
	assert.True(t, exceeded)
	assert.Positive(t, retryAfter)

	exceeded, _, err = CheckDailyTokenQuota(2, 100)
	require.NoError(t, err)
	assert.False(t, exceeded)

	require.NoError(t, RecordTokenUsage(2, Usage{}))
	exceeded, _, err = CheckDailyTokenQuota(2, 1)
	require.NoError(t, err)
	assert.False(t, exceeded)
}
