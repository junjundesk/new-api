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

export type AssistantProfileName = 'user' | 'admin'

export type AssistantSource = 'direct' | 'channel'

export type AssistantChannel = {
  id: number
  name: string
  type: number
  type_name: string
  status: number
  base_url: string
  models: string[]
}

export type AssistantChannelsResponse = {
  success: boolean
  message: string
  data?: AssistantChannel[]
}

export type AssistantTestRequest = {
  profile: AssistantProfileName
  source?: AssistantSource
  base_url?: string
  api_key?: string
  model?: string
  channel_id?: number
  channel_model?: string
}

export type AssistantTestData = {
  latency_ms: number
  model: string
  text: string
}

export type AssistantTestResponse = {
  success: boolean
  message: string
  data?: AssistantTestData
}

export type AssistantClearKeyResponse = {
  success: boolean
  message: string
}

/**
 * Lists every channel the assistant can bind, including disabled channels:
 * the backend uses the channel key and ignores channel status, so a disabled
 * channel is still a valid assistant upstream.
 */
export async function listAssistantChannels() {
  const res = await api.get<AssistantChannelsResponse>(
    '/api/assistant/admin/channels'
  )
  return res.data
}

/**
 * Tests one assistant profile with values from the form, including unsaved
 * ones. The outcome is rendered inline by the settings section, so the request
 * skips the global toast handlers.
 */
export async function testAssistantConnection(request: AssistantTestRequest) {
  const res = await api.post<AssistantTestResponse>(
    '/api/assistant/admin/test',
    request,
    { skipErrorHandler: true, skipBusinessError: true }
  )
  return res.data
}

/**
 * Clears one profile's stored API key. The backend also disables the profile
 * because it can no longer reach the upstream model on its own.
 */
export async function clearAssistantApiKey(profile: AssistantProfileName) {
  const res = await api.delete<AssistantClearKeyResponse>(
    '/api/assistant/admin/key',
    {
      params: { profile },
      skipErrorHandler: true,
      skipBusinessError: true,
    }
  )
  return res.data
}
