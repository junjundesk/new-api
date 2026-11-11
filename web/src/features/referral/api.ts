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

import type {
  ReferralInviteesResponse,
  ReferralInvitersResponse,
} from './types'

// ============================================================================
// Referral API
// ============================================================================

/**
 * One page of the users the caller invited (usernames are masked)
 */
export async function getMyInvitees(
  page: number,
  pageSize: number
): Promise<ReferralInviteesResponse> {
  const res = await api.get(
    `/api/user/referral/invitees?page=${page}&page_size=${pageSize}`
  )
  return res.data
}

/**
 * Admin: one page of inviters with their invite counts and site-wide totals
 */
export async function getInviters(
  keyword: string,
  page: number,
  pageSize: number
): Promise<ReferralInvitersResponse> {
  const res = await api.get('/api/user/inviter/list', {
    params: { keyword, page, page_size: pageSize },
  })
  return res.data
}

/**
 * Admin: one page of the users invited by the given inviter
 */
export async function getInviterInvitees(
  inviterId: number,
  page: number,
  pageSize: number
): Promise<ReferralInviteesResponse> {
  const res = await api.get(
    `/api/user/${inviterId}/invitees?page=${page}&page_size=${pageSize}`
  )
  return res.data
}
