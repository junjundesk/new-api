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
import { Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

interface BonusBalanceCardProps {
  /** Total remaining bonus quota (expiring + permanent). */
  bonusQuota: number
  /** Remaining bonus that expires; 0 when the user holds none. */
  expiringQuota?: number
  /** Remaining bonus that never expires. */
  permanentQuota?: number
  /** Nearest expiry timestamp in seconds (0 = no expiring bonus). */
  expireTime: number
  className?: string
}

/**
 * Compact bonus balance display.
 *
 * Expiring and never-expiring bonus are shown as separate lines, because a
 * single total with one expiry date misrepresents a user who holds both: the
 * permanent part would appear to vanish at the expiring part's deadline.
 * Renders nothing when the user has no bonus, so it is safe to mount
 * unconditionally on the wallet and profile pages.
 */
export function BonusBalanceCard(props: BonusBalanceCardProps) {
  const { t } = useTranslation()

  const total = props.bonusQuota ?? 0
  if (total <= 0) {
    return null
  }

  // Callers that only pass a total (legacy) still render: treat everything as
  // expiring so the deadline stays visible rather than silently disappearing.
  const expiring = props.expiringQuota ?? total
  const permanent = props.permanentQuota ?? 0
  const showBreakdown = expiring > 0 && permanent > 0

  return (
    <div
      className={`flex items-center gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2.5 ${props.className ?? ''}`}
    >
      <IconBadge tone='warning' size='stat'>
        <Gift />
      </IconBadge>
      <div className='min-w-0 flex-1'>
        <div className='text-muted-foreground text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
          {t('Bonus Quota')}
        </div>
        <div className='text-foreground font-mono text-sm font-bold tabular-nums sm:text-lg'>
          {formatQuota(total)}
        </div>
        {showBreakdown && (
          <div className='text-muted-foreground mt-0.5 flex flex-wrap gap-x-3 text-xs'>
            {expiring > 0 && (
              <span>
                {t('Expiring: {{quota}}', {
                  quota: formatQuota(expiring),
                })}
              </span>
            )}
            {permanent > 0 && (
              <span>
                {t('Permanent: {{quota}}', {
                  quota: formatQuota(permanent),
                })}
              </span>
            )}
          </div>
        )}
      </div>
      <div className='shrink-0 text-right text-xs font-medium'>
        {expiring > 0 && props.expireTime > 0 && (
          <div className='text-red-600 dark:text-red-400'>
            {t('Expires {{time}}', {
              time: formatTimestampToDate(props.expireTime),
            })}
          </div>
        )}
      </div>
    </div>
  )
}
