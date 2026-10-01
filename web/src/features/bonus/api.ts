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
import { api } from '@/lib/api'

// ============================================================================
// Bonus (赠金) API
// ============================================================================

export interface BonusGrant {
  id: number
  user_id: number
  amount_total: number
  amount_used: number
  expire_time: number
  status: string
  source: string
  remark: string
  created_at: number
  updated_at: number
}

export interface BonusSummary {
  bonus_remaining: number
  /** Remaining bonus that expires. */
  bonus_expiring_remaining: number
  /** Remaining bonus that never expires. */
  bonus_permanent_remaining: number
  nearest_expire_time: number
  grants: BonusGrant[]
}

export interface BonusGrantPayload {
  amount: number
  /** Duration string: bare number = hours ("1" = 1h); "1天" = 24h. Empty = never expires. */
  expire_input?: string
  remark?: string
  /** Send the arrival-notification email. Defaults to true server-side. */
  send_email?: boolean
}

export interface BonusGrantAllPayload extends BonusGrantPayload {
  all_users: true
}

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

/** Fetch the caller's bonus summary. */
export async function getSelfBonus(): Promise<ApiResponse<BonusSummary>> {
  const res = await api.get('/api/user/self/bonus')
  return res.data
}

/** Grant bonus to a single user (admin). */
export async function grantUserBonus(
  userId: number,
  payload: BonusGrantPayload
): Promise<ApiResponse<{ grant_id: number; expire_time: number }>> {
  const res = await api.post(`/api/user/${userId}/bonus`, payload)
  return res.data
}

/** Grant bonus to all users (admin). */
export async function grantAllUsersBonus(
  payload: BonusGrantAllPayload
): Promise<ApiResponse<{ granted: number; total: number }>> {
  const res = await api.post('/api/user/bonus/grant_all', payload)
  return res.data
}

/** List a user's bonus grants (admin). */
export async function listUserBonus(
  userId: number
): Promise<ApiResponse<BonusSummary>> {
  const res = await api.get(`/api/user/${userId}/bonus`)
  return res.data
}
