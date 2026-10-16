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
  ActivityCode,
  ActivityGrant,
  ActivityCodeFormData,
  ApiResponse,
  GetActivityCodesParams,
  GroupModelMember,
  PagedActivityCodes,
  PagedActivityGrants,
  SearchActivityCodesParams,
} from './types'

// ============================================================================
// Activity Code Management (admin)
// ============================================================================

export async function getActivityCodes(
  params: GetActivityCodesParams = {}
): Promise<ApiResponse<PagedActivityCodes>> {
  const { p = 1, page_size = 10 } = params
  const res = await api.get(`/api/activity/?p=${p}&page_size=${page_size}`)
  return res.data
}

export async function searchActivityCodes(
  params: SearchActivityCodesParams
): Promise<ApiResponse<PagedActivityCodes>> {
  const { keyword = '', status = '', p = 1, page_size = 10 } = params
  const queryParams = new URLSearchParams()
  queryParams.set('keyword', keyword)
  if (status) queryParams.set('status', status)
  queryParams.set('p', String(p))
  queryParams.set('page_size', String(page_size))
  const res = await api.get(`/api/activity/search?${queryParams.toString()}`)
  return res.data
}

export async function getActivityCode(
  id: number
): Promise<ApiResponse<ActivityCode>> {
  const res = await api.get(`/api/activity/${id}`)
  return res.data
}

export async function createActivityCode(
  data: ActivityCodeFormData
): Promise<ApiResponse<ActivityCode>> {
  const res = await api.post('/api/activity/', data)
  return res.data
}

export async function updateActivityCode(
  data: ActivityCodeFormData & { id: number }
): Promise<ApiResponse<ActivityCode>> {
  const res = await api.put('/api/activity/', data)
  return res.data
}

export async function updateActivityCodeStatus(
  id: number,
  status: number
): Promise<ApiResponse> {
  const res = await api.put(`/api/activity/${id}/status`, { status })
  return res.data
}

export async function deleteActivityCode(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/activity/${id}`)
  return res.data
}

export async function getActivityCodeGrants(
  id: number,
  params: GetActivityCodesParams = {}
): Promise<ApiResponse<PagedActivityGrants>> {
  const { p = 1, page_size = 20 } = params
  const res = await api.get(
    `/api/activity/${id}/grants?p=${p}&page_size=${page_size}`
  )
  return res.data
}

export async function revokeActivityGrant(
  grantId: number,
  remark = ''
): Promise<ApiResponse> {
  const res = await api.post(`/api/activity/grants/${grantId}/revoke`, {
    remark,
  })
  return res.data
}

// 活动码的目标分组必须已在「分组倍率」中配置，这里复用管理端分组接口。
export async function getGroupNames(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group/')
  return res.data
}

// 分组内在架模型，作为「指定模型」类型的模型建议值。
export async function getGroupModels(
  group: string
): Promise<ApiResponse<GroupModelMember[]>> {
  const res = await api.get(
    `/api/models/group_members?group=${encodeURIComponent(group)}`
  )
  return res.data
}

export type { ActivityGrant }
