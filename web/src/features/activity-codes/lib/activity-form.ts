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

import { parseQuotaFromDollars, quotaUnitsToEditableAmount } from '@/lib/format'

import {
  ACTIVITY_BENEFIT_SCOPE,
  ACTIVITY_CODE_FORMAT_MESSAGE,
  ACTIVITY_CODE_PATTERN,
  ACTIVITY_CODE_TYPE,
  ACTIVITY_LIMIT_MODE,
  ACTIVITY_RATIO_MODE,
  ACTIVITY_RATIO_SCOPE,
  ACTIVITY_VALIDATION,
} from '../constants'
import type {
  ActivityCode,
  ActivityCodeFormData,
  ActivityCodeType,
  ActivityLimitMode,
} from '../types'
import {
  type ActivityBenefitRow,
  getActivityBenefitCombination,
  getActivityBenefitRowSchema,
  transformBenefitRowsToPayload,
  transformBenefitsToRows,
} from './activity-benefit'

export type ActivityModelRatioRow = {
  model: string
  ratio: string
}

export function getActivityCodeFormSchema(t: TFunction) {
  return z
    .object({
      code: z
        .string()
        .trim()
        .min(1, t('Please enter an activity code'))
        // 按字符数（码点）计长度：一个汉字算一个字符，而不是三个字节。
        .refine(
          (value) => [...value].length <= ACTIVITY_VALIDATION.CODE_MAX_LENGTH,
          t(ACTIVITY_CODE_FORMAT_MESSAGE)
        )
        .refine(
          (value) => ACTIVITY_CODE_PATTERN.test(value),
          t(ACTIVITY_CODE_FORMAT_MESSAGE)
        ),
      name: z
        .string()
        .min(1, t('Please enter a name'))
        .max(
          ACTIVITY_VALIDATION.NAME_MAX_LENGTH,
          t('Name must be 1-64 characters')
        ),
      description: z.string(),
      type: z.enum([ACTIVITY_CODE_TYPE.RATIO, ACTIVITY_CODE_TYPE.BENEFIT]),
      // 行内校验只在 benefit 类型下执行（ratio 类型忽略遗留权益行），
      // 因此这里先接受任意行数据，由 superRefine 逐行套用行 schema。
      benefits: z.array(z.any()),
      group_name: z.string(),
      duration_hours: z
        .number()
        .min(0, t('Validity must be between 0 and 87600 hours'))
        .max(
          ACTIVITY_VALIDATION.DURATION_MAX_HOURS,
          t('Validity must be between 0 and 87600 hours')
        ),
      // 限流二选一：RPM 与并发都是 0..100000 整数，0 = 不限；
      // 未启用的限流类型数值不生效且提交时归 0，因此范围校验只针对当前
      // 选中的那个输入（见 superRefine），未选中输入的非法值不阻止保存。
      limit_mode: z.enum([
        ACTIVITY_LIMIT_MODE.RPM,
        ACTIVITY_LIMIT_MODE.CONCURRENCY,
      ]),
      rpm_limit: z.number(),
      concurrency_limit: z.number(),
      price_dollars: z.number().min(0, t('Price must be zero or greater')),
      ratio_mode: z.enum([
        ACTIVITY_RATIO_MODE.INHERIT,
        ACTIVITY_RATIO_MODE.OVERRIDE,
      ]),
      ratio_override: z
        .number()
        .min(0, t('Ratio must be between 0 and 1000'))
        .max(
          ACTIVITY_VALIDATION.RATIO_MAX,
          t('Ratio must be between 0 and 1000')
        ),
      ratio_scope: z.enum([
        ACTIVITY_RATIO_SCOPE.GROUP,
        ACTIVITY_RATIO_SCOPE.MODELS,
      ]),
      model_ratio_overrides: z.array(
        z.object({
          model: z
            .string()
            .trim()
            .max(
              ACTIVITY_VALIDATION.MODEL_NAME_MAX_LENGTH,
              t('Model name must be 1-255 characters')
            ),
          ratio: z.string(),
        })
      ),
      start_time: z.date().optional(),
      end_time: z.date().optional(),
      // 活动到期时间：到达后活动立即失效，与授权自身的有效期无关，留空表示不限制。
      active_end_time: z.date().optional(),
      // 卡片排序权重：数字越大越靠前，与后端 0..1000000 的范围保持一致。
      sort_weight: z
        .number()
        .min(0, t('Sort weight must be between 0 and 1000000'))
        .max(
          ACTIVITY_VALIDATION.SORT_WEIGHT_MAX,
          t('Sort weight must be between 0 and 1000000')
        ),
      // 卡片背景图：留空不启用；非空须是 http(s) 链接或以 / 开头的站内路径。
      image_url: z
        .string()
        .trim()
        .max(
          ACTIVITY_VALIDATION.IMAGE_URL_MAX_LENGTH,
          t('Background image URL must be at most 512 characters')
        )
        .refine(
          (value) =>
            value === '' ||
            value.startsWith('/') ||
            /^https?:\/\//i.test(value),
          t('Enter an http(s) URL or a path starting with /.')
        ),
      total_limit: z
        .number()
        .min(0, t('Total limit must be 0 or greater'))
        .max(ACTIVITY_VALIDATION.LIMIT_MAX, t('Total limit is too large')),
      per_user_limit: z
        .number()
        .min(0, t('Per-user limit must be 0 or greater'))
        .max(ACTIVITY_VALIDATION.LIMIT_MAX, t('Per-user limit is too large')),
      public_visible: z.boolean(),
    })
    .superRefine((values, ctx) => {
      if (values.type === ACTIVITY_CODE_TYPE.BENEFIT) {
        if (values.benefits.length === 0) {
          ctx.addIssue({
            code: 'custom',
            path: ['benefits'],
            message: t('Add at least one benefit'),
          })
          return
        }
        if (values.benefits.length > ACTIVITY_VALIDATION.BENEFIT_MAX_ENTRIES) {
          ctx.addIssue({
            code: 'custom',
            path: ['benefits'],
            message: t('At most {{count}} benefits can be added', {
              count: ACTIVITY_VALIDATION.BENEFIT_MAX_ENTRIES,
            }),
          })
          return
        }
        // 逐行套用行 schema（必填、RPM、Token 上限），再检查跨行的组合重复。
        const benefitRowSchema = getActivityBenefitRowSchema(t)
        const seenCombinations = new Set<string>()
        values.benefits.forEach((row, index) => {
          const parsedRow = benefitRowSchema.safeParse(row)
          if (!parsedRow.success) {
            for (const issue of parsedRow.error.issues) {
              ctx.addIssue({
                ...issue,
                path: ['benefits', index, ...(issue.path ?? [])],
              })
            }
            return
          }
          const combination = getActivityBenefitCombination(row)
          if (seenCombinations.has(combination)) {
            ctx.addIssue({
              code: 'custom',
              path: [
                'benefits',
                index,
                row.scope === ACTIVITY_BENEFIT_SCOPE.MODEL
                  ? 'model_name'
                  : 'group_name',
              ],
              message: t('This benefit is already configured'),
            })
            return
          }
          seenCombinations.add(combination)
        })
        return
      }

      // ratio 类型：只校验当前限流类型选中的数值；另一个输入已禁用且提交归 0。
      if (values.limit_mode === ACTIVITY_LIMIT_MODE.CONCURRENCY) {
        const limit = values.concurrency_limit
        if (!Number.isInteger(limit)) {
          ctx.addIssue({
            code: 'custom',
            path: ['concurrency_limit'],
            message: t('Concurrency limit must be a whole number'),
          })
        } else if (limit < 0) {
          ctx.addIssue({
            code: 'custom',
            path: ['concurrency_limit'],
            message: t('Concurrency limit must be 0 or greater'),
          })
        } else if (limit > ACTIVITY_VALIDATION.CONCURRENCY_MAX) {
          ctx.addIssue({
            code: 'custom',
            path: ['concurrency_limit'],
            message: t('Concurrency limit is too large'),
          })
        }
      } else {
        const limit = values.rpm_limit
        if (!Number.isInteger(limit)) {
          ctx.addIssue({
            code: 'custom',
            path: ['rpm_limit'],
            message: t('RPM limit must be a whole number'),
          })
        } else if (limit < 0) {
          ctx.addIssue({
            code: 'custom',
            path: ['rpm_limit'],
            message: t('RPM limit must be 0 or greater'),
          })
        } else if (limit > ACTIVITY_VALIDATION.RPM_MAX) {
          ctx.addIssue({
            code: 'custom',
            path: ['rpm_limit'],
            message: t('RPM limit is too large'),
          })
        }
      }

      // 分组必填，「指定模型」下每一行都必须给出模型与倍率。
      if (!values.group_name.trim()) {
        ctx.addIssue({
          code: 'custom',
          path: ['group_name'],
          message: t('Please select a group'),
        })
      }
      if (values.ratio_scope !== ACTIVITY_RATIO_SCOPE.MODELS) return
      const rows = values.model_ratio_overrides
      if (rows.length === 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['model_ratio_overrides'],
          message: t('Select at least one model'),
        })
        return
      }
      if (rows.length > ACTIVITY_VALIDATION.MODEL_RATIO_MAX_ENTRIES) {
        ctx.addIssue({
          code: 'custom',
          path: ['model_ratio_overrides'],
          message: t('At most {{count}} models can be configured', {
            count: ACTIVITY_VALIDATION.MODEL_RATIO_MAX_ENTRIES,
          }),
        })
        return
      }
      const seen = new Set<string>()
      rows.forEach((row, index) => {
        const model = row.model.trim()
        if (!model) {
          ctx.addIssue({
            code: 'custom',
            path: ['model_ratio_overrides', index, 'model'],
            message: t('Please enter a model name'),
          })
          return
        }
        if (seen.has(model)) {
          ctx.addIssue({
            code: 'custom',
            path: ['model_ratio_overrides', index, 'model'],
            message: t('Each model can be configured once'),
          })
          return
        }
        seen.add(model)

        const trimmedRatio = row.ratio.trim()
        const ratio = Number(trimmedRatio)
        if (trimmedRatio === '' || !Number.isFinite(ratio)) {
          ctx.addIssue({
            code: 'custom',
            path: ['model_ratio_overrides', index, 'ratio'],
            message: t('Please enter a ratio'),
          })
          return
        }
        if (ratio < 0 || ratio > ACTIVITY_VALIDATION.RATIO_MAX) {
          ctx.addIssue({
            code: 'custom',
            path: ['model_ratio_overrides', index, 'ratio'],
            message: t('Ratio must be between 0 and 1000'),
          })
        }
      })
    })
}

