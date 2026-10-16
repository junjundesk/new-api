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
import { AlertTriangle, Loader2, Search } from 'lucide-react'
import { useCallback, useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePagination,
  DataTableView,
  useDataTable,
} from '@/components/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getUserQuotaBreakdown } from '@/features/dashboard/api'
import { BREAKDOWN_TIME_PRESETS } from '@/features/dashboard/constants'
import {
  buildBreakdownFilterOptions,
  filterBreakdownRows,
  resolveBreakdownTimeRange,
  summarizeBreakdownRows,
} from '@/features/dashboard/lib'
import type {
  BreakdownFilters,
  BreakdownTimePreset,
} from '@/features/dashboard/types'
import {
  formatCompactNumber,
  formatNumber,
  formatQuota,
} from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { PanelWrapper } from '../ui/panel-wrapper'
import { useBreakdownColumns } from './breakdown-columns'
import { BreakdownFilterSelect } from './breakdown-filters'

interface UserConsumptionBreakdownProps {
  filters: BreakdownFilters
  onFiltersChange: (filters: BreakdownFilters) => void
}

interface BreakdownNoticeProps {
  children: ReactNode
}

function BreakdownNotice(props: BreakdownNoticeProps) {
  return (
    <Alert className='text-warning rounded-none border-x-0 border-t-0 px-4 py-2'>
      <AlertTriangle />
      <AlertDescription className='text-xs'>{props.children}</AlertDescription>
    </Alert>
  )
}

