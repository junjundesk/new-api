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
import type { ApiResponse } from '@/features/wallet/types'

/**
 * One user invited by an inviter. The username is masked by the server on the
 * user-facing endpoint and complete on the admin one.
 */
export interface ReferralInvitee {
  id: number
  username: string
  /** Registration time, Unix seconds */
  created_at: number
  /** Whether this invitee counted toward the inviter's commission tier */
  valid: boolean
  /** Commission this invitee generated for the inviter, in quota */
  reward_quota: number
}

/**
 * One row of the admin inviter overview
 */
export interface ReferralInviter {
  id: number
  username: string
  display_name: string
  /** Users currently bound to this inviter */
  registered_count: number
  /** Invitees that counted toward the commission tier */
  valid_invitees: number
  /** Lifetime commission paid out, in quota */
  history_quota: number
}

/**
 * Site-wide invite totals shown above the admin inviter list
 */
export interface ReferralOverview {
  inviters: number
  invitees: number
  valid_invitees: number
  reward_quota: number
}

export interface ReferralPage<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface ReferralInviterPage extends ReferralPage<ReferralInviter> {
  overview: ReferralOverview
}

export type ReferralInviteesResponse = ApiResponse<
  ReferralPage<ReferralInvitee>
>
export type ReferralInvitersResponse = ApiResponse<ReferralInviterPage>
