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
import { formatQuota, formatTimestamp } from '@/lib/format'

import type { ReferralInvitee } from '../types'

interface InviteesTableProps {
  invitees: ReferralInvitee[]
  loading?: boolean
  /** Admin views show the numeric user ID next to the username */
  showId?: boolean
}

/**
 * Users invited by one inviter, with whether each counted as a valid invite
 * and how much commission it generated.
 */
export function InviteesTable(props: InviteesTableProps) {
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
  if (props.invitees.length === 0) {
    return (
      <p className='text-muted-foreground py-8 text-center text-sm'>
        {t('No invited users yet')}
      </p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('User')}</TableHead>
          <TableHead>{t('Registered at')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead className='text-right'>{t('Reward generated')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.invitees.map((invitee) => (
          <TableRow key={invitee.id}>
            <TableCell>
              {props.showId ? (
                <span className='text-muted-foreground mr-1.5 font-mono text-xs'>
                  #{invitee.id}
                </span>
              ) : null}
              {invitee.username}
            </TableCell>
            <TableCell className='text-muted-foreground tabular-nums'>
              {invitee.created_at > 0
                ? formatTimestamp(invitee.created_at)
                : '-'}
            </TableCell>
            <TableCell>
              <StatusBadge
                label={invitee.valid ? t('Valid invite') : t('Registered only')}
                variant={invitee.valid ? 'success' : 'neutral'}
                copyable={false}
              />
            </TableCell>
            <TableCell className='text-right tabular-nums'>
              {invitee.reward_quota > 0
                ? formatQuota(invitee.reward_quota)
                : '-'}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