export type ActivityCodeFormValues = {
  code: string
  name: string
  description: string
  group_name: string
  duration_hours: number
  limit_mode: ActivityLimitMode
  rpm_limit: number
  concurrency_limit: number
  price_dollars: number
  ratio_mode: (typeof ACTIVITY_RATIO_MODE)[keyof typeof ACTIVITY_RATIO_MODE]
  ratio_override: number
  ratio_scope: (typeof ACTIVITY_RATIO_SCOPE)[keyof typeof ACTIVITY_RATIO_SCOPE]
  model_ratio_overrides: ActivityModelRatioRow[]
  start_time?: Date
  end_time?: Date
  active_end_time?: Date
  sort_weight: number
  image_url: string
  total_limit: number
  per_user_limit: number
  public_visible: boolean
  type: ActivityCodeType
  benefits: ActivityBenefitRow[]
}

export const ACTIVITY_CODE_FORM_DEFAULT_VALUES: ActivityCodeFormValues = {
  code: '',
  name: '',
  description: '',
  group_name: '',
  duration_hours: 24,
  limit_mode: ACTIVITY_LIMIT_MODE.RPM,
  rpm_limit: 0,
  concurrency_limit: 0,
  price_dollars: 0,
  ratio_mode: ACTIVITY_RATIO_MODE.INHERIT,
  ratio_override: 0,
  ratio_scope: ACTIVITY_RATIO_SCOPE.GROUP,
  model_ratio_overrides: [],
  start_time: undefined,
  end_time: undefined,
  active_end_time: undefined,
  sort_weight: 0,
  image_url: '',
  total_limit: 0,
  per_user_limit: 1,
  public_visible: false,
  type: ACTIVITY_CODE_TYPE.RATIO,
  benefits: [],
}

