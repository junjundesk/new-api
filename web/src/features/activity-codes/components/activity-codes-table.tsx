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
import { getRouteApi } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDataTable,
} from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { getActivityCodes, searchActivityCodes } from '../api'
import {
  ACTIVITY_CODE_STATUS,
  ERROR_MESSAGES,
  getActivityStatusFilterOptions,
} from '../constants'
import type { ActivityCode } from '../types'
import { useActivityCodesColumns } from './activity-codes-columns'
import { useActivityCodes } from './activity-codes-provider'

const route = getRouteApi('/_authenticated/activity-codes/')

function isDisabledActivityCodeRow(activityCode: ActivityCode) {
  return activityCode.status !== ACTIVITY_CODE_STATUS.ENABLED
}

export function ActivityCodesTable() {
  const { t } = useTranslation()
  const columns = useActivityCodesColumns()
  const { refreshTrigger, setOpen } = useActivityCodes()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [{ columnId: 'status', searchKey: 'status', type: 'array' }],
  })
  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const statusFilterValue = statusFilter[0] ?? ''

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'activity-codes',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      statusFilterValue,
      refreshTrigger,
    ],
    queryFn: async () => {
      const hasFilter = Boolean(globalFilter?.trim())
      const hasStatusFilter = statusFilterValue !== ''
      const params = {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }

      const result =
        hasFilter || hasStatusFilter
          ? await searchActivityCodes({
              ...params,
              keyword: globalFilter,
              status: statusFilterValue,
            })
          : await getActivityCodes(params)

      if (!result.success) {
        toast.error(
          result.message ||
            t(
              hasFilter || hasStatusFilter
                ? ERROR_MESSAGES.SEARCH_FAILED
                : ERROR_MESSAGES.LOAD_FAILED
            )
        )
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const activityCodes = data?.items || []

  const { table } = useDataTable({
    data: activityCodes,
    columns,
    enableRowSelection: true,
    columnFilters,
    globalFilter,
    pagination,
    globalFilterFn: (row, _columnId, filterValue) => {
      const code = String(row.getValue('code')).toLowerCase()
      const name = String(row.getValue('name')).toLowerCase()
      const searchValue = String(filterValue).toLowerCase()
      return code.includes(searchValue) || name.includes(searchValue)
    },
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  const statusOptions = useMemo(() => getActivityStatusFilterOptions(t), [t])
  const normalizedFilter = globalFilter?.trim() ?? ''
  const hasActiveFilters = normalizedFilter.length > 0 || !!statusFilterValue
  const emptyTitle = hasActiveFilters
    ? t('No matching results')
    : t('No Activity Codes Found')
  let emptyDescription = t(
    'No activity codes yet. Create one to grant a temporary group to your users.'
  )
  if (normalizedFilter) {
    emptyDescription = t(
      'No results for "{{query}}". Try adjusting your search or filters.',
      { query: normalizedFilter }
    )
  } else if (hasActiveFilters) {
    emptyDescription = t('No records found. Try adjusting your filters.')
  }

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={emptyTitle}
      emptyDescription={emptyDescription}
      emptyAction={
        !hasActiveFilters ? (
          <Button size='sm' onClick={() => setOpen('create')}>
            <Plus className='size-4' />
            {t('Create Activity Code')}
          </Button>
        ) : undefined
      }
      skeletonKeyPrefix='activity-codes-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Filter by code or name...'),
        searchDebounceMs: 500,
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: statusOptions,
            singleSelect: true,
          },
        ],
      }}
      getRowClassName={(row, { isMobile: mobile }) => {
        if (!isDisabledActivityCodeRow(row.original)) return undefined
        return mobile ? DISABLED_ROW_MOBILE : DISABLED_ROW_DESKTOP
      }}
    />
  )
}
