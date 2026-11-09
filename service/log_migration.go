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
package service

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	logMigrationDefaultBatchSize = 1000
	logMigrationMaxBatchSize     = 5000
	// logMigrationBatchPause yields between batches so the copy never starves
	// online traffic on either database.
	logMigrationBatchPause = 50 * time.Millisecond
)

type LogMigrationPayload struct {
	BatchSize    int  `json:"batch_size"`
	DeleteSource bool `json:"delete_source"`
}

type LogMigrationState struct {
	CursorId  int   `json:"cursor_id"`
	Total     int64 `json:"total"`
	Processed int64 `json:"processed"`
	Progress  int   `json:"progress"`
	Remaining int64 `json:"remaining"`
}

type LogMigrationResult struct {
	MigratedCount int64 `json:"migrated_count"`
}

type logMigrationHandler struct{}

func (logMigrationHandler) Type() string { return model.SystemTaskTypeLogMigration }

func (logMigrationHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	runLogMigrationTask(ctx, task, runnerID)
}

func init() {
	RegisterSystemTaskHandler(logMigrationHandler{})
}

// StartLogMigrationTask enqueues copying legacy usage logs from the primary
// database into the separated log database. An active task is returned as is.
func StartLogMigrationTask(batchSize int, deleteSource bool) (*model.SystemTask, error) {
	if !model.IsLogDatabaseSeparated() {
		return nil, model.ErrLogDatabaseNotSeparated
	}
	if batchSize <= 0 {
		batchSize = logMigrationDefaultBatchSize
	}
	if batchSize > logMigrationMaxBatchSize {
		batchSize = logMigrationMaxBatchSize
	}
	payload := LogMigrationPayload{BatchSize: batchSize, DeleteSource: deleteSource}
	task, _, err := EnqueueSystemTask(model.SystemTaskTypeLogMigration, payload)
	return task, err
}

func runLogMigrationTask(ctx context.Context, task *model.SystemTask, runnerID string) {
	payload := LogMigrationPayload{}
	if err := task.DecodePayload(&payload); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if payload.BatchSize <= 0 || payload.BatchSize > logMigrationMaxBatchSize {
		payload.BatchSize = logMigrationDefaultBatchSize
	}
	state := LogMigrationState{}
	if err := task.DecodeState(&state); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}

	remaining, err := model.CountLegacyLogs(ctx, state.CursorId)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if state.Total <= 0 {
		state.Total = remaining
	}
	state.Remaining = remaining
	state.Progress = logCleanupProgress(state.Processed, state.Total)
	if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
		logSystemTaskLockError(ctx, task, err)
		return
	}

	for {
		if ctx.Err() != nil {
			logSystemTaskLockError(ctx, task, ctx.Err())
			return
		}
		count, minId, err := model.MigrateLegacyLogBatch(ctx, state.CursorId, payload.BatchSize, payload.DeleteSource)
		if err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		if count == 0 {
			break
		}
		state.CursorId = minId
		state.Processed += int64(count)
		if state.Total < state.Processed {
			state.Total = state.Processed
		}
		state.Remaining = state.Total - state.Processed
		state.Progress = logCleanupProgress(state.Processed, state.Total)
		if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
			logSystemTaskLockError(ctx, task, err)
			return
		}
		if count < payload.BatchSize {
			break
		}
		time.Sleep(logMigrationBatchPause)
	}

	if err := model.MarkLogMigrationCompleted(); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	state.Remaining = 0
	state.Progress = 100
	if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
		logSystemTaskLockError(ctx, task, err)
		return
	}
	result := LogMigrationResult{MigratedCount: state.Processed}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}

// LogMigrationStatus describes where usage logs live and how much legacy data
// remains in the primary database.
type LogMigrationStatus struct {
	Separated   bool   `json:"separated"`
	LogDbType   string `json:"log_db_type"`
	LegacyTable bool   `json:"legacy_table"`
	LegacyCount int64  `json:"legacy_count"`
	Completed   bool   `json:"completed"`
}

func GetLogMigrationStatus(ctx context.Context) (*LogMigrationStatus, error) {
	status := &LogMigrationStatus{
		Separated:   model.IsLogDatabaseSeparated(),
		LegacyTable: model.HasLegacyLogTable(),
		Completed:   model.IsLogMigrationCompleted(),
	}
	status.LogDbType = string(common.LogDatabaseType())
	if !status.LegacyTable {
		return status, nil
	}
	count, err := model.CountLegacyLogs(ctx, 0)
	if err != nil {
		return nil, err
	}
	status.LegacyCount = count
	return status, nil
}

// ReclaimLegacyLogs drops the legacy logs table after migration completed. It
// refuses while a migration task is still active.
func ReclaimLegacyLogs() error {
	activeTask, err := model.GetActiveSystemTask(model.SystemTaskTypeLogMigration)
	if err != nil {
		return err
	}
	if activeTask != nil {
		return errors.New("log migration task is still running")
	}
	return model.ReclaimLegacyLogTable()
}
