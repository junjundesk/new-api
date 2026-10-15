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
  ACTIVITY_RATIO_MODE,
  ACTIVITY_VALIDATION,
} from '../constants'
import type { ActivityCode, ActivityCodeFormData } from '../types'

export function getActivityCodeFormSchema(t: TFunction) {
  return z.object({
    code: z
      .string()
      .min(1, t('Please enter an activity code'))
      .max(
        ACTIVITY_VALIDATION.CODE_MAX_LENGTH,
        t('Activity code must be 1-64 characters of a-z, 0-9, _ or -')
      )
      .regex(
        /^[a-z0-9_-]+$/,
        t('Activity code must be 1-64 characters of a-z, 0-9, _ or -')
      ),
    name: z
      .string()
      .min(1, t('Please enter a name'))
      .max(
        ACTIVITY_VALIDATION.NAME_MAX_LENGTH,
        t('Name must be 1-64 characters')
      ),
    description: z.string(),
    group_name: z.string().min(1, t('Please select a group')),
    duration_hours: z
      .number()
      .min(0, t('Validity must be between 0 and 87600 hours'))
      .max(
        ACTIVITY_VALIDATION.DURATION_MAX_HOURS,
        t('Validity must be between 0 and 87600 hours')
      ),
    rpm_limit: z
      .number()
      .min(0, t('RPM limit must be 0 or greater'))
      .max(ACTIVITY_VALIDATION.RPM_MAX, t('RPM limit is too large')),
    price_dollars: z.number().min(0, t('Price must be zero or greater')),
    ratio_mode: z.enum([
      ACTIVITY_RATIO_MODE.INHERIT,
      ACTIVITY_RATIO_MODE.OVERRIDE,
    ]),
    ratio_override: z
      .number()
      .min(0, t('Ratio must be between 0 and 1000'))
      .max(ACTIVITY_VALIDATION.RATIO_MAX, t('Ratio must be between 0 and 1000')),
    start_time: z.date().optional(),
    end_time: z.date().optional(),
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
}

export type ActivityCodeFormValues = {
  code: string
  name: string
  description: string
  group_name: string
  duration_hours: number
  rpm_limit: number
  price_dollars: number
  ratio_mode: (typeof ACTIVITY_RATIO_MODE)[keyof typeof ACTIVITY_RATIO_MODE]
  ratio_override: number
  start_time?: Date
  end_time?: Date
  total_limit: number
  per_user_limit: number
  public_visible: boolean
}

export const ACTIVITY_CODE_FORM_DEFAULT_VALUES: ActivityCodeFormValues = {
  code: '',
  name: '',
  description: '',
  group_name: '',
  duration_hours: 24,
  rpm_limit: 0,
  price_dollars: 0,
  ratio_mode: ACTIVITY_RATIO_MODE.INHERIT,
  ratio_override: 0,
  start_time: undefined,
  end_time: undefined,
  total_limit: 0,
  per_user_limit: 1,
  public_visible: false,
}

function toEpochSeconds(value?: Date): number {
  return value ? Math.floor(value.getTime() / 1000) : 0
}

export function transformFormValuesToPayload(
  values: ActivityCodeFormValues
): ActivityCodeFormData {
  return {
    code: values.code.trim().toLowerCase(),
    name: values.name.trim(),
    description: values.description.trim(),
    group_name: values.group_name,
    duration_hours: values.duration_hours,
    rpm_limit: values.rpm_limit,
    price_quota: parseQuotaFromDollars(values.price_dollars),
    ratio_mode: values.ratio_mode,
    ratio_override:
      values.ratio_mode === ACTIVITY_RATIO_MODE.OVERRIDE
        ? values.ratio_override
        : 0,
    start_time: toEpochSeconds(values.start_time),
    end_time: toEpochSeconds(values.end_time),
    total_limit: values.total_limit,
    per_user_limit: values.per_user_limit,
    public_visible: values.public_visible ? 1 : 0,
  }
}

export function transformActivityCodeToFormValues(
  activityCode: ActivityCode
): ActivityCodeFormValues {
  return {
    code: activityCode.code,
    name: activityCode.name,
    description: activityCode.description,
    group_name: activityCode.group_name,
    duration_hours: activityCode.duration_seconds / 3600,
    rpm_limit: activityCode.rpm_limit,
    price_dollars: quotaUnitsToEditableAmount(activityCode.price_quota),
    ratio_mode:
      activityCode.ratio_mode === ACTIVITY_RATIO_MODE.OVERRIDE
        ? ACTIVITY_RATIO_MODE.OVERRIDE
        : ACTIVITY_RATIO_MODE.INHERIT,
    ratio_override: activityCode.ratio_override,
    start_time:
      activityCode.start_time > 0
        ? new Date(activityCode.start_time * 1000)
        : undefined,
    end_time:
      activityCode.end_time > 0
        ? new Date(activityCode.end_time * 1000)
        : undefined,
    total_limit: activityCode.total_limit,
    per_user_limit: activityCode.per_user_limit,
    public_visible: activityCode.public_visible === 1,
  }
}
