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

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { formatQuota } from '@/lib/format'

import {
  describeActivityDuration,
  describeActivityModelScope,
  describeActivityRatio,
} from '../lib/activity-format'
import type { ActivityOffer } from '../types'

type ActivityOfferCardsProps = {
  offers: ActivityOffer[]
  disabled?: boolean
  onRedeem: (offer: ActivityOffer) => void
}

// 活动套餐以卡片网格展示：窄屏单列、≥sm 双列，卡片等高，长文案截断。
export function ActivityOfferCards(props: ActivityOfferCardsProps) {
  const { t } = useTranslation()
  if (props.offers.length === 0) return null

  return (
    <div
      data-slot='activity-offer-cards'
      className='grid grid-cols-1 gap-3 sm:grid-cols-2'
    >
      {props.offers.map((offer) => {
        const modelScope = describeActivityModelScope(
          t,
          offer.model_ratio_overrides
        )
        const details = [
          {
            label: t('Group'),
            value: offer.group_desc || offer.group_name,
          },
          {
            label: t('Validity'),
            value: describeActivityDuration(t, offer.duration_seconds),
          },
          {
            label: t('Ratio'),
            value: describeActivityRatio(
              t,
              offer.ratio_mode,
              offer.ratio_override
            ),
          },
          {
            label: t('RPM limit'),
            value:
              offer.rpm_limit > 0 ? String(offer.rpm_limit) : t('Unlimited'),
          },
        ]

        return (
          <Card key={offer.code_id} size='sm' className='h-full'>
            <CardHeader>
              <CardTitle className='truncate'>
                {offer.name || offer.code}
              </CardTitle>
              <CardDescription className='line-clamp-2'>
                {offer.description ||
                  t('Limited-time group access granted by an activity code')}
              </CardDescription>
            </CardHeader>
            <CardContent className='grid grid-cols-2 gap-x-3 gap-y-2'>
              {details.map((detail) => (
                <div key={detail.label} className='min-w-0'>
                  <p className='text-muted-foreground text-xs'>{detail.label}</p>
                  <p className='truncate text-xs font-medium'>{detail.value}</p>
                </div>
              ))}
              {/* 指定模型时直接列出具体模型 ID 与倍率：整行展示且允许换行，
                  不用 truncate 截断模型名。 */}
              <div className='col-span-2 min-w-0'>
                <p className='text-muted-foreground text-xs'>
                  {t('Models (ratio)')}
                </p>
                <p className='text-xs font-medium break-all' title={modelScope}>
                  {modelScope}
                </p>
              </div>
            </CardContent>
            {/* 价格紧贴兑换按钮：放在卡片右上角太容易被忽略，付费码需要在下单前看到金额。 */}
            <CardFooter className='mt-auto justify-between gap-2'>
              <span className='text-muted-foreground text-xs'>
                {t('Remaining')}:{' '}
                {offer.remaining_total >= 0
                  ? offer.remaining_total
                  : t('Unlimited')}
              </span>
              <div className='flex shrink-0 items-center gap-2'>
                <Badge
                  variant={offer.price_quota > 0 ? 'warning' : 'default'}
                  className='h-6 px-2.5 text-sm font-semibold'
                >
                  {offer.price_quota > 0
                    ? formatQuota(offer.price_quota)
                    : t('Free')}
                </Badge>
                <Button
                  size='sm'
                  disabled={props.disabled}
                  aria-label={t('Redeem {{name}}', {
                    name: offer.name || offer.code,
                  })}
                  onClick={() => props.onRedeem(offer)}
                >
                  {t('Redeem')}
                </Button>
              </div>
            </CardFooter>
          </Card>
        )
      })}
    </div>
  )
}
