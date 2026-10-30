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
import type { TFunction } from 'i18next'

import { formatModelRatioOverrides, formatRatioMultiplier } from '@/lib/format'
import { formatTokenAmount } from '@/lib/token-amount'

import type { ActivityBenefit } from '../types'

// ============================================================================
// Activity display helpers
// ============================================================================

export function describeActivityDuration(
  t: TFunction,
  durationSeconds: number
): string {
  if (durationSeconds <= 0) return t('Permanent')
  const hours = durationSeconds / 3600
  if (hours < 1) {
    return t('{{count}} minutes', { count: Math.round(durationSeconds / 60) })
  }
  if (hours < 24) {
    return t('{{count}} hours', { count: Math.round(hours * 10) / 10 })
  }
  return t('{{count}} days', { count: Math.round((hours / 24) * 10) / 10 })
}

export function describeActivityRatio(
  t: TFunction,
  ratioMode: string,
  ratioOverride: number
): string {
  if (ratioMode !== 'override') return t('Group default ratio')
  if (ratioOverride === 0) return t('Fully free')
  return `${formatRatioMultiplier(ratioOverride)}x`
}

// 「指定模型」类型：直接列出具体模型与倍率（例如 "gpt-5: 0, o3: 0.5"），
// 只有在未指定模型时才回落到「分组全部模型」。
export function describeActivityModelScope(
  t: TFunction,
  modelRatioOverrides: Record<string, number> | null | undefined
): string {
  const listed = formatModelRatioOverrides(t, modelRatioOverrides)
  if (!listed) return t('All models in the group')
  return listed
}

export function describeExpireTime(t: TFunction, expireTime: number): string {
  if (expireTime <= 0) return t('Permanent')
  return new Date(expireTime * 1000).toLocaleString()
}

// 单条活动权益：「分组 · 整组/单模型 · Token 总量/不限量」。
// token_total 为 0 表示不限量免费。
export function describeActivityBenefit(
  t: TFunction,
  benefit: ActivityBenefit,
  language?: string
): string {
  const group = benefit.group_desc || benefit.group_name
  const scope =
    benefit.scope === 'model'
      ? t('Model {{model}}', { model: benefit.model_name })
      : t('Whole group')
  const tokens =
    benefit.token_total > 0
      ? t('{{amount}} tokens', {
          amount: formatTokenAmount(benefit.token_total, language),
        })
      : t('Unlimited tokens')
  return `${group} · ${scope} · ${tokens}`
}

// 多权益活动逐条描述，每条权益一行；旧接口没有 benefits 时返回空串。
export function describeActivityBenefits(
  t: TFunction,
  benefits: ActivityBenefit[] | null | undefined,
  language?: string
): string {
  return (benefits ?? [])
    .map((benefit) => describeActivityBenefit(t, benefit, language))
    .join('\n')
}

// Token 套餐用量：已用/总量 + 剩余；用尽后提示回退原价计费。
// total <= 0 表示不限量免费，此时不显示剩余。
export function describeTokenUsage(
  t: TFunction,
  used: number,
  total: number,
  language?: string
): string {
  if (total <= 0) {
    return t('Used {{used}} / {{total}}', {
      used: formatTokenAmount(used, language),
      total: t('Unlimited'),
    })
  }
  const usage = t('Used {{used}} / {{total}}', {
    used: formatTokenAmount(used, language),
    total: formatTokenAmount(total, language),
  })
  if (used >= total) {
    return `${usage} · ${t('Token quota used up (billed at the original price)')}`
  }
  return `${usage} · ${t('Remaining {{amount}}', {
    amount: formatTokenAmount(total - used, language),
  })}`
}
