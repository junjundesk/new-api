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

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { formatQuota } from '@/lib/format'

import {
  describeActivityBenefit,
  describeActivityDuration,
  describeActivityModelScope,
  describeActivityRatio,
} from '../../lib/activity-format'
import type { ActivityPreview } from '../../types'

type ActivityRedeemConfirmDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  preview: ActivityPreview | null
  redeeming: boolean
  onConfirm: () => void
}

export function ActivityRedeemConfirmDialog(
  props: ActivityRedeemConfirmDialogProps
) {
  const { t, i18n } = useTranslation()
  if (!props.preview) return null

  const preview = props.preview
  const benefits = preview.benefits ?? []
  // 旧接口没有 type/benefits：按 ratio 预览展示，保持原行为。
  const isBenefit = preview.type === 'benefit' && benefits.length > 0
  // benefit 类型可能覆盖多个分组，确认文案里全部列出。
  const benefitGroups = isBenefit
    ? [
        ...new Set(
          benefits.map((benefit) => benefit.group_desc || benefit.group_name)
        ),
      ]
    : []
  const targetLabel =
    benefitGroups.length > 0
      ? benefitGroups.join(', ')
      : preview.group_desc || preview.group_name

  // benefit 类型以权益列表为准，不再展示 ratio/模型倍率行。
  const rows: { label: string; value: string }[] = isBenefit
    ? [
        {
          label: t('Validity'),
          value: describeActivityDuration(t, preview.duration_seconds),
        },
        {
          label: t('Wallet balance'),
          value: formatQuota(preview.balance_quota),
        },
      ]
    : [
        {
          label: t('Group'),
          value: preview.group_desc || preview.group_name,
        },
        {
          label: t('Validity'),
          value: describeActivityDuration(t, preview.duration_seconds),
        },
        {
          label: t('Ratio'),
          value: describeActivityRatio(
            t,
            preview.ratio_mode,
            preview.ratio_override
          ),
        },
        {
          label: t('Models (ratio)'),
          value: describeActivityModelScope(t, preview.model_ratio_overrides),
        },
        {
          label: t('RPM limit'),
          value:
            preview.rpm_limit > 0 ? String(preview.rpm_limit) : t('Unlimited'),
        },
        {
          label: t('Wallet balance'),
          value: formatQuota(preview.balance_quota),
        },
      ]

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-[480px]'>
        <DialogHeader>
          <DialogTitle>{t('Confirm Redemption')}</DialogTitle>
          <DialogDescription>
            {t('Spend {{quota}} of your wallet balance to redeem {{group}}?', {
              quota: formatQuota(preview.price_quota),
              group: targetLabel,
            })}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-2'>
          {isBenefit && (
            <div className='space-y-1'>
              <p className='text-muted-foreground text-sm'>{t('Benefits')}</p>
              <ul className='space-y-1'>
                {benefits.map((benefit) => (
                  <li
                    key={`${benefit.key}:${benefit.group_name}:${benefit.model_name}`}
                    className='text-sm font-medium break-all'
                  >
                    {describeActivityBenefit(t, benefit, i18n.language)}
                  </li>
                ))}
              </ul>
            </div>
          )}
          {rows.map((row) => (
            <div
              key={row.label}
              className='flex items-start justify-between gap-4 text-sm'
            >
              <span className='text-muted-foreground shrink-0'>
                {row.label}
              </span>
              <span
                className='min-w-0 text-right font-medium break-all'
                title={row.value}
              >
                {row.value}
              </span>
            </div>
          ))}
          <p className='text-muted-foreground text-xs'>
            {t('Bonus credit cannot be used for redemption.')}
          </p>
        </div>

        <DialogFooter>
          <Button
            variant='outline'
            disabled={props.redeeming}
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            disabled={props.redeeming || !preview.can_afford}
            onClick={props.onConfirm}
          >
            {t('Confirm and Redeem')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
