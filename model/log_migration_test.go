package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// useSeparatedLogDB points LOG_DB at a fresh in-memory SQLite database for the
// duration of the test, keeping the shared DB as the legacy primary database.
func useSeparatedLogDB(t *testing.T) {
	t.Helper()
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := logDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, logDB.AutoMigrate(&Log{}))

	require.NoError(t, DB.AutoMigrate(&Option{}))
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMapRWMutex.Unlock()

	originalLogDB := LOG_DB
	LOG_DB = logDB
	t.Cleanup(func() {
		LOG_DB = originalLogDB
		_ = sqlDB.Close()
		common.OptionMapRWMutex.Lock()
		delete(common.OptionMap, LogMigrationCompletedOptionKey)
		common.OptionMapRWMutex.Unlock()
		DB.Where("key = ?", LogMigrationCompletedOptionKey).Delete(&Option{})
	})
}

func seedLegacyLogs(t *testing.T, ids ...int) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM logs").Error)
	for _, id := range ids {
		require.NoError(t, DB.Create(&Log{Id: id, UserId: 1, CreatedAt: int64(1000 + id), Type: LogTypeConsume, Content: "legacy"}).Error)
	}
	t.Cleanup(func() { DB.Exec("DELETE FROM logs") })
}

func TestReserveLogIdsAboveLegacyKeepsNewIdsClearOfLegacyIds(t *testing.T) {
	seedLegacyLogs(t, 1, 2, 30)
	useSeparatedLogDB(t)

	require.NoError(t, reserveLogIdsAboveLegacy())

	fresh := &Log{UserId: 1, CreatedAt: 5000, Type: LogTypeConsume, Content: "new"}
	require.NoError(t, LOG_DB.Create(fresh).Error)
	assert.Equal(t, 30+legacyLogIdReserveGap, fresh.Id)
}

func TestReserveLogIdsAboveLegacySkipsNonEmptyLogDatabase(t *testing.T) {
	seedLegacyLogs(t, 1, 2, 30)
	useSeparatedLogDB(t)
	require.NoError(t, LOG_DB.Create(&Log{Id: 7, Content: "existing"}).Error)

	require.NoError(t, reserveLogIdsAboveLegacy())

	var count int64
	require.NoError(t, LOG_DB.Model(&Log{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestMigrateLegacyLogBatchCopiesNewestFirstAndPreservesIds(t *testing.T) {
	seedLegacyLogs(t, 1, 2, 3, 4, 5)
	useSeparatedLogDB(t)
	ctx := context.Background()

	count, minId, err := MigrateLegacyLogBatch(ctx, 0, 3, false)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, 3, minId)

	// Re-running an overlapping range must not duplicate rows.
	_, _, err = MigrateLegacyLogBatch(ctx, 0, 3, false)
	require.NoError(t, err)

	count, minId, err = MigrateLegacyLogBatch(ctx, minId, 3, true)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, 1, minId)

	var migratedIds []int
	require.NoError(t, LOG_DB.Model(&Log{}).Order("id").Pluck("id", &migratedIds).Error)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, migratedIds)

	var legacyIds []int
	require.NoError(t, DB.Model(&Log{}).Order("id").Pluck("id", &legacyIds).Error)
	assert.Equal(t, []int{3, 4, 5}, legacyIds, "only the batch copied with deleteSource is removed")

	remaining, err := CountLegacyLogs(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), remaining)
}

func TestMarkLogMigrationCompletedRemovesIdReservation(t *testing.T) {
	seedLegacyLogs(t, 10)
	useSeparatedLogDB(t)
	require.NoError(t, reserveLogIdsAboveLegacy())

	require.NoError(t, MarkLogMigrationCompleted())

	var count int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("content = ?", logIdReservationContent).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	assert.True(t, IsLogMigrationCompleted())
}

func TestReclaimLegacyLogTableRequiresCompletedMigration(t *testing.T) {
	seedLegacyLogs(t, 1)
	useSeparatedLogDB(t)

	require.Error(t, ReclaimLegacyLogTable())
	assert.True(t, DB.Migrator().HasTable(&Log{}))
}

func TestMigrateLegacyLogBatchRejectsSharedLogDatabase(t *testing.T) {
	_, _, err := MigrateLegacyLogBatch(context.Background(), 0, 10, false)
	assert.ErrorIs(t, err, ErrLogDatabaseNotSeparated)
}
