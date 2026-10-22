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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import type { ExcludedUser } from '@/features/dashboard/types'
import { searchUsers } from '@/features/users/api'
import { useDebounce } from '@/hooks'
import { cn } from '@/lib/utils'

const SEARCH_PAGE_SIZE = 20
const SEARCH_DEBOUNCE_MS = 300

interface ExcludedUsersPickerProps {
  selected: ExcludedUser[]
  onChange: (users: ExcludedUser[]) => void
}

/**
 * Search-and-pick control for users whose usage must stay out of the model
 * analytics aggregates. It only ever produces a list of ids; the server does
 * the actual exclusion so totals, charts and the breakdown never disagree.
 */
export function ExcludedUsersPicker(props: ExcludedUsersPickerProps) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebounce(keyword.trim(), SEARCH_DEBOUNCE_MS)

  const searchQuery = useQuery({
    queryKey: ['dashboard', 'excluded-users', debouncedKeyword],
    queryFn: () =>
      searchUsers({
        keyword: debouncedKeyword,
        p: 1,
        page_size: SEARCH_PAGE_SIZE,
      }),
    enabled: debouncedKeyword.length > 0,
    staleTime: 30_000,
    retry: false,
  })

  const selectedIds = useMemo(
    () => new Set(props.selected.map((user) => user.id)),
    [props.selected]
  )
  const results = debouncedKeyword ? (searchQuery.data?.data?.items ?? []) : []

  const toggleUser = (id: number, username: string) => {
    if (selectedIds.has(id)) {
      props.onChange(props.selected.filter((user) => user.id !== id))
      return
    }
    props.onChange([...props.selected, { id, username }])
  }

  let resultContent: React.ReactNode
  if (searchQuery.isLoading) {
    resultContent = (
      <div className='text-muted-foreground flex items-center justify-center gap-2 px-2 py-3 text-xs'>
        <Spinner className='size-3' />
        {t('Searching...')}
      </div>
    )
  } else if (results.length === 0) {
    resultContent = (
      <div className='text-muted-foreground px-2 py-3 text-center text-xs'>
        {t('No users found')}
      </div>
    )
  } else {
    resultContent = results.map((user) => {
      const checked = selectedIds.has(user.id)
      const label =
        user.display_name && user.display_name !== user.username
          ? `${user.username} (${user.display_name})`
          : user.username
      return (
        <button
          key={user.id}
          type='button'
          aria-pressed={checked}
          onClick={() => toggleUser(user.id, user.username)}
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
    <div className='grid gap-2'>
      <div className='relative'>
        <Search
          className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2'
          aria-hidden='true'
        />
        <Input
          aria-label={t('Search users to exclude')}
          placeholder={t('Search users to exclude')}
          className='pl-8'
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
        />
      </div>

      {props.selected.length > 0 && (
        <div className='flex flex-wrap gap-1'>
          {props.selected.map((user) => (
            <Badge key={user.id} variant='secondary' className='gap-1'>
              <span className='max-w-40 truncate'>{user.username}</span>
              <button
                type='button'
                aria-label={t('Remove {{username}}', {
                  username: user.username,
                })}
                className='hover:text-destructive cursor-pointer'
                onClick={() =>
                  props.onChange(
                    props.selected.filter((item) => item.id !== user.id)
                  )
                }
              >
                <X className='size-3' aria-hidden='true' />
              </button>
            </Badge>
          ))}
        </div>
      )}

      {debouncedKeyword.length > 0 && (
        <div className='max-h-40 overflow-y-auto rounded-md border'>
          {resultContent}
        </div>
      )}
    </div>
  )
}
