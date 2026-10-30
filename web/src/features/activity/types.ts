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
// ============================================================================
// Activity (user side) Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export type ActivityCodeType = 'ratio' | 'benefit'

export type ActivityBenefitScope = 'group' | 'model'

// 一条活动权益：某个分组（整组或单个模型）的免费额度。
// token_total 为 0 表示不限量免费；旧接口不会返回 benefits。
export interface ActivityBenefit {
  key: string
  group_name: string
  group_desc: string
  scope: ActivityBenefitScope
  model_name: string
  rpm_limit: number
  token_total: number
}

export interface ActivityOffer {
  code_id: number
  code: string
  name: string
  description: string
  group_name: string
  group_desc: string
  duration_seconds: number
  rpm_limit: number
  ratio_mode: string
  ratio_override: number
  ratio_scope?: string
  model_ratio_overrides?: Record<string, number> | null
  price_quota: number
  remaining_total: number
  // 旧接口没有 type/benefits：缺失时按 ratio 类型展示。
  type?: ActivityCodeType
  benefits?: ActivityBenefit[] | null
}

export interface ActivityOffersResponse {
  enabled: boolean
  offers: ActivityOffer[]
}

export interface ActivityGrant {
  id: number
  user_id: number
  code_id: number
  code: string
  code_name: string
  group_name: string
  ratio_mode: string
  ratio_override: number
  ratio_scope?: string
  model_ratio_overrides?: Record<string, number> | null
  rpm_limit: number
  price_quota: number
  start_time: number
  expire_time: number
  status: string
  remark: string
  created_time: number
  // benefit 授权：benefit_key 非空，token_total 为 0 表示不限量免费。
  type?: ActivityCodeType
  benefit_key?: string
  redeem_batch?: string
  token_total?: number
  token_used?: number
}

export interface ActivityActiveView {
  grant_id: number
  group_name: string
  code: string
  code_name: string
  ratio_mode: string
  ratio_override: number
  ratio_scope?: string
  model_ratio_overrides?: Record<string, number> | null
  rpm_limit: number
  expire_time: number
  type?: ActivityCodeType
  benefit_key?: string
  redeem_batch?: string
  token_total?: number
  token_used?: number
}

export interface ActivityGrantsResponse {
  grants: ActivityGrant[]
  active: ActivityActiveView[]
}

export interface ActivityPreview {
  code_id: number
  code: string
  name: string
  description: string
  group_name: string
  group_desc: string
  duration_seconds: number
  rpm_limit: number
  ratio_mode: string
  ratio_override: number
  ratio_scope?: string
  model_ratio_overrides?: Record<string, number> | null
  price_quota: number
  balance_quota: number
  bonus_quota: number
  can_afford: boolean
  need_confirm: boolean
  already_granted: boolean
  current_expire_time: number
  remaining_total: number
  redeemable: boolean
  blocked_reason: string
  message: string
  type?: ActivityCodeType
  benefits?: ActivityBenefit[] | null
}

export interface RedeemActivityResult {
  grant_id: number
  code_id: number
  code: string
  code_name: string
  group_name: string
  expire_time: number
  price_quota: number
  balance_after: number
  // benefit 类型一次兑换可能覆盖多个分组；旧接口缺失时回退 group_name。
  group_names?: string[]
}

export type RedeemOutcome =
  | { ok: true; data: RedeemActivityResult }
  | { ok: false; message: string; preview?: ActivityPreview }
