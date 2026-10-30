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
import { z } from 'zod'

import { formatTokenAmount, parseTokenAmountInput } from '@/lib/token-amount'

import {
  ACTIVITY_BENEFIT_SCOPE,
  ACTIVITY_BENEFIT_TOKEN_MODE,
  ACTIVITY_VALIDATION,
} from '../constants'
import {
  activityBenefitScopeSchema,
  type ActivityBenefit,
  type ActivityBenefitScope,
} from '../types'

export type ActivityBenefitTokenMode =
  (typeof ACTIVITY_BENEFIT_TOKEN_MODE)[keyof typeof ACTIVITY_BENEFIT_TOKEN_MODE]

// 权益编辑器的行数据：token_total 以文本保存，提交时按 token_mode 解析。
export type ActivityBenefitRow = {
  key: string
  group_name: string
  scope: (typeof ACTIVITY_BENEFIT_SCOPE)[keyof typeof ACTIVITY_BENEFIT_SCOPE]
  model_name: string
  rpm_limit: number
  token_mode: ActivityBenefitTokenMode
  token_total_input: string
}

export function createActivityBenefitRow(
  overrides: Partial<ActivityBenefitRow> = {}
): ActivityBenefitRow {
  return {
    key: '',
    group_name: '',
    scope: ACTIVITY_BENEFIT_SCOPE.GROUP,
    model_name: '',
    rpm_limit: 0,
    token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.UNLIMITED,
    token_total_input: '',
    ...overrides,
  }
}

// 「分组 + 范围 + 单模型」组合在一条活动码内唯一，用于去重校验。
export function getActivityBenefitCombination(
  row: Pick<ActivityBenefitRow, 'group_name' | 'scope' | 'model_name'>
): string {
  const model =
    row.scope === ACTIVITY_BENEFIT_SCOPE.MODEL ? row.model_name.trim() : ''
  return [row.group_name.trim(), row.scope, model].join('\u0000')
}

export function getActivityBenefitRowSchema(t: TFunction) {
  return z
    .object({
      key: z.string(),
      group_name: z.string(),
      scope: activityBenefitScopeSchema,
      model_name: z
        .string()
        .max(
          ACTIVITY_VALIDATION.MODEL_NAME_MAX_LENGTH,
          t('Model name must be 1-255 characters')
        ),
      rpm_limit: z
        .number()
        .int(t('RPM limit must be a whole number'))
        .min(0, t('RPM limit must be 0 or greater'))
        .max(ACTIVITY_VALIDATION.RPM_MAX, t('RPM limit is too large')),
      token_mode: z.enum([
        ACTIVITY_BENEFIT_TOKEN_MODE.UNLIMITED,
        ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE,
      ]),
      token_total_input: z.string(),
    })
    .superRefine((row, ctx) => {
      if (!row.group_name.trim()) {
        ctx.addIssue({
          code: 'custom',
          path: ['group_name'],
          message: t('Please select a group'),
        })
      }
      if (
        row.scope === ACTIVITY_BENEFIT_SCOPE.MODEL &&
        !row.model_name.trim()
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['model_name'],
          message: t('Please select a model'),
        })
      }
      // 「不限量」模式不读取输入框；套餐模式必须解析出大于 0 的 Token 总量。
      if (row.token_mode !== ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE) return
      const trimmed = row.token_total_input.trim()
      if (trimmed === '') {
        ctx.addIssue({
          code: 'custom',
          path: ['token_total_input'],
          message: t('Please enter a token amount'),
        })
        return
      }
      const parsed = parseTokenAmountInput(trimmed)
      if (!parsed.ok) {
        ctx.addIssue({
          code: 'custom',
          path: ['token_total_input'],
          message: t(
            'Enter a valid token amount, for example 1000000, 1M or 100万'
          ),
        })
        return
      }
      if (parsed.value <= 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['token_total_input'],
          message: t('Token amount must be greater than 0'),
        })
      }
    })
}

export function transformBenefitRowsToPayload(
  rows: ActivityBenefitRow[]
): ActivityBenefit[] {
  return rows.map((row) => {
    const isModelScope = row.scope === ACTIVITY_BENEFIT_SCOPE.MODEL
    const parsed =
      row.token_mode === ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE
        ? parseTokenAmountInput(row.token_total_input)
        : null
    return {
      key: row.key,
      group_name: row.group_name.trim(),
      scope: row.scope,
      model_name: isModelScope ? row.model_name.trim() : '',
      rpm_limit: Number.isFinite(row.rpm_limit) ? Math.trunc(row.rpm_limit) : 0,
      token_total: parsed?.ok ? parsed.value : 0,
    }
  })
}

export function transformBenefitsToRows(
  benefits: ActivityBenefit[] | null | undefined
): ActivityBenefitRow[] {
  return (benefits ?? []).map((benefit) => {
    const scope =
      benefit.scope === ACTIVITY_BENEFIT_SCOPE.MODEL
        ? ACTIVITY_BENEFIT_SCOPE.MODEL
        : ACTIVITY_BENEFIT_SCOPE.GROUP
    const tokenTotal = benefit.token_total ?? 0
    return {
      key: benefit.key ?? '',
      group_name: benefit.group_name,
      scope,
      model_name: scope === ACTIVITY_BENEFIT_SCOPE.MODEL ? benefit.model_name ?? '' : '',
      rpm_limit: benefit.rpm_limit ?? 0,
      // 回填用原始整数字符串，避免单位格式化造成精度丢失。
      token_mode:
        tokenTotal > 0
          ? ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE
          : ACTIVITY_BENEFIT_TOKEN_MODE.UNLIMITED,
      token_total_input: tokenTotal > 0 ? String(tokenTotal) : '',
    }
  })
}
// 权益范围文案：整组，或具体模型名。
export function getActivityBenefitScopeLabel(
  t: TFunction,
  scope: ActivityBenefitScope,
  modelName: string
): string {
  return scope === ACTIVITY_BENEFIT_SCOPE.MODEL ? modelName : t('Whole group')
}

// 权益 Token 总量文案：0 表示不限量免费。
export function formatActivityBenefitTokenTotal(
  t: TFunction,
  tokenTotal: number,
  language: string
): string {
  if (tokenTotal > 0) {
    return t('{{amount}} tokens', {
      amount: formatTokenAmount(tokenTotal, language),
    })
  }
  return t('Unlimited tokens')
}

// 管理端列表/赠送弹窗的单条权益摘要：分组 · 范围 · Token 总量。
export function describeActivityBenefit(
  t: TFunction,
  benefit: ActivityBenefit,
  language: string
): string {
  const scopeLabel = getActivityBenefitScopeLabel(
    t,
    benefit.scope,
    benefit.model_name
  )
  const tokenLabel = formatActivityBenefitTokenTotal(
    t,
    benefit.token_total,
    language
  )
  return `${benefit.group_name} · ${scopeLabel} · ${tokenLabel}`
}

// 授权列表的 Token 用量文案，配合 formatTokenAmount 使用。
export function formatActivityBenefitUsage(
  t: TFunction,
  tokenUsed: number,
  tokenTotal: number,
  language: string
): string {
  if (tokenTotal > 0) {
    return t('Used {{used}} / {{total}} tokens', {
      used: formatTokenAmount(tokenUsed, language),
      total: formatTokenAmount(tokenTotal, language),
    })
  }
  return t('Used {{used}} tokens · Unlimited', {
    used: formatTokenAmount(tokenUsed, language),
  })
}
