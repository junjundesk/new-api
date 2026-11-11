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

import { Card, CardContent, CardHeader } from '@/components/ui/card'
import type { ReferralSummary } from '@/features/wallet/types'
import { cn } from '@/lib/utils'

import { currentTierIndex, formatRate } from '../lib/format'

interface TierCardProps {
  summary: ReferralSummary
}

/**
 * Commission ladder with the tier the inviter currently sits on highlighted
 * and how many more valid invitees the next tier needs.
 */
export function TierCard(props: TierCardProps) {
  const { t } = useTranslation()
  const summary = props.summary
  const tiers = summary.tiers ?? []
  if (tiers.length === 0) {
    return null
  }
  const activeIndex = currentTierIndex(tiers, summary.valid_invitees)
  const remaining = Math.max(
    summary.next_tier_invitees - summary.valid_invitees,
    0
  )

  return (
    <Card data-card-hover='false'>
      <CardHeader>
        <p className='text-sm font-medium'>{t('Commission tiers')}</p>
      </CardHeader>
      <CardContent className='space-y-3'>
        <ol className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
          {tiers.map((tier, index) => {
            const active = index === activeIndex
            return (
              <li
                key={tier.min_invitees}
                aria-current={active ? 'step' : undefined}
                className={cn(
                  'rounded-lg border p-2.5 text-center',
                  active && 'border-foreground bg-muted'
                )}
              >
                <div className='text-lg font-semibold tabular-nums'>
                  {formatRate(tier.rate_bps)}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('{{count}} valid invitees', { count: tier.min_invitees })}
                </div>
                {active ? (
                  <div className='mt-0.5 text-xs font-medium'>
                    {t('Current tier')}
                  </div>
                ) : null}
              </li>
            )
          })}
        </ol>
        {summary.next_tier_invitees > 0 ? (
          <p className='text-muted-foreground text-xs'>
            {t('{{count}} more valid invitees to reach {{rate}}', {
              count: remaining,
              rate: formatRate(summary.next_tier_rate_bps),
            })}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
