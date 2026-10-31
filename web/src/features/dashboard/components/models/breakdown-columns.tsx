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
import type { ColumnDef } from '@tanstack/react-table'
import { ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { DataTableColumnHeader } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import {
  breakdownShare,
  computeCacheRate,
  isGroupedBreakdownRow,
} from '@/features/dashboard/lib'
import type { UserQuotaBreakdownRow } from '@/features/dashboard/types'
import {
  formatCompactNumber,
  formatNumber,
  formatPercent,
  formatQuota,
} from '@/lib/format'
import { cn } from '@/lib/utils'

const EMPTY_DIMENSION = '—'

interface BreakdownColumnOptions {
  // Admin view collapses each user into one row; non-admins keep the flat
  // group x model table.
  grouped: boolean
  // Non-admins only ever see their own rows, so the user column is redundant.
  showUsername: boolean
  // Denominator of the share column: the cost of the current filter selection.
  totalQuota: number
  expandedRowKeys: ReadonlySet<string>
  getDetailPanelId: (key: string) => string
  onToggleRow: (key: string) => void
}

export function useBreakdownColumns(
  options: BreakdownColumnOptions
): ColumnDef<UserQuotaBreakdownRow, unknown>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<UserQuotaBreakdownRow, unknown>[] = []

  if (options.showUsername) {
    columns.push({
      accessorKey: 'username',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('User')} />
      ),
      cell: ({ row }) => {
        const original = row.original
        if (!isGroupedBreakdownRow(original)) {
          return (
            <span className='block max-w-40 truncate font-medium'>
              {original.username || EMPTY_DIMENSION}
            </span>
          )
        }

        const expanded = options.expandedRowKeys.has(original.key)
        return (
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='h-auto max-w-40 justify-start gap-1 p-0 font-medium'
            aria-expanded={expanded}
            aria-controls={options.getDetailPanelId(original.key)}
            onClick={() => options.onToggleRow(original.key)}
          >
            <ChevronRight
              className={cn(
                'size-3.5 shrink-0 transition-transform',
                expanded && 'rotate-90'
              )}
              aria-hidden='true'
            />
            <span className='truncate'>
              {original.username || EMPTY_DIMENSION}
            </span>
          </Button>
        )
      },
      size: options.grouped ? 200 : 160,
    })
  }

  if (!options.grouped) {
    columns.push(
      {
        accessorKey: 'use_group',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Group')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono text-xs'>
            {row.original.use_group || EMPTY_DIMENSION}
          </span>
        ),
        size: 140,
      },
      {
        accessorKey: 'model_name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Model')} />
        ),
        cell: ({ row }) => (
          <span className='block max-w-60 truncate font-mono text-xs'>
            {row.original.model_name || EMPTY_DIMENSION}
          </span>
        ),
        size: 220,
      }
    )
  }

  columns.push(
    {
      accessorKey: 'count',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('Requests')}
          align='end'
        />
      ),
      cell: ({ row }) => (
        <div className='text-right font-mono text-xs tabular-nums'>
          {formatNumber(row.original.count)}
        </div>
      ),
      size: 110,
    },
    {
      accessorKey: 'token_used',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('Tokens')}
          align='end'
        />
      ),
      cell: ({ row }) => (
        <div
          className='text-right font-mono text-xs tabular-nums'
          title={formatNumber(row.original.token_used)}
        >
          {formatCompactNumber(row.original.token_used)}
        </div>
      ),
      size: 120,
    },
    {
      accessorKey: 'quota',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Cost')} align='end' />
      ),
      cell: ({ row }) => (
        <div className='text-right font-mono text-xs font-semibold tabular-nums'>
          {formatQuota(row.original.quota)}
        </div>
      ),
      size: 130,
    },
    {
      id: 'cache-rate',
      header: ({ column }) => (
        <DataTableColumnHeader
          column={column}
          title={t('Cache Rate')}
          align='end'
        />
      ),
      cell: ({ row }) => {
        const rate = computeCacheRate([row.original])
        return (
          <div className='text-right font-mono text-xs tabular-nums'>
            {rate === null ? '--' : formatPercent(rate)}
          </div>
        )
      },
      size: 110,
    },
    {
      id: 'share',
      accessorFn: (row) => row.quota,
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Share')} align='end' />
      ),
      cell: ({ row }) => {
        const share = breakdownShare(row.original.quota, options.totalQuota)
        return (
          <div className='flex min-w-28 items-center gap-2'>
            <Progress
              value={Math.min(100, Math.max(0, share))}
              className='w-16 flex-1 flex-nowrap gap-2'
              aria-label={t('Share')}
            />
            <span className='text-muted-foreground w-14 shrink-0 text-right font-mono text-xs tabular-nums'>
              {formatPercent(share)}
            </span>
          </div>
        )
      },
      size: 150,
    }
  )

  return columns
}
