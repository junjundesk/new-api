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
import { z } from 'zod'

// ============================================================================
// Activity Code Type & Benefits
// ============================================================================

export const activityCodeTypeSchema = z.enum(['ratio', 'benefit'])

export type ActivityCodeType = z.infer<typeof activityCodeTypeSchema>

// 权益范围：整组免费，或只对单个模型免费。
export const activityBenefitScopeSchema = z.enum(['group', 'model'])

export type ActivityBenefitScope = z.infer<typeof activityBenefitScopeSchema>

// 一条免费权益：目标分组 + 范围 + 单模型名 + 独立 RPM + Token 总量（0 = 不限量）。
// 创建/更新时已有权益的 key 原样回传，新权益传空字符串由后端生成。
export const activityBenefitSchema = z.object({
  key: z.string(),
  group_name: z.string(),
  scope: activityBenefitScopeSchema,
  model_name: z.string().default(''),
  rpm_limit: z.number(),
  token_total: z.number(),
})

export type ActivityBenefit = z.infer<typeof activityBenefitSchema>

// ============================================================================
// Activity Code Schema & Types
// ============================================================================

export const activityCodeSchema = z.object({
  id: z.number(),
  code: z.string(),
  name: z.string(),
  description: z.string(),
  // 老数据没有 type/benefits：按 ratio 类型、无权益处理；非法值也回退 ratio。
  type: activityCodeTypeSchema.catch('ratio'),
  benefits: z.array(activityBenefitSchema).nullable().default(null),
  status: z.number(), // 1: enabled, 2: disabled
  group_name: z.string(),
  duration_seconds: z.number(),
  rpm_limit: z.number(),
  price_quota: z.number(),
  ratio_mode: z.string(), // inherit | override
  ratio_override: z.number(),
  ratio_scope: z.string(), // group | models
  model_ratio_overrides: z.record(z.string(), z.number()).nullish(),
  start_time: z.number(),
  end_time: z.number(),
  total_limit: z.number(),
  used_count: z.number(),
  per_user_limit: z.number(),
  public_visible: z.number(),
  created_by: z.number(),
  created_time: z.number(),
  updated_time: z.number(),
})

export type ActivityCode = z.infer<typeof activityCodeSchema>

export const activityGrantSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  code_id: z.number(),
  code: z.string(),
  code_name: z.string(),
  group_name: z.string(),
  ratio_mode: z.string(),
  ratio_override: z.number(),
  ratio_scope: z.string(),
  model_ratio_overrides: z.record(z.string(), z.number()).nullish(),
  rpm_limit: z.number(),
  price_quota: z.number(),
  start_time: z.number(),
  expire_time: z.number(),
  status: z.string(),
  source: z.string().optional(),
  remark: z.string(),
  created_time: z.number(),
  // benefit 类型扩展字段：同一 redeem_batch 的多条授权属于同一次兑换。
  redeem_batch: z.string().default(''),
  benefit_key: z.string().default(''),
  token_total: z.number().default(0),
  token_used: z.number().default(0),
})

export type ActivityGrant = z.infer<typeof activityGrantSchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PagedActivityCodes {
  items: ActivityCode[]
  total: number
  page: number
  page_size: number
}

export interface PagedActivityGrants {
  items: ActivityGrant[]
  total: number
  page: number
  page_size: number
}

export interface GetActivityCodesParams {
  p?: number
  page_size?: number
}

export interface SearchActivityCodesParams extends GetActivityCodesParams {
  keyword?: string
  status?: string
}

export interface ActivityCodeFormData {
  id?: number
  code: string
  name: string
  description: string
  group_name: string
  duration_hours: number
  rpm_limit: number
  price_quota: number
  ratio_mode: string
  ratio_override: number
  ratio_scope: string
  model_ratio_overrides: Record<string, number>
  start_time: number
  end_time: number
  total_limit: number
  per_user_limit: number
  public_visible: number
  type: ActivityCodeType
  benefits: ActivityBenefit[]
}

// 分组内在架模型（GET /api/models/group_members），用作指定模型的选择建议。
export interface GroupModelMember {
  model_name: string
  description?: string
  group_description?: string
}

// 管理员赠送活动码（POST /api/activity/:id/grants）。
export interface GrantActivityCodePayload {
  user_ids: number[]
  remark?: string
}

export interface GrantActivityCodeItem {
  user_id: number
  success: boolean
  grant_id?: number
  expire_time?: number
  group_name?: string
  // benefit 类型一次赠送可能覆盖多个分组；缺失时回退 group_name。
  group_names?: string[]
  message?: string
}

export interface GrantActivityCodeResult {
  granted: number
  failed: number
  total: number
  results: GrantActivityCodeItem[]
}

// ============================================================================
// Dialog Types
// ============================================================================

export type ActivityCodesDialogType =
  | 'create'
  | 'update'
  | 'delete'
  | 'grants'
  | 'grant'
