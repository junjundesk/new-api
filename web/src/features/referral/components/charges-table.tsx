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

import { StatusBadge } from '@/components/status-badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { ReferralCharge } from '@/features/wallet/types'
import { formatQuota, formatTimestamp } from '@/lib/format'

import { formatRate } from '../lib/format'

const KIND_INVITER = 1
const STATUS_PENDING = 0
const STATUS_SETTLED = 1

interface ChargesTableProps {
  charges: ReferralCharge[]
  loading?: boolean
}

/**
 * The caller's referral reward history: commission earned from invitees and
 * the caller's own first-top-up reward.
 */
export function ChargesTable(props: ChargesTableProps) {
  const { t } = useTranslation()

  if (props.loading) {
    return (
      <div className='space-y-2'>
        <Skeleton className='h-8 w-full' />
        <Skeleton className='h-8 w-full' />
        <Skeleton className='h-8 w-full' />
      </div>
    )
  }
  if (props.charges.length === 0) {
    return (
      <p className='text-muted-foreground py-8 text-center text-sm'>
        {t('No reward records yet')}
      </p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Time')}</TableHead>
          <TableHead>{t('Type')}</TableHead>
          <TableHead className='text-right'>{t('Top-up amount')}</TableHead>
          <TableHead className='text-right'>{t('Reward rate')}</TableHead>
          <TableHead className='text-right'>{t('Reward')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.charges.map((charge) => {
          let statusLabel = t('Cancelled')
          let statusVariant: 'warning' | 'success' | 'neutral' = 'neutral'
          if (charge.status === STATUS_PENDING) {
            statusLabel = t('Pending')
            statusVariant = 'warning'
          } else if (charge.status === STATUS_SETTLED) {
            statusLabel = t('Credited')
            statusVariant = 'success'
          }

          return (
            <TableRow key={charge.id}>
              <TableCell className='text-muted-foreground tabular-nums'>
                {formatTimestamp(charge.create_time)}
              </TableCell>
              <TableCell>
                {charge.kind === KIND_INVITER
                  ? t('Inviter commission')
                  : t('Invitee first top-up reward')}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {formatQuota(charge.base_quota)}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {formatRate(charge.rate_bps)}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {formatQuota(charge.amount)}
              </TableCell>
              <TableCell>
                <StatusBadge
                  label={statusLabel}
                  variant={statusVariant}
                  copyable={false}
                />
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
