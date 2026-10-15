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

import type { ActivityCodesDialogType } from './types'

// ============================================================================
// Activity Code Constants
// ============================================================================

export const ACTIVITY_CODE_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
} as const

export const ACTIVITY_RATIO_MODE = {
  INHERIT: 'inherit',
  OVERRIDE: 'override',
} as const

export const ACTIVITY_GRANT_STATUS = {
  ACTIVE: 'active',
  EXPIRED: 'expired',
  REVOKED: 'revoked',
} as const

export const ACTIVITY_FILTER_VALUES = ['1', '2', 'expired'] as const

export const ACTIVITY_FILTER = {
  ENABLED: '1',
  DISABLED: '2',
  EXPIRED: 'expired',
} as const

export function getActivityStatusFilterOptions(t: TFunction) {
  return [
    { label: t('Enabled'), value: ACTIVITY_FILTER.ENABLED },
    { label: t('Disabled'), value: ACTIVITY_FILTER.DISABLED },
    { label: t('Ended'), value: ACTIVITY_FILTER.EXPIRED },
  ]
}

export function getActivityStatusLabelKey(status: number): string {
  return status === ACTIVITY_CODE_STATUS.ENABLED ? 'Enabled' : 'Disabled'
}

export function getActivityRatioModeOptions(t: TFunction) {
  return [
    { label: t('Group Default Ratio'), value: ACTIVITY_RATIO_MODE.INHERIT },
    { label: t('Custom Ratio'), value: ACTIVITY_RATIO_MODE.OVERRIDE },
  ]
}

export function getActivityGrantStatusLabelKey(status: string): string {
  if (status === ACTIVITY_GRANT_STATUS.EXPIRED) return 'Expired'
  if (status === ACTIVITY_GRANT_STATUS.REVOKED) return 'Revoked'
  return 'Active'
}

export const ACTIVITY_CODE_DIALOG_TYPES: ActivityCodesDialogType[] = [
  'create',
  'update',
  'delete',
  'grants',
]

export const ERROR_MESSAGES = {
  LOAD_FAILED: 'Failed to load activity codes',
  SEARCH_FAILED: 'Failed to search activity codes',
  CREATE_FAILED: 'Failed to create activity code',
  UPDATE_FAILED: 'Failed to update activity code',
  DELETE_FAILED: 'Failed to delete activity code',
  STATUS_FAILED: 'Failed to update activity code status',
  GROUPS_FAILED: 'Failed to load groups',
  GRANTS_FAILED: 'Failed to load activity grants',
  REVOKE_FAILED: 'Failed to revoke activity grant',
} as const

export const SUCCESS_MESSAGES = {
  CREATED: 'Activity code created',
  UPDATED: 'Activity code updated',
  DELETED: 'Activity code deleted',
  DISABLED: 'Activity code disabled',
  ENABLED: 'Activity code enabled',
  GRANT_REVOKED: 'Activity grant revoked',
} as const

// 活动码允许任意语言的字母/数字，以及下划线和连字符；不接受空格与其它符号。
export const ACTIVITY_CODE_PATTERN = /^[\p{L}\p{N}_-]+$/u

export const ACTIVITY_CODE_FORMAT_MESSAGE =
  'Activity code must be 1-64 letters (any language), digits, _ or -'

export const ACTIVITY_VALIDATION = {
  CODE_MAX_LENGTH: 64,
  NAME_MAX_LENGTH: 64,
  DURATION_MAX_HOURS: 87600,
  RPM_MAX: 100000,
  RATIO_MAX: 1000,
  LIMIT_MAX: 1000000,
} as const
