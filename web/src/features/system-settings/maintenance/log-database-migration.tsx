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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'

import {
  getCurrentLogMigrationTask,
  getLogMigrationStatus,
  getLogMigrationTask,
  reclaimLegacyLogs,
  startLogMigrationTask,
} from '../api'
import type { LogMigrationStatus, LogMigrationTask } from '../types'

function isActiveTask(task: LogMigrationTask | null) {
  return task?.status === 'pending' || task?.status === 'running'
}

export function LogDatabaseMigration() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<LogMigrationStatus | null>(null)
  const [task, setTask] = useState<LogMigrationTask | null>(null)
  const [deleteSource, setDeleteSource] = useState(false)
  const [isStarting, setIsStarting] = useState(false)
  const [isReclaiming, setIsReclaiming] = useState(false)

  const refreshStatus = useCallback(async () => {
    try {
      const res = await getLogMigrationStatus()
      if (res.success && res.data) setStatus(res.data)
    } catch {
      /* status is informational only */
    }
  }, [])

  useEffect(() => {
    refreshStatus()
    getCurrentLogMigrationTask()
      .then((res) => {
        if (res.success && res.data) setTask(res.data)
      })
      .catch(() => undefined)
  }, [refreshStatus])

  const active = isActiveTask(task)
  const taskId = task?.task_id

  useEffect(() => {
    if (!taskId || !active) return
    let cancelled = false
    const interval = window.setInterval(async () => {
      try {
        const res = await getLogMigrationTask(taskId)
        if (cancelled || !res.success || !res.data) return
        setTask(res.data)
        if (isActiveTask(res.data)) return
        if (res.data.status === 'succeeded') {
          toast.success(
            t('{{count}} legacy log entries migrated.', {
              count: res.data.result?.migrated_count ?? 0,
            })
          )
        } else if (res.data.status === 'failed') {
          toast.error(res.data.error || t('Log migration failed'))
        }
        refreshStatus()
      } catch {
        /* keep polling */
      }
    }, 1000)
    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [active, taskId, refreshStatus, t])

  const handleStart = async () => {
    setIsStarting(true)
    try {
      const res = await startLogMigrationTask(deleteSource)
      if (!res.success || !res.data) {
        throw new Error(res.message || t('Log migration failed'))
      }
      setTask(res.data)
      toast.success(t('Log migration task started.'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Log migration failed')
      )
    } finally {
      setIsStarting(false)
    }
  }

  const handleReclaim = async () => {
    setIsReclaiming(true)
    try {
      const res = await reclaimLegacyLogs()
      if (!res.success) {
        throw new Error(res.message || t('Failed to remove legacy logs'))
      }
      toast.success(t('Legacy log table removed from the main database.'))
      refreshStatus()
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to remove legacy logs')
      )
    } finally {
      setIsReclaiming(false)
    }
  }

  if (!status) return null

  const progress = Math.min(100, Math.max(0, task?.state?.progress ?? 0))

  return (
    <div className='space-y-4'>
      <div>
        <h4 className='font-medium'>{t('Log Database')}</h4>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t(
            'Usage logs can be stored in a dedicated database via LOG_SQL_DSN or LOG_SQLITE_PATH to keep the main database small.'
          )}
        </p>
      </div>

      {!status.separated ? (
        <Alert>
          <AlertDescription>
            {t(
              'Usage logs currently share the main database. Set LOG_SQL_DSN or LOG_SQLITE_PATH and restart to separate them.'
            )}
          </AlertDescription>
        </Alert>
      ) : (
        <div className='space-y-3'>
          <div className='grid grid-cols-2 gap-2 rounded-lg border p-4 text-sm md:grid-cols-3'>
            <div>
              <span className='text-muted-foreground'>
                {t('Log database type')}:
              </span>{' '}
              {status.log_db_type}
            </div>
            <div>
              <span className='text-muted-foreground'>
                {t('Legacy logs in main database')}:
              </span>{' '}
              {status.legacy_table ? status.legacy_count : t('None')}
            </div>
            <div>
              <span className='text-muted-foreground'>
                {t('Migration status')}:
              </span>{' '}
              {status.completed ? t('Completed') : t('Not completed')}
            </div>
          </div>

          {status.legacy_table && (
            <div className='flex flex-wrap items-center gap-4'>
              <div className='flex items-center gap-2'>
                <Switch
                  id='log-migration-delete-source'
                  checked={deleteSource}
                  onCheckedChange={setDeleteSource}
                  disabled={active}
                />
                <Label
                  htmlFor='log-migration-delete-source'
                  className='text-sm'
                >
                  {t('Delete migrated rows from the main database')}
                </Label>
              </div>
              <Button
                type='button'
                onClick={handleStart}
                disabled={isStarting || active}
              >
                {isStarting || active
                  ? t('Migrating...')
                  : t('Migrate legacy logs')}
              </Button>
              {status.completed && (
                <AlertDialog>
                  <AlertDialogTrigger
                    render={
                      <Button
                        type='button'
                        variant='destructive'
                        disabled={isReclaiming || active}
                      />
                    }
                  >
                    {isReclaiming
                      ? t('Removing...')
                      : t('Remove legacy log table')}
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>
                        {t('Remove legacy log table?')}
                      </AlertDialogTitle>
                      <AlertDialogDescription>
                        {t(
                          'The logs table in the main database will be dropped to free space. Migrated logs remain in the log database.'
                        )}{' '}
                        {t('This action cannot be undone.')}
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                      <AlertDialogAction
                        variant='destructive'
                        onClick={handleReclaim}
                      >
                        {t('Remove')}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}
            </div>
          )}

          {task && (
            <div className='rounded-md border p-3'>
              <div className='mb-2 flex items-center justify-between gap-3 text-sm'>
                <span className='font-medium'>
                  {t('Log migration progress')}
                </span>
                <span className='text-muted-foreground tabular-nums'>
                  {progress}%
                </span>
              </div>
              <Progress value={progress} />
              <div className='text-muted-foreground mt-2 text-xs'>
                {t('{{processed}} of {{total}} log entries processed.', {
                  processed: task.state?.processed ?? 0,
                  total: task.state?.total ?? 0,
                })}
              </div>
              {task.status === 'failed' && task.error && (
                <div className='text-destructive mt-2 text-xs'>
                  {task.error}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
