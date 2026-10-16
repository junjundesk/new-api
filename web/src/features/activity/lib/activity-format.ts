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

import { formatModelRatioOverrides } from '@/lib/format'

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
  return `${ratioOverride}x`
}

// 「指定模型」类型：直接列出具体模型与倍率（例如 "gpt-5: 0, o3: 0.5"），
// 只有在未指定模型时才回落到「分组全部模型」。
export function describeActivityModelScope(
  t: TFunction,
  modelRatioOverrides: Record<string, number> | null | undefined
): string {
  const listed = formatModelRatioOverrides(modelRatioOverrides)
  if (!listed) return t('All models in the group')
  return listed
}

export function describeExpireTime(
  t: TFunction,
  expireTime: number
): string {
  if (expireTime <= 0) return t('Permanent')
  return new Date(expireTime * 1000).toLocaleString()
}
