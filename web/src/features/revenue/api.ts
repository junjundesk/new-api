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

import type { CostSettings } from './schema'

export type RevenueRange = 'today' | 'yesterday' | '7d' | '30d' | 'all'
// Decimal quota units arrive as strings. Only convert for the site's currency formatter.
export interface UsageAmounts {
  total: string
  cost: string
  profit: string
  unpriced: string
}
export interface RevenueTotals extends UsageAmounts {
  online: string
  admin: string
  redeemed: string
  credited: string
}
export interface RevenueItem extends UsageAmounts {
  group: string
  model: string
  cost_ratio: string | null
}
export interface RevenueReport {
  as_of: number
  summary: RevenueTotals
  days: (RevenueTotals & { date: string })[]
  items: RevenueItem[]
}
interface ResponseData<T> {
  success: boolean
  message: string
  data: T
}
function result<T>(response: ResponseData<T>): T {
  if (!response.success) throw new Error(response.message)
  return response.data
}
export async function getRevenueReport(
  range: RevenueRange
): Promise<RevenueReport> {
  return result(
    (
      await api.get<ResponseData<RevenueReport>>('/api/revenue/report', {
        params: { range },
      })
    ).data
  )
}
export async function getCostGroups() {
  return result(
    (
      await api.get<
        ResponseData<CostSettings & { available_groups: string[] }>
      >('/api/revenue/groups')
    ).data
  )
}
export async function saveCostGroups(settings: CostSettings) {
  return result(
    (await api.put<ResponseData<CostSettings>>('/api/revenue/groups', settings))
      .data
  )
}
