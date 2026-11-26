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
  Redemption,
  RedemptionBonusRule,
  RedemptionCategory,
  RedemptionMigrationMode,
  RedemptionMigrationResult,
  ApiResponse,
  GetRedemptionsParams,
  GetRedemptionsResponse,
  SearchRedemptionsParams,
  RedemptionFormData,
} from './types'

// ============================================================================
// Redemption Code Management
// ============================================================================

// Get paginated redemption codes list
export async function getRedemptions(
  params: GetRedemptionsParams = {}
): Promise<GetRedemptionsResponse> {
  const { p = 1, page_size = 10 } = params
  const res = await api.get(`/api/redemption/?p=${p}&page_size=${page_size}`)
  return res.data
}

// Search redemption codes by keyword
export async function searchRedemptions(
  params: SearchRedemptionsParams
): Promise<GetRedemptionsResponse> {
  const { keyword = '', status = '', p = 1, page_size = 10 } = params
  const queryParams = new URLSearchParams()
  queryParams.set('keyword', keyword)
  if (params.category_id !== undefined) {
    queryParams.set('category_id', String(params.category_id))
  }
  if (status) queryParams.set('status', status)
  queryParams.set('p', String(p))
  queryParams.set('page_size', String(page_size))
  const res = await api.get(`/api/redemption/search?${queryParams.toString()}`)
  return res.data
}

export async function getRedemptionCategories(): Promise<RedemptionCategory[]> {
  const res = await api.get<ApiResponse<RedemptionCategory[]>>(
    '/api/redemption/categories'
  )
  if (!res.data.success || !res.data.data) throw new Error(res.data.message)
  return res.data.data
}

export async function saveRedemptionCategory(
  category: Omit<RedemptionCategory, 'id'> & { id?: number }
) {
  const url = '/api/redemption/categories'
  const config = { skipBusinessError: true }
  const res =
    category.id === undefined
      ? await api.post<ApiResponse<RedemptionCategory>>(url, category, config)
      : await api.put<ApiResponse<RedemptionCategory>>(
          `${url}/${category.id}`,
          category,
          config
        )
  if (!res.data.success || !res.data.data) throw new Error(res.data.message)
  return res.data.data
}

export async function deleteRedemptionCategory(id: number) {
  const res = await api.delete<ApiResponse>(
    `/api/redemption/categories/${id}`,
    { skipBusinessError: true }
  )
  if (!res.data.success) throw new Error(res.data.message)
}

export async function getRedemptionBonusRules(): Promise<
  RedemptionBonusRule[]
> {
  const res = await api.get<ApiResponse<RedemptionBonusRule[]>>(
    '/api/redemption/bonus-rules'
  )
  if (!res.data.success || !res.data.data) throw new Error(res.data.message)
  return res.data.data
}

export async function saveRedemptionBonusRules(rules: RedemptionBonusRule[]) {
  const res = await api.put<ApiResponse<RedemptionBonusRule[]>>(
    '/api/redemption/bonus-rules',
    { rules },
    { skipBusinessError: true }
  )
  if (!res.data.success || !res.data.data) throw new Error(res.data.message)
  return res.data.data
}

export async function migrateUncategorizedRedemptions(
  mode: RedemptionMigrationMode
): Promise<RedemptionMigrationResult> {
  const res = await api.post<ApiResponse<RedemptionMigrationResult>>(
    '/api/redemption/migrate-uncategorized',
    { mode },
    { skipBusinessError: true }
  )
  if (!res.data.success || !res.data.data) throw new Error(res.data.message)
  return res.data.data
}

// Get single redemption code by ID
export async function getRedemption(
  id: number
): Promise<ApiResponse<Redemption>> {
  const res = await api.get(`/api/redemption/${id}`)
  return res.data
}

// Create redemption code(s)
export async function createRedemption(
  data: RedemptionFormData
): Promise<ApiResponse<string[]>> {
  const res = await api.post('/api/redemption/', data)
  return res.data
}

// Update redemption code
export async function updateRedemption(
  data: RedemptionFormData & { id: number }
): Promise<ApiResponse<Redemption>> {
  const res = await api.put('/api/redemption/', data)
  return res.data
}

// Update redemption code status (enable/disable)
export async function updateRedemptionStatus(
  id: number,
  status: number
): Promise<ApiResponse<Redemption>> {
  const res = await api.put('/api/redemption/?status_only=true', { id, status })
  return res.data
}

// Delete a single redemption code
export async function deleteRedemption(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/redemption/${id}/`)
  return res.data
}

// Delete invalid redemption codes (used, disabled, expired)
export async function deleteInvalidRedemptions(
  categoryId?: number
): Promise<ApiResponse<number>> {
  const res = await api.delete('/api/redemption/invalid', {
    params: { category_id: categoryId },
  })
  return res.data
}
