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
import { cn } from '@/lib/utils'

import {
  describeActivityBenefit,
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
  const { t, i18n } = useTranslation()
  if (props.offers.length === 0) return null

  return (
    <div
      data-slot='activity-offer-cards'
      className='grid grid-cols-1 gap-3 sm:grid-cols-2'
    >
      {props.offers.map((offer) => {
        const benefits = offer.benefits ?? []
        const imageUrl = (offer.image_url ?? '').trim()
        const hasImage = imageUrl !== ''
        // 售罄/到期后后端仍返回该活动，只是把 redeemable 置为 false；
        // 旧接口没有该字段（undefined）时保持原有可兑换行为。
        const closed = offer.redeemable === false
        // 旧接口没有 type/benefits：按 ratio 卡片展示，保持原行为。
        const isBenefit = offer.type === 'benefit' && benefits.length > 0
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
          <Card
            key={offer.code_id}
            size='sm'
            className={cn(
              'relative isolate h-full',
              hasImage ? 'pt-24' : closed && 'pt-8'
            )}
          >
            <CardHeader>
              <CardTitle className='truncate'>
                {offer.name || offer.code}
              </CardTitle>
              <CardDescription className='line-clamp-2'>
                {offer.description ||
                  t('Limited-time group access granted by an activity code')}
              </CardDescription>
            </CardHeader>
            {isBenefit ? (
              <CardContent className='space-y-2'>
                <div className='min-w-0'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Validity')}
                  </p>
                  <p className='text-xs font-medium'>
                    {describeActivityDuration(t, offer.duration_seconds)}
                  </p>
                </div>
                {/* 一个活动码可以有多条权益，逐条列出分组、范围与 Token 总量。 */}
                <div className='min-w-0'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Benefits')}
                  </p>
                  <ul className='space-y-1'>
                    {benefits.map((benefit) => (
                      <li
                        key={`${benefit.key}:${benefit.group_name}:${benefit.model_name}`}
                        className='text-xs font-medium break-all'
                      >
                        {describeActivityBenefit(t, benefit, i18n.language)}
                      </li>
                    ))}
                  </ul>
                </div>
              </CardContent>
            ) : (
              <CardContent className='grid grid-cols-2 gap-x-3 gap-y-2'>
                {details.map((detail) => (
                  <div key={detail.label} className='min-w-0'>
                    <p className='text-muted-foreground text-xs'>
                      {detail.label}
                    </p>
                    <p className='truncate text-xs font-medium'>
                      {detail.value}
                    </p>
                  </div>
                ))}
                {/* 指定模型时直接列出具体模型 ID 与倍率：整行展示且允许换行，
                    不用 truncate 截断模型名。 */}
                <div className='col-span-2 min-w-0'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Models (ratio)')}
                  </p>
                  <p
                    className='text-xs font-medium break-all'
                    title={modelScope}
                  >
                    {modelScope}
                  </p>
                </div>
              </CardContent>
            )}
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
                    : t('Free of charge')}
                </Badge>
                <Button
                  size='sm'
                  disabled={props.disabled || closed}
                  aria-label={t('Redeem {{name}}', {
                    name: offer.name || offer.code,
                  })}
                  onClick={() => props.onRedeem(offer)}
                >
                  {t('Redeem')}
                </Button>
              </div>
            </CardFooter>
            {/* 背景图与角标放在最后：Card 的 \`*:[img:first-child]\` 等样式只认首个子节点，
                绝对定位元素不参与布局，顺序不影响视觉层级（负 z-index 绘制在内容之下）。 */}
            {hasImage && (
              // 封面带：图片按 object-cover 铺满卡片顶部 96px，再向下渐变到卡片底色。
              // 图片在顶部完整可见（不压文字），正文从 96px 处开始，对比度等同无背景图。
              <div
                data-slot='activity-offer-image-band'
                className='pointer-events-none absolute inset-x-0 top-0 -z-10 h-24'
              >
                <img
                  data-slot='activity-offer-image'
                  src={imageUrl}
                  alt=''
                  aria-hidden='true'
                  loading='lazy'
                  className='absolute inset-0 h-full w-full object-cover'
                />
                <div
                  data-slot='activity-offer-image-scrim'
                  aria-hidden='true'
                  className='to-card absolute inset-0 bg-linear-to-b from-transparent from-40%'
                />
              </div>
            )}
            {closed && (
              // 斜角角标：45° 长条被左上角方框裁掉两侧，只留贴角的斜带；
              // 文案不设 aria-hidden，读屏会把它作为卡片内容读出来。
              <div className='pointer-events-none absolute top-0 left-0 z-10 h-16 w-16 overflow-hidden'>
                <span
                  data-slot='activity-offer-closed-badge'
                  className='bg-foreground text-background absolute top-[7px] -left-[35px] w-[100px] -rotate-45 text-center text-[10px] leading-4 font-semibold tracking-wide'
                >
                  {t('Closed')}
                </span>
              </div>
            )}
          </Card>
        )
      })}
    </div>
  )
}
