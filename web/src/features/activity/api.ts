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
  ActivityGrantsResponse,
  ActivityOffersResponse,
  ActivityPreview,
  ApiResponse,
  RedeemActivityResult,
} from './types'

// ============================================================================
// Activity API (user side)
// ============================================================================

export async function getActivityOffers(): Promise<
  ApiResponse<ActivityOffersResponse>
> {
  const res = await api.get('/api/user/activity/offers')
  return res.data
}

export async function getActivityGrants(): Promise<
  ApiResponse<ActivityGrantsResponse>
> {
  const res = await api.get('/api/user/activity/grants')
  return res.data
}

export async function verifyActivityCode(
  code: string
): Promise<ApiResponse<ActivityPreview>> {
  const res = await api.post('/api/user/activity/verify', { code })
  return res.data
}

export async function redeemActivityCode(input: {
  code: string
  confirm: boolean
  expectedPriceQuota: number
}): Promise<ApiResponse<RedeemActivityResult>> {
  const res = await api.post('/api/user/activity/redeem', {
    code: input.code,
    confirm: input.confirm,
    expected_price_quota: input.expectedPriceQuota,
  })
  return res.data
}
