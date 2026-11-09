/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm/clause"
)

const (
	// LogMigrationCompletedOptionKey marks that legacy usage logs in the primary
	// database were fully copied into the separated log database.
	LogMigrationCompletedOptionKey = "LogMigrationCompleted"

	// legacyLogIdReserveGap keeps ids of logs written to a fresh log database
	// far above every legacy id, so migrated rows keep their original ids
	// without colliding and "id desc" ordering stays chronological.
	legacyLogIdReserveGap = 1000000

	// logIdReservationContent marks the SQLite placeholder row that pins the
	// rowid sequence above legacy ids until migration completes.
	logIdReservationContent = "__log_id_reservation__"
)

var ErrLogDatabaseNotSeparated = errors.New("log database is not separated from the primary database")

// IsLogDatabaseSeparated reports whether usage logs are stored in a database
// other than the primary one.
func IsLogDatabaseSeparated() bool {
	return LOG_DB != nil && DB != nil && LOG_DB != DB
}

func IsLogMigrationCompleted() bool {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[LogMigrationCompletedOptionKey] == "true"
}

// HasLegacyLogTable reports whether the primary database still holds a logs
// table left over from before the log database was separated.
func HasLegacyLogTable() bool {
	return IsLogDatabaseSeparated() && DB.Migrator().HasTable(&Log{})
}

// reserveLogIdsAboveLegacy runs once when a separated log database is still
// empty: it moves the id sequence above the largest legacy id plus a gap so
// that logs written before/while migrating never collide with legacy ids.
func reserveLogIdsAboveLegacy() error {
	if !HasLegacyLogTable() || common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return nil
	}
	var logCount int64
	if err := LOG_DB.Model(&Log{}).Count(&logCount).Error; err != nil {
		return err
	}
	if logCount > 0 {
		return nil
	}
	var maxLegacyId int64
	if err := DB.Model(&Log{}).Select("COALESCE(MAX(id), 0)").Scan(&maxLegacyId).Error; err != nil {
		return err
	}
	if maxLegacyId <= 0 {
		return nil
	}
	nextId := maxLegacyId + legacyLogIdReserveGap
	switch common.LogDatabaseType() {
	case common.DatabaseTypeMySQL:
		if err := LOG_DB.Exec(fmt.Sprintf("ALTER TABLE logs AUTO_INCREMENT = %d", nextId)).Error; err != nil {
			return err
		}
	case common.DatabaseTypePostgreSQL:
		if err := LOG_DB.Exec("SELECT setval(pg_get_serial_sequence('logs', 'id'), ?, false)", nextId).Error; err != nil {
			return err
		}
	default:
		// SQLite rowid tables allocate max(rowid)+1, so the placeholder must stay
		// until real logs exceed it; it is removed when migration completes.
		placeholder := &Log{
			Id:        int(nextId - 1),
			CreatedAt: 1, // keep it out of recent log views
			Type:      LogTypeUnknown,
			Content:   logIdReservationContent,
			AdminOnly: true,
		}
		if err := LOG_DB.Create(placeholder).Error; err != nil {
			return err
		}
	}
	common.SysLog(fmt.Sprintf("log database ids reserved above legacy logs, next id starts at %d", nextId))
	return nil
}

// CountLegacyLogs counts legacy logs in the primary database with ids below
// beforeId (all rows when beforeId <= 0).
func CountLegacyLogs(ctx context.Context, beforeId int) (int64, error) {
	if !HasLegacyLogTable() {
		return 0, nil
	}
	tx := DB.WithContext(ctx).Model(&Log{})
	if beforeId > 0 {
		tx = tx.Where("id < ?", beforeId)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// MigrateLegacyLogBatch copies up to limit legacy logs with ids below beforeId
// (newest first) from the primary database into the log database, preserving
// ids. Re-copying a row is a no-op, so a failed migration can simply restart.
// When deleteSource is true the copied rows are removed from the primary
// database. It returns the number of rows read and the smallest id read.
func MigrateLegacyLogBatch(ctx context.Context, beforeId int, limit int, deleteSource bool) (int, int, error) {
	if !IsLogDatabaseSeparated() {
		return 0, 0, ErrLogDatabaseNotSeparated
	}
	if !DB.Migrator().HasTable(&Log{}) {
		return 0, 0, nil
	}
	tx := DB.WithContext(ctx).Model(&Log{})
	if beforeId > 0 {
		tx = tx.Where("id < ?", beforeId)
	}
	var logs []*Log
	if err := tx.Order("id desc").Limit(limit).Find(&logs).Error; err != nil {
		return 0, 0, err
	}
	if len(logs) == 0 {
		return 0, 0, nil
	}

	insert := LOG_DB.WithContext(ctx)
	if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		insert = insert.Clauses(clause.OnConflict{DoNothing: true})
	}
	if err := insert.CreateInBatches(logs, 200).Error; err != nil {
		return 0, 0, err
	}

	minId := logs[len(logs)-1].Id
	if deleteSource {
		ids := make([]int, 0, len(logs))
		for _, entry := range logs {
			ids = append(ids, entry.Id)
		}
		if err := DB.WithContext(ctx).Where("id IN ?", ids).Delete(&Log{}).Error; err != nil {
			return 0, 0, err
		}
	}
	return len(logs), minId, nil
}

// MarkLogMigrationCompleted records completion and drops the SQLite id
// reservation placeholder, which is no longer needed once legacy ids exist in
// the log database.
func MarkLogMigrationCompleted() error {
	if err := LOG_DB.Where("content = ? AND type = ? AND user_id = ?", logIdReservationContent, LogTypeUnknown, 0).Delete(&Log{}).Error; err != nil {
		return err
	}
	return UpdateOption(LogMigrationCompletedOptionKey, "true")
}

// ReclaimLegacyLogTable drops the legacy logs table from the primary database
// after migration completed, and compacts SQLite files to release disk space.
func ReclaimLegacyLogTable() error {
	if !IsLogDatabaseSeparated() {
		return ErrLogDatabaseNotSeparated
	}
	if !IsLogMigrationCompleted() {
		return errors.New("legacy log migration has not completed")
	}
	if !DB.Migrator().HasTable(&Log{}) {
		return nil
	}
	if err := DB.Migrator().DropTable(&Log{}); err != nil {
		return err
	}
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return DB.Exec("VACUUM").Error
	}
	return nil
}
