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
import {
  CheckCircle2,
  Globe,
  Infinity as InfinityIcon,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { ENTERPRISE_PRESET } from '@/features/enterprise/lib'
import { formatLocalCurrencyAmount } from '@/lib/currency'

interface EnterpriseBenefitsCardProps {
  /** Cumulative successfully paid recharge amount (same unit as Price) */
  totalRecharge?: number
  /** Threshold that unlocks the enterprise identity */
  threshold?: number
  /** Whether the account has already unlocked the enterprise identity */
  unlocked: boolean
}

/**
 * Enterprise identity card shown on the profile page.
 *
 * Unlocked accounts see the gold "Enterprise" badge and their benefits;
 * locked accounts see a progress hint toward the cumulative-recharge threshold
 * so the unlock rule is discoverable rather than hidden.
 */
export function EnterpriseBenefitsCard({
  totalRecharge = 0,
  threshold = 0,
  unlocked,
}: EnterpriseBenefitsCardProps) {
  const { t } = useTranslation()
  const { customization, setPreset, defaults } = useThemeCustomization()

  const remaining = Math.max(threshold - totalRecharge, 0)
  const progress =
    threshold > 0
      ? Math.min(Math.max((totalRecharge / threshold) * 100, 0), 100)
      : 100

  const benefits = [
    {
      icon: CheckCircle2,
      title: t('Verified enterprise account'),
      description: t('This account has the enterprise tier enabled.'),
    },
    {
      icon: InfinityIcon,
      title: t('Unlimited RPM'),
      description: t('Not subject to user-level request rate limits.'),
    },
    {
      icon: Globe,
      title: t('Regional access allowlist'),
      description: t(
        'Allowlisted IPs can open the web console in restricted regions.'
      ),
    },
    {
      icon: Users,
      title: t('Enterprise user group'),
      description: t('Join the dedicated enterprise user community.'),
    },
  ]

  const goldActive = customization.preset === ENTERPRISE_PRESET

  return (
    <Card
      data-card-hover='false'
      className='border-primary/30 gap-0 overflow-hidden'
    >
      <CardHeader className='from-primary/10 via-primary/5 border-b bg-gradient-to-r to-transparent'>
        <div className='flex items-center justify-between gap-3'>
          <div className='flex items-center gap-2'>
            <span className='text-primary text-base font-semibold'>
              {t('Enterprise benefits')}
            </span>
            <span className='border-primary/50 text-primary rounded-full border px-2 py-0.5 text-[10px] font-semibold tracking-wider'>
              ENTERPRISE
            </span>
          </div>
          {unlocked && (
            <Button
              size='sm'
              variant={goldActive ? 'secondary' : 'default'}
              onClick={() =>
                setPreset(goldActive ? defaults.preset : ENTERPRISE_PRESET)
              }
            >
              {goldActive
                ? t('Default theme')
                : t('Enterprise black-gold theme')}
            </Button>
          )}
        </div>
        {unlocked ? (
          <p className='text-muted-foreground text-sm'>
            {t('Enterprise account exclusive benefits')}
          </p>
        ) : (
          <div className='space-y-2'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Reach a cumulative recharge of {{amount}} to unlock the enterprise identity',
                { amount: formatLocalCurrencyAmount(threshold) }
              )}
            </p>
            <div className='bg-muted h-1.5 w-full overflow-hidden rounded-full'>
              <div
                className='bg-primary h-full rounded-full transition-all'
                style={{ width: `${progress}%` }}
              />
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('{{remaining}} remaining', {
                remaining: formatLocalCurrencyAmount(remaining),
              })}
            </p>
          </div>
        )}
      </CardHeader>

      {unlocked && (
        <CardContent className='grid gap-4 pt-4 sm:grid-cols-2'>
          {benefits.map((benefit) => (
            <div key={benefit.title} className='flex items-start gap-3'>
              <IconBadge tone='primary' size='md'>
                <benefit.icon />
              </IconBadge>
              <div className='min-w-0'>
                <p className='text-sm font-medium'>{benefit.title}</p>
                <p className='text-muted-foreground text-xs'>
                  {benefit.description}
                </p>
              </div>
            </div>
          ))}
        </CardContent>
      )}
    </Card>
  )
}
