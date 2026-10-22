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
import { useQueryClient } from '@tanstack/react-query'
import { Loader2, Search, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { searchUsers } from '@/features/users/api'
import { formatTimestampToDate } from '@/lib/format'

import { grantActivityCode } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { useActivityCodes } from './activity-codes-provider'

type SelectedUser = {
  id: number
  username: string
  display_name?: string
  email?: string
}

// 管理员主动赠送活动码：搜索用户、批量勾选、一次提交，无需用户确认且不扣余额。
export function ActivityGrantDialog() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { open, setOpen, currentRow, triggerRefresh } = useActivityCodes()
  const [keyword, setKeyword] = useState('')
  const [results, setResults] = useState<SelectedUser[]>([])
  const [selected, setSelected] = useState<SelectedUser[]>([])
  const [remark, setRemark] = useState('')
  const [searching, setSearching] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const activityCode = currentRow
  const selectedIds = new Set(selected.map((user) => user.id))

  const reset = () => {
    setKeyword('')
    setResults([])
    setSelected([])
    setRemark('')
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      reset()
      setOpen(null)
    }
  }

  const handleSearch = async () => {
    const trimmed = keyword.trim()
    if (!trimmed) {
      toast.error(t('Enter a username, email or user ID to search'))
      return
    }
    setSearching(true)
    try {
      const response = await searchUsers({
        keyword: trimmed,
        p: 1,
        page_size: 20,
      })
      setResults((response.data?.items ?? []) as SelectedUser[])
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : t('Search failed'))
    } finally {
      setSearching(false)
    }
  }

  const toggleUser = (user: SelectedUser) => {
    setSelected((previous) =>
      previous.some((item) => item.id === user.id)
        ? previous.filter((item) => item.id !== user.id)
        : [...previous, user]
    )
  }

  const handleGrant = async () => {
    if (!activityCode) return
    if (selected.length === 0) {
      toast.error(t('Select at least one user'))
      return
    }
    setSubmitting(true)
    try {
      const result = await grantActivityCode(activityCode.id, {
        user_ids: selected.map((user) => user.id),
        remark: remark.trim(),
      })
      if (!result.success || !result.data) {
        toast.error(result.message || t(ERROR_MESSAGES.GRANT_FAILED))
        return
      }
      const { granted, failed, results } = result.data
      if (granted > 0) {
        toast.success(
          t(SUCCESS_MESSAGES.GRANTED, {
            count: granted,
          })
        )
      }
      if (failed > 0) {
        const firstFailure = results.find((item) => !item.success)
        toast.warning(
          t(SUCCESS_MESSAGES.GRANT_PARTIAL, {
            failed,
            message: firstFailure?.message || '',
          })
        )
      }
      await queryClient.invalidateQueries({
        queryKey: ['activity-code-grants', activityCode.id],
      })
      triggerRefresh()
      if (failed === 0) {
        reset()
        setOpen(null)
      } else {
        const failedIds = new Set(
          results.filter((item) => !item.success).map((item) => item.user_id)
        )
        setSelected((previous) =>
          previous.filter((user) => failedIds.has(user.id))
        )
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t(ERROR_MESSAGES.GRANT_FAILED)
      )
    } finally {
      setSubmitting(false)
    }
  }

  let expireText = t('Permanent')
  if (activityCode && activityCode.duration_seconds > 0) {
    const hours = activityCode.duration_seconds / 3600
    expireText = t('{{count}} hours', { count: hours })
  }

  return (
    <Dialog
      open={open === 'grant'}
      onOpenChange={handleOpenChange}
      title={t('Grant Activity Code')}
      description={
        activityCode
          ? t('Grant {{code}} to selected users without charging them.', {
              code: activityCode.code,
            })
          : undefined
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleGrant}
            disabled={submitting || selected.length === 0}
          >
            {submitting
              ? t('Granting...')
              : t('Grant to {{count}} users', { count: selected.length })}
          </Button>
        </>
      }
    >
      {activityCode && (
        <div className='bg-muted/40 space-y-1 rounded-lg border p-3 text-sm'>
          <p className='font-medium'>{activityCode.name}</p>
          <p className='text-muted-foreground text-xs'>
            {activityCode.group_name} · {t('Validity')}: {expireText}
            {activityCode.start_time > 0
              ? ` · ${t('From')} ${formatTimestampToDate(activityCode.start_time)}`
              : ''}
            {activityCode.end_time > 0
              ? ` · ${t('Until')} ${formatTimestampToDate(activityCode.end_time)}`
              : ''}
          </p>
        </div>
      )}

      <div className='space-y-2'>
        <Label htmlFor='activity-grant-search'>{t('Users')}</Label>
        <div className='flex gap-2'>
          <Input
            id='activity-grant-search'
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault()
                void handleSearch()
              }
            }}
            placeholder={t('Search by username, email or user ID')}
          />
          <Button
            type='button'
            variant='outline'
            onClick={() => void handleSearch()}
            disabled={searching}
          >
            {searching ? (
              <Loader2 className='h-4 w-4 animate-spin' />
            ) : (
              <Search className='h-4 w-4' />
            )}
            {t('Search')}
          </Button>
        </div>

        {selected.length > 0 && (
          <div className='flex flex-wrap gap-1.5'>
            {selected.map((user) => (
              <Badge key={user.id} variant='secondary' className='gap-1'>
                {user.username}
                <button
                  type='button'
                  onClick={() => toggleUser(user)}
                  aria-label={t('Remove {{username}}', {
                    username: user.username,
                  })}
                >
                  <X className='h-3 w-3' />
                </button>
              </Badge>
            ))}
          </div>
        )}

        {results.length > 0 && (
          <ScrollArea className='h-48 rounded-md border'>
            <div className='divide-y'>
              {results.map((user) => (
                <label
                  key={user.id}
                  className='hover:bg-muted/50 flex cursor-pointer items-center gap-3 px-3 py-2 text-sm'
                >
                  <Checkbox
                    checked={selectedIds.has(user.id)}
                    onCheckedChange={() => toggleUser(user)}
                  />
                  <span className='min-w-0 flex-1 truncate'>
                    {user.display_name || user.username}
                  </span>
                  <span className='text-muted-foreground shrink-0 text-xs'>
                    #{user.id}
                  </span>
                </label>
              ))}
            </div>
          </ScrollArea>
        )}
      </div>

      <div className='space-y-2'>
        <Label htmlFor='activity-grant-remark'>{t('Remark')}</Label>
        <Input
          id='activity-grant-remark'
          value={remark}
          onChange={(event) => setRemark(event.target.value)}
          placeholder={t('Optional note for this grant')}
        />
      </div>
    </Dialog>
  )
}
