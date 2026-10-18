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

import { ASSISTANT_API_ENDPOINTS, ASSISTANT_LIMITS } from './constants'
import type { AssistantConfig, AssistantProfile } from './types'

function readRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function readPositiveInt(
  value: unknown,
  fallback: number,
  cap: number
): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return fallback
  const rounded = Math.floor(value)
  if (rounded < 1) return fallback
  return Math.min(rounded, cap)
}

export function normalizeAssistantConfig(payload: unknown): AssistantConfig {
  const envelope = readRecord(payload)
  const data = envelope ? readRecord(envelope.data) : null
  const source = data ?? envelope
  if (!source) {
    return {
      enabled: false,
      canCreateTokens: false,
      maxMessageChars: ASSISTANT_LIMITS.MAX_MESSAGE_CHARS,
    }
  }
  const model = typeof source.model === 'string' ? source.model.trim() : ''
  return {
    enabled: source.enabled === true,
    model: model === '' ? undefined : model,
    canCreateTokens: source.can_create_tokens === true,
    maxMessageChars: readPositiveInt(
      source.max_message_chars,
      ASSISTANT_LIMITS.MAX_MESSAGE_CHARS,
      ASSISTANT_LIMITS.MAX_MESSAGE_CHARS
    ),
  }
}

export async function getAssistantConfig(
  profile: AssistantProfile = 'user'
): Promise<AssistantConfig> {
  const res = await api.get(ASSISTANT_API_ENDPOINTS[profile].CONFIG)
  return normalizeAssistantConfig(res.data)
}

/**
 * Send the user's decision for one pending write confirmation. The pending
 * operation is resolved by the backend and the SSE stream continues with the
 * matching confirmation_resolved event.
 */
export async function confirmAssistantOperation(
  profile: AssistantProfile,
  confirmationId: string,
  approve: boolean
): Promise<void> {
  const res = await api.post(ASSISTANT_API_ENDPOINTS[profile].CONFIRM, {
    confirmation_id: confirmationId,
    approve,
  })
  const envelope = readRecord(res.data)
  if (envelope?.success === false) {
    throw new Error(
      typeof envelope.message === 'string' ? envelope.message : ''
    )
  }
}
