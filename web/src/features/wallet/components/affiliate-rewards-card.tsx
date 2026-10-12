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
import { Share2, TrendingUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatQuota } from '@/lib/format'

import type { ReferralSummary, UserWalletData } from '../types'

interface AffiliateRewardsCardProps {
  user: UserWalletData | null
  affiliateLink: string
  summary?: ReferralSummary | null
  onTransfer: () => void
  complianceConfirmed?: boolean
  loading?: boolean
}

// formatRate renders a basis-point rate as a percentage without losing the
// sub-percent precision the backend stores (150 bps -> 1.5%).
function formatRate(rateBps: number): string {
  if (!rateBps) {
    return '0%'
  }
  if (rateBps % 100 === 0) {
    return `${rateBps / 100}%`
  }
  return `${(rateBps / 100).toFixed(2).replace(/0+$/, '').replace(/\.$/, '')}%`
}

export function AffiliateRewardsCard({
  user,
  affiliateLink,
  summary,
  onTransfer,
  complianceConfirmed = true,
  loading,
}: AffiliateRewardsCardProps) {
  const { t } = useTranslation()
  if (loading) {
    return (
      <Card data-card-hover='false' className='bg-muted/20 py-0'>
        <CardContent className='grid gap-4 p-3 sm:p-4 lg:grid-cols-[minmax(220px,1fr)_minmax(220px,0.72fr)_minmax(320px,1.15fr)] lg:items-center'>
          <div>
            <Skeleton className='h-5 w-32' />
            <Skeleton className='mt-2 h-4 w-48' />
          </div>
          <Skeleton className='h-14 rounded-lg' />
          <Skeleton className='h-10 rounded-lg' />
        </CardContent>
      </Card>
    )
  }

  const claimable = summary?.claimable_quota ?? user?.aff_quota ?? 0
  const hasRewards = claimable > 0
  const programEnabled = summary?.enabled !== false
  const hasNextTier = (summary?.next_tier_invitees ?? 0) > 0

  return (
    <Card data-card-hover='false' className='bg-muted/20 py-0'>
      <CardContent className='grid gap-3 p-3 sm:gap-4 sm:p-4 lg:grid-cols-[minmax(200px,1fr)_minmax(180px,0.65fr)_minmax(280px,1fr)] lg:items-center'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <IconBadge tone='chart-3'>
            <Share2 />
          </IconBadge>
          <div className='min-w-0'>
            <h3 className='truncate text-sm font-semibold'>
              {t('Referral Program')}
            </h3>
            <p className='text-muted-foreground line-clamp-1 text-xs'>
              {programEnabled
                ? t(
                    'Earn commission when your invitees make their first top-up. Move referral rewards to your balance anytime.'
                  )
                : t('The referral program is currently disabled.')}
            </p>
          </div>
        </div>

        <div className='grid grid-cols-3 gap-1.5 text-center'>
          {[
            [t('Pending'), formatQuota(summary?.pending_quota ?? 0)],
            [
              t('Transferable'),
              formatQuota(summary?.claimable_quota ?? user?.aff_quota ?? 0),
            ],
            [
              t('Total Earned'),
              formatQuota(summary?.history_quota ?? user?.aff_history_quota ?? 0),
            ],
          ].map(([label, value]) => (
            <div key={label}>
              <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
                {label}
              </div>
              <div className='mt-0.5 truncate text-sm font-semibold tabular-nums'>
                {value}
              </div>
            </div>
          ))}
        </div>

        <div className='flex items-center gap-2'>
          <Input
            value={affiliateLink}
            readOnly
            className='border-muted bg-background/70 h-9 min-w-0 flex-1 font-mono text-xs'
          />
          <CopyButton
            value={affiliateLink}
            variant='outline'
            className='bg-background size-9 shrink-0'
            iconClassName='size-4'
            tooltip={t('Copy referral link')}
            aria-label={t('Copy referral link')}
          />
          {hasRewards && (
            <Button
              onClick={onTransfer}
              disabled={!complianceConfirmed}
              className='h-9 shrink-0 px-3'
              size='sm'
            >
              {t('Transfer to Balance')}
            </Button>
          )}
          <TooltipProvider delay={0}>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant='outline'
                    className='h-9 shrink-0 px-3'
                    size='sm'
                    disabled
                  />
                }
              >
                {t('Withdraw')}
                <Badge variant='secondary' className='ml-1.5 px-1.5 py-0 text-[10px]'>
                  {t('Coming soon')}
                </Badge>
              </TooltipTrigger>
              <TooltipContent>
                {t('Withdrawal is under development and is not available yet.')}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        {programEnabled && summary?.inviter_reward_enabled ? (
          <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs lg:col-span-3'>
            <span className='inline-flex items-center gap-1 font-medium'>
              <TrendingUp className='size-3.5' />
              {t('Current rate')}: {formatRate(summary.current_rate_bps)}
            </span>
            <span>
              {t('Valid invitees')}: {summary.valid_invitees}
              {' · '}
              {t('Registered')}: {summary.registered_count}
            </span>
            {hasNextTier ? (
              <span>
                {t('{{count}} more valid invitees to reach {{rate}}', {
                  count: Math.max(
                    summary.next_tier_invitees - summary.valid_invitees,
                    0
                  ),
                  rate: formatRate(summary.next_tier_rate_bps),
                })}
              </span>
            ) : null}
            {summary.invitee_reward_enabled ? (
              <span>
                {t(
                  'Invitees receive {{rate}} of their first top-up as bonus credit',
                  { rate: formatRate(summary.invitee_rate_bps) }
                )}
              </span>
            ) : null}
          </div>
        ) : null}

        {!complianceConfirmed || summary?.compliance_confirmed === false ? (
          <p className='text-muted-foreground text-xs lg:col-span-3'>
            {t(
              'Referral reward transfer is disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