function toEpochSeconds(value?: Date): number {
  return value ? Math.floor(value.getTime() / 1000) : 0
}

export function transformFormValuesToPayload(
  values: ActivityCodeFormValues
): ActivityCodeFormData {
  // 限流二选一：未启用的限流类型数值归 0，两个输入互不影响。
  const isConcurrencyLimit =
    values.limit_mode === ACTIVITY_LIMIT_MODE.CONCURRENCY
  const basePayload: ActivityCodeFormData = {
    code: values.code.trim().toLowerCase(),
    name: values.name.trim(),
    description: values.description.trim(),
    group_name: values.group_name,
    duration_hours: values.duration_hours,
    limit_mode: isConcurrencyLimit
      ? ACTIVITY_LIMIT_MODE.CONCURRENCY
      : ACTIVITY_LIMIT_MODE.RPM,
    rpm_limit: isConcurrencyLimit ? 0 : values.rpm_limit,
    concurrency_limit: isConcurrencyLimit ? values.concurrency_limit : 0,
    price_quota: parseQuotaFromDollars(values.price_dollars),
    ratio_mode: ACTIVITY_RATIO_MODE.INHERIT,
    ratio_override: 0,
    ratio_scope: ACTIVITY_RATIO_SCOPE.GROUP,
    model_ratio_overrides: {},
    start_time: toEpochSeconds(values.start_time),
    end_time: toEpochSeconds(values.end_time),
    // 未填到期时间时传 0，后端按「不限制」处理。
    active_end_time: toEpochSeconds(values.active_end_time),
    sort_weight: values.sort_weight,
    image_url: values.image_url.trim(),
    total_limit: values.total_limit,
    per_user_limit: values.per_user_limit,
    public_visible: values.public_visible ? 1 : 0,
    type: ACTIVITY_CODE_TYPE.RATIO,
    benefits: [],
  }

  // benefit 类型以权益为准：顶层分组取第一条权益分组（兼容旧字段），
  // 倍率与活动码级限流不再有意义，统一归一为默认值（rpm + 0 + 0，不参与执行）。
  if (values.type === ACTIVITY_CODE_TYPE.BENEFIT) {
    const benefits = transformBenefitRowsToPayload(values.benefits)
    return {
      ...basePayload,
      type: ACTIVITY_CODE_TYPE.BENEFIT,
      benefits,
      group_name: benefits[0]?.group_name ?? values.group_name.trim(),
      limit_mode: ACTIVITY_LIMIT_MODE.RPM,
      rpm_limit: 0,
      concurrency_limit: 0,
    }
  }

  // 指定模型类型只按模型倍率改价，分组级倍率固定为继承；切回整个分组时清空模型列表。
  const isModelScope = values.ratio_scope === ACTIVITY_RATIO_SCOPE.MODELS
  const modelRatioOverrides: Record<string, number> = {}
  if (isModelScope) {
    for (const row of values.model_ratio_overrides) {
      const model = row.model.trim()
      if (!model) continue
      modelRatioOverrides[model] = Number(row.ratio)
    }
  }

  let ratioOverride = 0
  if (!isModelScope && values.ratio_mode === ACTIVITY_RATIO_MODE.OVERRIDE) {
    ratioOverride = values.ratio_override
  }

  return {
    ...basePayload,
    group_name: values.group_name,
    ratio_mode: isModelScope ? ACTIVITY_RATIO_MODE.INHERIT : values.ratio_mode,
    ratio_override: ratioOverride,
    ratio_scope: values.ratio_scope,
    model_ratio_overrides: modelRatioOverrides,
  }
}

