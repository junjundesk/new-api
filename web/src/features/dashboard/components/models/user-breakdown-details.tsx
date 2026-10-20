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
import { useTranslation } from 'react-i18next'

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { UserQuotaBreakdownRow } from '@/features/dashboard/types'
import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'

const EMPTY_DIMENSION = '—'

interface UserBreakdownDetailsProps {
  details: UserQuotaBreakdownRow[]
}

// Inline disclosure content for one admin user row: the original group x model
// contributions that were collapsed into the user's total.
export function UserBreakdownDetails(props: UserBreakdownDetailsProps) {
  const { t } = useTranslation()
  if (props.details.length === 0) {
    return null
  }

  return (
    <div className='bg-muted/20 px-3 py-2'>
      <Table>
        <TableHeader>
          <TableRow className='hover:bg-transparent'>
            <TableHead className='h-8'>{t('Group')}</TableHead>
            <TableHead className='h-8'>{t('Model')}</TableHead>
            <TableHead className='h-8 text-right'>{t('Requests')}</TableHead>
            <TableHead className='h-8 text-right'>{t('Tokens')}</TableHead>
            <TableHead className='h-8 text-right'>{t('Cost')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.details.map((detail) => (
            <TableRow
              key={`${detail.use_group}\u0000${detail.model_name}`}
              className='hover:bg-muted/40'
            >
              <TableCell className='font-mono text-xs'>
                {detail.use_group || EMPTY_DIMENSION}
              </TableCell>
              <TableCell className='font-mono text-xs'>
                {detail.model_name || EMPTY_DIMENSION}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatNumber(detail.count)}
              </TableCell>
              <TableCell
                className='text-right font-mono text-xs tabular-nums'
                title={formatNumber(detail.token_used)}
              >
                {formatCompactNumber(detail.token_used)}
              </TableCell>
              <TableCell className='text-right font-mono text-xs font-semibold tabular-nums'>
                {formatQuota(detail.quota)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
