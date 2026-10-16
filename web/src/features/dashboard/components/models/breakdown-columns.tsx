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
import { useTranslation } from 'react-i18next'

import { DataTableColumnHeader } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { breakdownShare } from '@/features/dashboard/lib'
import type { UserQuotaBreakdownRow } from '@/features/dashboard/types'
import {
  formatCompactNumber,
  formatNumber,
  formatPercent,
  formatQuota,
} from '@/lib/format'

const EMPTY_DIMENSION = '—'

interface BreakdownColumnOptions {
  // Non-admins only ever see their own rows, so the user column is redundant.
  showUsername: boolean
  // Denominator of the share column: the cost of the current filter selection.
  totalQuota: number
  onSelectUsername: (username: string) => void
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
        const username = row.original.username
        if (!username) {
          return (
            <span className='text-muted-foreground'>{EMPTY_DIMENSION}</span>
          )
        }
        return (
          <Button
            type='button'
            variant='link'
            size='sm'
            className='h-auto max-w-40 justify-start truncate p-0 font-medium'
            onClick={() => options.onSelectUsername(username)}
          >
            {username}
          </Button>
        )
      },
      size: 160,
    })
  }

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
    },
    {
      accessorKey: 'count',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Requests')} />
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
        <DataTableColumnHeader column={column} title={t('Tokens')} />
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
        <DataTableColumnHeader column={column} title={t('Cost')} />
      ),
      cell: ({ row }) => (
        <div className='text-right font-mono text-xs font-semibold tabular-nums'>
          {formatQuota(row.original.quota)}
        </div>
      ),
      size: 130,
    },
    {
      id: 'share',
      accessorFn: (row) => row.quota,
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Share')} />
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
