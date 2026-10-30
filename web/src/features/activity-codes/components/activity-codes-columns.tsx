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
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Checkbox } from '@/components/ui/checkbox'
import {
  formatModelRatioOverrides,
  formatQuota,
  formatRatioMultiplier,
  formatTimestampToDate,
} from '@/lib/format'

import {
  ACTIVITY_CODE_STATUS,
  ACTIVITY_CODE_TYPE,
  ACTIVITY_RATIO_MODE,
} from '../constants'
import { describeActivityBenefit } from '../lib/activity-benefit'
import type { ActivityCode } from '../types'
import { DataTableRowActions } from './activity-codes-row-actions'

function formatDuration(t: TFunction, seconds: number): string {
  if (seconds <= 0) return t('Permanent')
  const hours = seconds / 3600
  if (hours < 1) return t('{{count}} minutes', { count: seconds / 60 })
  if (hours < 24) return t('{{count}} hours', { count: hours })
  return t('{{count}} days', { count: hours / 24 })
}

export function useActivityCodesColumns(): ColumnDef<ActivityCode>[] {
  const { t, i18n } = useTranslation()
  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('Select all')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('Select row')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },
    {
      accessorKey: 'code',
      header: t('Activity Code'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm font-medium'>
          {row.original.code}
        </span>
      ),
      size: 180,
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      cell: ({ row }) => <span className='font-medium'>{row.original.name}</span>,
      size: 160,
    },
    {
      accessorKey: 'group_name',
      header: t('Group'),
      cell: ({ row }) => {
        const benefits = row.original.benefits ?? []
        const isBenefit = row.original.type === ACTIVITY_CODE_TYPE.BENEFIT
        if (!isBenefit || benefits.length === 0) {
          return (
            <StatusBadge
              label={row.original.group_name}
              variant='info'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        // 权益类型可能覆盖多个分组：展示首分组，其余用 +N 折叠，title 给出全部。
        const groups = [
          ...new Set(benefits.map((benefit) => benefit.group_name)),
        ]
        return (
          <span
            className='inline-flex items-center gap-1'
            title={groups.join(', ')}
          >
            <StatusBadge
              label={groups[0]}
              variant='info'
              copyable={false}
              className='-ml-1.5'
            />
            {groups.length > 1 && (
              <span className='text-muted-foreground text-xs'>
                +{groups.length - 1}
              </span>
            )}
          </span>
        )
      },
      size: 140,
    },
    {
      accessorKey: 'duration_seconds',
      header: t('Validity'),
      cell: ({ row }) => (
        <span>{formatDuration(t, row.original.duration_seconds)}</span>
      ),
      size: 110,
    },
    {
      accessorKey: 'price_quota',
      header: t('Price'),
      cell: ({ row }) =>
        row.original.price_quota > 0 ? (
          <span>{formatQuota(row.original.price_quota)}</span>
        ) : (
          <StatusBadge
            label={t('Free of charge')}
            variant='success'
            copyable={false}
            className='-ml-1.5'
          />
        ),
      size: 110,
    },
    {
      id: 'ratio',
      header: t('Ratio'),
      cell: ({ row }) => {
        // 权益类型展示每条权益的「分组 · 范围 · Token 总量」摘要。
        if (row.original.type === ACTIVITY_CODE_TYPE.BENEFIT) {
          const benefits = row.original.benefits ?? []
          const summary = benefits
            .map((benefit) => describeActivityBenefit(t, benefit, i18n.language))
            .join('; ')
          if (!summary) {
            return <span className='text-muted-foreground'>-</span>
          }
          return (
            <span
              className='block max-w-[18rem] truncate text-xs'
              title={summary}
            >
              {summary}
            </span>
          )
        }
        // 「指定模型」类型直接展示模型 ID 与倍率（如 gpt-5: 0.1x、o3: 免费）；
        // 超出列宽时截断，完整内容放 title。
        const listedOverrides = formatModelRatioOverrides(
          t,
          row.original.model_ratio_overrides
        )
        if (listedOverrides) {
          return (
            <span
              className='block max-w-[16rem] truncate font-mono text-xs'
              title={listedOverrides}
            >
              {listedOverrides}
            </span>
          )
        }
        if (row.original.ratio_mode !== ACTIVITY_RATIO_MODE.OVERRIDE) {
          return <span className='text-muted-foreground'>{t('Inherited')}</span>
        }
        if (row.original.ratio_override === 0) {
          return (
            <StatusBadge
              label={t('Fully Free')}
              variant='success'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        return <span>{formatRatioMultiplier(row.original.ratio_override)}x</span>
      },
      size: 200,
    },
    {
      id: 'rpm_limit',
      header: t('RPM'),
      cell: ({ row }) =>
        row.original.rpm_limit > 0 ? (
          <span>{row.original.rpm_limit}</span>
        ) : (
          <span className='text-muted-foreground'>{t('Unlimited')}</span>
        ),
      size: 90,
    },
    {
      id: 'limits',
      header: t('Limits'),
      cell: ({ row }) => {
        const totalText =
          row.original.total_limit > 0
            ? `${row.original.used_count}/${row.original.total_limit}`
            : `${row.original.used_count}/${t('Unlimited')}`
        const perUserText =
          row.original.per_user_limit > 0
            ? String(row.original.per_user_limit)
            : t('Unlimited')
        return (
          <span className='text-muted-foreground text-xs'>
            {t('Total')}: {totalText} · {t('Per user')}: {perUserText}
          </span>
        )
      },
      size: 170,
    },
    {
      id: 'window',
      header: t('Redemption Window'),
      cell: ({ row }) => {
        const start = row.original.start_time
        const end = row.original.end_time
        let text = t('Always')
        if (start > 0 && end > 0) {
          text = `${formatTimestampToDate(start)} ~ ${formatTimestampToDate(end)}`
        } else if (end > 0) {
          text = `${t('Until')} ${formatTimestampToDate(end)}`
        } else if (start > 0) {
          text = `${t('From')} ${formatTimestampToDate(start)}`
        }
        return <span className='text-muted-foreground text-xs'>{text}</span>
      },
      size: 200,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => (
        <StatusBadge
          label={
            row.original.status === ACTIVITY_CODE_STATUS.ENABLED
              ? t('Enabled')
              : t('Disabled')
          }
          variant={
            row.original.status === ACTIVITY_CODE_STATUS.ENABLED
              ? 'success'
              : 'neutral'
          }
          copyable={false}
          className='-ml-1.5'
        />
      ),
      size: 110,
    },
    {
      id: 'actions',
      cell: ({ row }) => <DataTableRowActions row={row} />,
      enableSorting: false,
      enableHiding: false,
      size: 120,
    },
  ]
}
