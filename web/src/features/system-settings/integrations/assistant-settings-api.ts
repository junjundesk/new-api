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

export type AssistantProtocol = 'responses' | 'chat'

export type AssistantReasoningEffort = 'high' | 'xhigh' | 'max'

/** Missing or legacy values fall back to the project default, high. */
export function normalizeAssistantReasoningEffort(
  value: unknown
): AssistantReasoningEffort {
  if (value === 'xhigh' || value === 'max') return value
  return 'high'
}

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

export type AssistantProfileRequest = {
  profile: AssistantProfileName
  source?: AssistantSource
  protocol?: AssistantProtocol
  reasoning_effort?: AssistantReasoningEffort
  base_url?: string
  api_key?: string
  model?: string
  channel_id?: number
  channel_model?: string
}

export type AssistantTestRequest = AssistantProfileRequest

export type AssistantModelsRequest = AssistantProfileRequest & {
  context_window?: number
  max_output_tokens?: number
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

export type AssistantModelsResponse = {
  success: boolean
  message: string
  data?: {
    models: string[]
  }
}

export type AssistantClearKeyResponse = {
  success: boolean
  message: string
}

const TOKEN_AMOUNT_PATTERN = /^(\d+(?:\.\d+)?)\s*([kmb万亿])?$/i

const TOKEN_AMOUNT_MULTIPLIERS: Record<string, number> = {
  k: 1000,
  m: 1000000,
  b: 1000000000,
  万: 10000,
  亿: 100000000,
}

/**
 * Parses a human-readable token amount ("128k", "1M", "1亿", "1000000") into
 * an integer. Returns null when the input is not a supported token count.
 */
export function parseTokenAmount(raw: string): number | null {
  const match = TOKEN_AMOUNT_PATTERN.exec(raw.trim())
  if (!match) return null
  const base = Number(match[1])
  if (!Number.isFinite(base)) return null
  const suffix = match[2]?.toLowerCase() ?? ''
  return Math.round(base * (TOKEN_AMOUNT_MULTIPLIERS[suffix] ?? 1))
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
 * Fetches the upstream model list for one profile. The same unsaved form values
 * as the connection test are used, so the model picker works before saving.
 */
export async function listAssistantModels(request: AssistantModelsRequest) {
  const res = await api.post<AssistantModelsResponse>(
    '/api/assistant/admin/models',
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