export function transformActivityCodeToFormValues(
  activityCode: ActivityCode
): ActivityCodeFormValues {
  // 存量活动码没有该字段，按「整个分组」处理。
  const ratioScope =
    activityCode.ratio_scope === ACTIVITY_RATIO_SCOPE.MODELS
      ? ACTIVITY_RATIO_SCOPE.MODELS
      : ACTIVITY_RATIO_SCOPE.GROUP
  const modelRatioOverrides = Object.entries(
    activityCode.model_ratio_overrides ?? {}
  )
    .map(([model, ratio]) => ({ model, ratio: String(ratio) }))
    .sort((left, right) => left.model.localeCompare(right.model))
  const type =
    activityCode.type === ACTIVITY_CODE_TYPE.BENEFIT
      ? ACTIVITY_CODE_TYPE.BENEFIT
      : ACTIVITY_CODE_TYPE.RATIO
  const activeEndTime = activityCode.active_end_time ?? 0

  return {
    code: activityCode.code,
    name: activityCode.name,
    description: activityCode.description,
    group_name: activityCode.group_name,
    duration_hours: activityCode.duration_seconds / 3600,
    // 历史数据缺 limit_mode 或为空时按 RPM 处理，缺 concurrency_limit 时按 0（不限）。
    limit_mode:
      activityCode.limit_mode === ACTIVITY_LIMIT_MODE.CONCURRENCY
        ? ACTIVITY_LIMIT_MODE.CONCURRENCY
        : ACTIVITY_LIMIT_MODE.RPM,
    rpm_limit: activityCode.rpm_limit,
    concurrency_limit: activityCode.concurrency_limit ?? 0,
    price_dollars: quotaUnitsToEditableAmount(activityCode.price_quota),
    ratio_mode:
      activityCode.ratio_mode === ACTIVITY_RATIO_MODE.OVERRIDE
        ? ACTIVITY_RATIO_MODE.OVERRIDE
        : ACTIVITY_RATIO_MODE.INHERIT,
    ratio_override: activityCode.ratio_override,
    ratio_scope: ratioScope,
    model_ratio_overrides: modelRatioOverrides,
    start_time:
      activityCode.start_time > 0
        ? new Date(activityCode.start_time * 1000)
        : undefined,
    end_time:
      activityCode.end_time > 0
        ? new Date(activityCode.end_time * 1000)
        : undefined,
    // 0 表示不限制，回填为空；老数据缺字段时同样按默认值处理。
    active_end_time:
      activeEndTime > 0 ? new Date(activeEndTime * 1000) : undefined,
    sort_weight: activityCode.sort_weight ?? 0,
    image_url: activityCode.image_url ?? '',
    total_limit: activityCode.total_limit,
    per_user_limit: activityCode.per_user_limit,
    public_visible: activityCode.public_visible === 1,
    type,
    benefits: transformBenefitsToRows(activityCode.benefits),
  }
}
