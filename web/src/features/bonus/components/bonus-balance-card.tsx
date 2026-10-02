import { Gift } from 'lucide-react'
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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

import { getSelfBonus } from '../api'

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

interface ExpiryGroup {
  /** Remaining quota across grants sharing this expiry. */
  remaining: number
  /** Expiry timestamp in seconds; 0 = never expires. */
  expireTime: number
}

/**
 * Compact bonus balance display.
 *
 * Expiring and never-expiring bonus are shown as separate lines, because a
 * single total with one expiry date misrepresents a user who holds both: the
 * permanent part would appear to vanish at the expiring part's deadline.
 * When the user holds several grants with different expiries, each expiry
 * bucket is listed individually (e.g. "$1 · expires Oct 3, $2 · expires Oct 5")
 * so users can see how much disappears when. Renders nothing when the user has
 * no bonus, so it is safe to mount unconditionally on the wallet and profile
 * pages.
 */
export function BonusBalanceCard(props: BonusBalanceCardProps) {
  const { t } = useTranslation()
  const [expiryGroups, setExpiryGroups] = useState<ExpiryGroup[] | null>(null)

  const total = props.bonusQuota ?? 0
  const hasBonus = total > 0

  useEffect(() => {
    if (!hasBonus) {
      return
    }
    let cancelled = false
    getSelfBonus()
      .then((res) => {
        if (cancelled || !res?.success || !res.data?.grants) {
          return
        }
        const byExpiry = new Map<number, number>()
        for (const grant of res.data.grants) {
          const remaining = grant.amount_total - grant.amount_used
          if (remaining <= 0) {
            continue
          }
          byExpiry.set(
            grant.expire_time,
            (byExpiry.get(grant.expire_time) ?? 0) + remaining
          )
        }
        // Soonest expiry first, never-expiring last.
        const groups = [...byExpiry.entries()]
          .sort(
            (a, b) => (a[0] === 0 ? 1 : 0) - (b[0] === 0 ? 1 : 0) || a[0] - b[0]
          )
          .map(([expireTime, remaining]) => ({ expireTime, remaining }))
        setExpiryGroups(groups)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [hasBonus])

  if (!hasBonus) {
    return null
  }

  // Callers that only pass a total (legacy) still render: treat everything as
  // expiring so the deadline stays visible rather than silently disappearing.
  const expiring = props.expiringQuota ?? total
  const permanent = props.permanentQuota ?? 0
  const showPerGrantBreakdown = (expiryGroups?.length ?? 0) > 1
  const showSummaryBreakdown =
    !showPerGrantBreakdown && expiring > 0 && permanent > 0

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
        {showPerGrantBreakdown && (
          <div className='text-muted-foreground mt-0.5 flex flex-col gap-0.5 text-xs'>
            {expiryGroups?.map((group) =>
              group.expireTime > 0 ? (
                <span key={group.expireTime}>
                  {formatQuota(group.remaining)}
                  {' · '}
                  {t('Expires {{time}}', {
                    time: formatTimestampToDate(group.expireTime),
                  })}
                </span>
              ) : (
                <span key='permanent'>
                  {t('Permanent: {{quota}}', {
                    quota: formatQuota(group.remaining),
                  })}
                </span>
              )
            )}
          </div>
        )}
        {showSummaryBreakdown && (
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
        {!showPerGrantBreakdown && expiring > 0 && props.expireTime > 0 && (
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
