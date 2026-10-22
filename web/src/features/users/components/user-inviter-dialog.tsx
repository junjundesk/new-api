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
import { useQuery } from '@tanstack/react-query'
import { Search, X } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { useDebounce } from '@/hooks'
import { cn } from '@/lib/utils'

import { bindUserInviter, searchUsers } from '../api'
import type { User } from '../types'

const SEARCH_PAGE_SIZE = 20
const SEARCH_DEBOUNCE_MS = 300

interface UserInviterDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userIds: number[]
  onSuccess: () => void
}

/**
 * Admin control that points one inviter at every selected user. It only edits
 * the referral relationship; the server never replays signup rewards or past
 * top-up commissions for a manual assignment.
 */
export function UserInviterDialog(props: UserInviterDialogProps) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const [selected, setSelected] = useState<User | null>(null)
  const [isBinding, setIsBinding] = useState(false)
  const debouncedKeyword = useDebounce(keyword.trim(), SEARCH_DEBOUNCE_MS)

  const searchQuery = useQuery({
    queryKey: ['users', 'inviter-candidates', debouncedKeyword],
    queryFn: () =>
      searchUsers({
        keyword: debouncedKeyword,
        p: 1,
        page_size: SEARCH_PAGE_SIZE,
      }),
    enabled: props.open && debouncedKeyword.length > 0,
    staleTime: 30_000,
    retry: false,
  })

  // Every opening starts from a clean slate so a previous selection cannot be
  // submitted against a different set of users.
  useEffect(() => {
    if (props.open) {
      setKeyword('')
      setSelected(null)
    }
  }, [props.open])

  const targetIds = useMemo(() => new Set(props.userIds), [props.userIds])
  const candidates = (searchQuery.data?.data?.items ?? []).filter(
    (user) => !user.DeletedAt && !targetIds.has(user.id)
  )

  const handleBind = async () => {
    if (!selected || props.userIds.length === 0) return

    setIsBinding(true)
    try {
      const result = await bindUserInviter({
        user_ids: props.userIds,
        inviter_id: selected.id,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to bind inviter'))
        return
      }
      toast.success(
        t('Bound {{count}} user(s) to inviter {{username}}', {
          count: result.data?.updated ?? props.userIds.length,
          username: selected.username,
        })
      )
      props.onOpenChange(false)
      props.onSuccess()
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to bind inviter')
      )
    } finally {
      setIsBinding(false)
    }
  }

  let resultContent: React.ReactNode
  if (searchQuery.isLoading) {
    resultContent = (
      <div className='text-muted-foreground flex items-center justify-center gap-2 px-2 py-3 text-xs'>
        <Spinner className='size-3' />
        {t('Searching...')}
      </div>
    )
  } else if (candidates.length === 0) {
    resultContent = (
      <div className='text-muted-foreground px-2 py-3 text-center text-xs'>
        {t('No users found')}
      </div>
    )
  } else {
    resultContent = candidates.map((user) => {
      const checked = selected?.id === user.id
      const label =
        user.display_name && user.display_name !== user.username
          ? `${user.username} (${user.display_name})`
          : user.username
      return (
        <button
          key={user.id}
          type='button'
          aria-pressed={checked}
          onClick={() => setSelected(user)}
          className={cn(
            'hover:bg-accent flex w-full items-center justify-between gap-2 px-2 py-1.5 text-left text-xs',
            checked && 'bg-accent/60'
          )}
        >
          <span className='min-w-0 truncate'>{label}</span>
          <span className='text-muted-foreground shrink-0 font-mono'>
            #{user.id}
          </span>
        </button>
      )
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Bind Inviter')}
      description={t(
        'Set the inviter for {{count}} selected user(s). Existing inviter bindings will be overwritten and no invitation reward is granted retroactively.',
        { count: props.userIds.length }
      )}
      contentHeight='auto'
      bodyClassName='space-y-3'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={isBinding}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleBind}
            disabled={!selected || props.userIds.length === 0 || isBinding}
          >
            {isBinding ? t('Binding...') : t('Bind Inviter')}
          </Button>
        </>
      }
    >
      <div className='grid gap-2'>
        <div className='relative'>
          <Search
            className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2'
            aria-hidden='true'
          />
          <Input
            aria-label={t('Search users')}
            placeholder={t('Search by username, email or ID')}
            className='pl-8'
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
          />
        </div>

        {selected && (
          <div className='flex flex-wrap gap-1'>
            <Badge variant='secondary' className='gap-1'>
              <span className='max-w-48 truncate'>{selected.username}</span>
              <button
                type='button'
                aria-label={t('Remove {{username}}', {
                  username: selected.username,
                })}
                className='hover:text-destructive cursor-pointer'
                onClick={() => setSelected(null)}
              >
                <X className='size-3' aria-hidden='true' />
              </button>
            </Badge>
          </div>
        )}

        {debouncedKeyword.length > 0 && (
          <div className='max-h-56 overflow-y-auto rounded-md border'>
            {resultContent}
          </div>
        )}
      </div>
    </Dialog>
  )
}
