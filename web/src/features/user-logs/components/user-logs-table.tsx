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
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { Input } from '@/components/ui/input'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getUserLogs } from '../api'
import { USER_LOG_DEFAULT_RANGE_DAYS, USER_LOG_TYPE_FILTERS } from '../constants'
import { useUserLogsColumns } from './user-logs-columns'

const route = getRouteApi('/_authenticated/user-logs/')

const DAY_MS = 24 * 60 * 60 * 1000

export function UserLogsTable() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = route.useSearch()
  const isAdmin = useAuthStore((state) => (state.auth.user?.role ?? 0) >= ROLE.ADMIN)
  const isMobile = useMediaQuery('(max-width: 640px)')
  const columns = useUserLogsColumns(isAdmin)

  const usernameFilter = typeof search.username === 'string' ? search.username : ''
  const [usernameDraft, setUsernameDraft] = useState(usernameFilter)
  useEffect(() => {
    setUsernameDraft(usernameFilter)
  }, [usernameFilter])

  const startMs = useMemo(
    () =>
      typeof search.startTime === 'number'
        ? search.startTime
        : Date.now() - USER_LOG_DEFAULT_RANGE_DAYS * DAY_MS,
    [search.startTime]
  )
  const endMs = useMemo(
    () => (typeof search.endTime === 'number' ? search.endTime : Date.now()),
    [search.endTime]
  )

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 20 : 50 },
    globalFilter: { enabled: false },
    columnFilters: [{ columnId: 'type', searchKey: 'type', type: 'array' }],
  })
  const typeFilter =
    (columnFilters.find((filter) => filter.id === 'type')?.value as
      | string[]
      | undefined) ?? []
  const selectedType = typeFilter[0]

  const patchSearch = (patch: Record<string, unknown>) => {
    void navigate({
      to: '/user-logs',
      search: { ...search, ...patch, page: undefined },
      replace: true,
    })
  }

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'user-logs',
      isAdmin,
      pagination.pageIndex + 1,
      pagination.pageSize,
      selectedType,
      usernameFilter,
      startMs,
      endMs,
    ],
    queryFn: async () => {
      const result = await getUserLogs(
        {
          p: pagination.pageIndex + 1,
          page_size: pagination.pageSize,
          type: selectedType ? Number(selectedType) : undefined,
          username: isAdmin && usernameFilter ? usernameFilter : undefined,
          start_timestamp: Math.floor(startMs / 1000),
          end_timestamp: Math.floor(endMs / 1000),
        },
        isAdmin
      )

      if (!result?.success) {
        toast.error(result?.message || t('Failed to load logs'))
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const logs = data?.items ?? []

  const { table } = useDataTable({
    data: logs,
    columns,
    columnFilters,
    pagination,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    ensurePageInRange,
  })

  const typeOptions = useMemo(
    () =>
      USER_LOG_TYPE_FILTERS.filter((type) => type.value !== '0').map((type) => ({
        label: t(type.label),
        value: type.value,
      })),
    [t]
  )
  const hasTimeRange =
    typeof search.startTime === 'number' || typeof search.endTime === 'number'

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No user logs yet')}
      emptyDescription={t(
        'Account activity such as sign-in, registration, API keys and top-ups appears here.'
      )}
      skeletonKeyPrefix='user-logs-skeleton'
      toolbarProps={{
        customSearch: (
          <div className='flex flex-wrap items-center gap-2'>
            <CompactDateTimeRangePicker
              start={new Date(startMs)}
              end={new Date(endMs)}
              onChange={(range) =>
                patchSearch({
                  startTime: range.start ? range.start.getTime() : undefined,
                  endTime: range.end ? range.end.getTime() : undefined,
                })
              }
            />
            {isAdmin ? (
              <Input
                value={usernameDraft}
                placeholder={t('Filter by username...')}
                className='h-8 w-[180px]'
                onChange={(event) => setUsernameDraft(event.target.value)}
                onBlur={() => patchSearch({ username: usernameDraft.trim() })}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    patchSearch({ username: usernameDraft.trim() })
                  }
                }}
              />
            ) : null}
          </div>
        ),
        hasAdditionalFilters: hasTimeRange || usernameFilter !== '',
        onReset: () => {
          setUsernameDraft('')
          patchSearch({ username: undefined, startTime: undefined, endTime: undefined })
        },
        filters: [
          {
            columnId: 'type',
            title: t('Type'),
            options: typeOptions,
            singleSelect: true,
          },
        ],
      }}
    />
  )
}