export function UserConsumptionBreakdown(
  props: UserConsumptionBreakdownProps
) {
  const { t } = useTranslation()
  const role = useAuthStore((state) => state.auth.user?.role)
  const isAdmin = Boolean(role && role >= ROLE.ADMIN)
  const { filters, onFiltersChange } = props

  // The panel owns its range instead of reusing the chart filters, so that
  // "all time" can never drag the charts into a full-table scan.
  const timeRange = useMemo(
    () => resolveBreakdownTimeRange(filters.preset),
    [filters.preset]
  )

  const { data, isLoading, isError } = useQuery({
    queryKey: ['dashboard', 'user-breakdown', timeRange, isAdmin],
    queryFn: () => getUserQuotaBreakdown(timeRange, isAdmin),
    select: (res) => (res.success ? res.data : null),
    staleTime: 60_000,
  })

  const rows = useMemo(() => data?.rows ?? [], [data])
  const filterOptions = useMemo(() => buildBreakdownFilterOptions(rows), [rows])
  const visibleRows = useMemo(
    () => filterBreakdownRows(rows, filters),
    [rows, filters]
  )
  const summary = useMemo(
    () => summarizeBreakdownRows(visibleRows),
    [visibleRows]
  )
  const hasDimensionFilter = Boolean(
    filters.username ||
      filters.use_group ||
      filters.model_name ||
      filters.query.trim()
  )

  const handleSelectUsername = useCallback(
    (username: string) => {
      onFiltersChange({ ...filters, username })
    },
    [filters, onFiltersChange]
  )

  const handleFilterChange = useCallback(
    (key: 'username' | 'use_group' | 'model_name' | 'query', value: string) => {
      onFiltersChange({ ...filters, [key]: value })
    },
    [filters, onFiltersChange]
  )

  const columns = useBreakdownColumns({
    showUsername: isAdmin,
    totalQuota: summary.quota,
    onSelectUsername: handleSelectUsername,
  })

  const { table } = useDataTable({
    data: visibleRows,
    columns,
    initialSorting: [{ id: 'quota', desc: true }],
    initialPagination: { pageIndex: 0, pageSize: 20 },
  })

  const tiles = [
    { key: 'cost', label: t('Total Cost'), value: formatQuota(summary.quota) },
    {
      key: 'tokens',
      label: t('Total Tokens'),
      value: formatCompactNumber(summary.tokens),
    },
    {
      key: 'requests',
      label: t('Total Requests'),
      value: formatNumber(summary.requests),
    },
    ...(isAdmin
      ? [
          {
            key: 'users',
            label: t('Active Users'),
            value: formatNumber(summary.users),
          },
        ]
      : []),
  ]

  return (
    <PanelWrapper
      title={t('User Consumption Details')}
      description={t(
        'Consumption by user, group and model for the selected time range.'
      )}
      className='rounded-lg'
      contentClassName='p-0'
      headerActions={
        isLoading ? (
          <Loader2
            className='text-muted-foreground size-4 animate-spin'
            aria-hidden='true'
          />
        ) : undefined
      }
    >
      <div className='flex flex-wrap items-center gap-1.5 px-4 pt-3 sm:gap-2'>
        <Tabs
          value={filters.preset}
          onValueChange={(value) =>
            onFiltersChange({
              ...filters,
              preset: value as BreakdownTimePreset,
            })
          }
          className='shrink-0'
        >
          <TabsList>
            {BREAKDOWN_TIME_PRESETS.map((preset) => (
              <TabsTrigger
                key={preset.value}
                value={preset.value}
                className='px-2.5 text-xs'
              >
                {t(preset.labelKey)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {isAdmin && (
          <BreakdownFilterSelect
            label={t('Filter by username')}
            allLabel={t('All users')}
            searchPlaceholder={t('Filter by username')}
            options={filterOptions.users}
            value={filters.username}
            onChange={(value) => handleFilterChange('username', value)}
          />
        )}
        <BreakdownFilterSelect
          label={t('Filter by group')}
          allLabel={t('All Groups')}
          searchPlaceholder={t('Filter by group')}
          options={filterOptions.groups}
          value={filters.use_group}
          onChange={(value) => handleFilterChange('use_group', value)}
        />
        <BreakdownFilterSelect
          label={t('Filter by model')}
          allLabel={t('All Models')}
          searchPlaceholder={t('Filter by model')}
          options={filterOptions.models}
          value={filters.model_name}
          onChange={(value) => handleFilterChange('model_name', value)}
        />

        <div className='relative ms-auto min-w-40 flex-1 sm:max-w-64'>
          <Search
            className='text-muted-foreground pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2'
            aria-hidden='true'
          />
          <Input
            value={filters.query}
            onChange={(event) =>
              handleFilterChange('query', event.target.value)
            }
            placeholder={t('Search')}
            aria-label={t('Search')}
            className='h-8 pl-7 text-xs'
          />
        </div>
      </div>

      <div className='divide-border/60 mt-3 grid min-w-0 grid-cols-2 divide-x border-y sm:grid-cols-4'>
        {tiles.map((tile) => (
          <div key={tile.key} className='min-w-0 px-2.5 py-2 sm:px-5 sm:py-3'>
            <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wide uppercase'>
              {tile.label}
            </div>
            <div className='text-foreground mt-1 truncate font-mono text-sm font-semibold tabular-nums sm:text-lg'>
              {isError ? '--' : tile.value}
            </div>
          </div>
        ))}
      </div>

      {isError && <BreakdownNotice>{t('Failed to load')}</BreakdownNotice>}
      {data?.data_export_enabled === false && (
        <BreakdownNotice>
          {t(
            'Usage data collection is disabled; enable data export to collect this data.'
          )}
        </BreakdownNotice>
      )}
      {data?.truncated === true && (
        <BreakdownNotice>
          {t(
            'Only the top {{count}} rows by cost are shown. Narrow the time range for complete data.',
            { count: rows.length }
          )}
        </BreakdownNotice>
      )}

      <DataTableView
        table={table}
        isLoading={isLoading}
        emptyTitle={
          hasDimensionFilter
            ? t('No consumption in this range')
            : t('No data available')
        }
        skeletonRowHeight='h-8'
        containerClassName='rounded-none border-0'
      />

      <div className='flex flex-wrap items-center justify-between gap-2 px-4 py-3'>
        <p className='text-muted-foreground/70 text-[11px]'>
          {data && data.data_export_interval_minutes > 0
            ? t('Usage data is aggregated every {{count}} minutes.', {
                count: data.data_export_interval_minutes,
              })
            : null}
        </p>
        <DataTablePagination table={table} />
      </div>
    </PanelWrapper>
  )
}
