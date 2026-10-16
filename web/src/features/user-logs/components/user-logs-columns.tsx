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

import {
  StatusBadge,
  type StatusBadgeProps,
} from '@/components/status-badge'
import type { UsageLog } from '@/features/usage-logs/data/schema'
import {
  parseLogOther,
  renderAuditContent,
} from '@/features/usage-logs/lib/format'
import { getLogTypeConfig } from '@/features/usage-logs/lib/utils'
import { formatTimestampToDate } from '@/lib/format'

/**
 * 用户日志列：时间 / 类型 / 用户（仅管理员）/ 内容 / IP。
 * 内容优先用 other.op 的语言无关描述本地化渲染，老日志回退到落库文本。
 */
export function useUserLogsColumns(isAdmin: boolean): ColumnDef<UsageLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<UsageLog>[] = [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => (
        <span className='font-mono text-xs tabular-nums'>
          {formatTimestampToDate(row.original.created_at)}
        </span>
      ),
      enableHiding: false,
      size: 180,
    },
    {
      accessorKey: 'type',
      header: t('Type'),
      cell: ({ row }) => {
        const config = getLogTypeConfig(row.original.type)
        return (
          <StatusBadge
            label={t(config.label)}
            variant={config.color as StatusBadgeProps['variant']}
            size='sm'
            copyable={false}
          />
        )
      },
      enableHiding: false,
      size: 110,
    },
  ]

  if (isAdmin) {
    columns.push({
      accessorKey: 'username',
      header: t('User'),
      cell: ({ row }) => (
        <span className='text-sm'>{row.original.username || '-'}</span>
      ),
      size: 140,
    })
  }

  columns.push(
    {
      id: 'content',
      header: t('Content'),
      cell: ({ row }) => {
        const log = row.original
        const localized = renderAuditContent(parseLogOther(log.other), t)
        return (
          <span className='text-sm break-words whitespace-pre-wrap'>
            {localized ?? log.content}
          </span>
        )
      },
      enableHiding: false,
      size: 420,
    },
    {
      accessorKey: 'ip',
      header: t('IP Address'),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs'>
          {row.original.ip || '-'}
        </span>
      ),
      size: 140,
    }
  )

  return columns
}
