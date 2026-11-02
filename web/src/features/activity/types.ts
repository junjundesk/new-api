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

// 限流类型：按每分钟请求数（RPM）或按并发数限流，一条配置二选一。
export type ActivityLimitMode = 'rpm' | 'concurrency'

// active 聚合视图的限流类型：同组混合授权会同时存在 RPM 与并发约束，
// 聚合结果为 mixed，两个约束的有效值必须分别展示，不能只挑一种覆盖。
export type ActivityActiveLimitMode = ActivityLimitMode | 'mixed'

// 一条活动权益：某个分组（整组或单个模型）的免费额度。
// token_total 为 0 表示不限量免费；旧接口不会返回 benefits。
// 历史数据缺 limit_mode 时按 RPM，缺 concurrency_limit 时按 0（不限）。
export interface ActivityBenefit {
  key: string
  group_name: string
  group_desc: string
  scope: ActivityBenefitScope
  model_name: string
  limit_mode?: ActivityLimitMode
  rpm_limit: number
  concurrency_limit?: number
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
  // 历史数据缺 limit_mode 时按 RPM，缺 concurrency_limit 时按 0（不限）。
  limit_mode?: ActivityLimitMode
  rpm_limit: number
  concurrency_limit?: number
  ratio_mode: string
  ratio_override: number
  ratio_scope?: string
  model_ratio_overrides?: Record<string, number> | null
  price_quota: number
  remaining_total: number
  // 旧接口没有 type/benefits：缺失时按 ratio 类型展示。
  type?: ActivityCodeType
  benefits?: ActivityBenefit[] | null
  // 排序权重，数值越大越靠前；旧接口不返回时按后端顺序展示。
  sort_weight?: number
  // 活动背景图 URL；空串表示没有背景图。
  image_url?: string
  // 活动结束时间（unix 秒），0 表示不限。
  active_end_time?: number
  // 售罄或活动到期后后端不再过滤该活动，而是返回 redeemable=false；
  // 旧接口没有这两个字段，缺失时按可兑换处理。
  redeemable?: boolean
  blocked_reason?: string
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
  // 历史数据缺 limit_mode 时按 RPM，缺 concurrency_limit 时按 0（不限）。
  limit_mode?: ActivityLimitMode
  rpm_limit: number
  concurrency_limit?: number
  price_quota: number
  start_time: number
  expire_time: number
  // 生效到期时间 = min(expire_time, activity_end_time)，0 表示永久；
  // 活动可能先于卡密有效期结束，旧接口不返回该字段时回退 expire_time。
  effective_expire_time?: number
  // 活动结束时间（unix 秒），0 表示不限。
  activity_end_time?: number
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
  // 同组混合授权聚合后 limit_mode 可能为 "mixed"：rpm_limit 与 concurrency_limit
  // 都是该组聚合后的有效值（各自 0 = 不限），展示时两者都要给出；
  // 缺省字段按 RPM / 0（不限）处理。
  limit_mode?: ActivityActiveLimitMode
  rpm_limit: number
  concurrency_limit?: number
  expire_time: number
  // 该聚合行是否带 Token 总量限制（token_total > 0）。
  token_limited?: boolean
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
  // 历史数据缺 limit_mode 时按 RPM，缺 concurrency_limit 时按 0（不限）。
  limit_mode?: ActivityLimitMode
  rpm_limit: number
  concurrency_limit?: number
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
  // 活动背景图 URL；空串表示没有背景图。
  image_url?: string
  // 活动结束时间（unix 秒），0 表示不限。
  active_end_time?: number
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
