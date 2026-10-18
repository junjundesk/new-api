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
import type { TFunction } from 'i18next'

import { ASSISTANT_ERROR_KEYS } from '../constants'
import type {
  AssistantConfirmationResolutionStatus,
  AssistantStreamError,
  AssistantStreamEvent,
  AssistantUsage,
} from '../types'

const STREAM_CLOSED_READY_STATE = 2

function readRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function readString(value: unknown): string | undefined {
  if (typeof value !== 'string' || value === '') return undefined
  return value
}

function readNumber(value: unknown): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) return undefined
  return value
}

function readConfirmationResolutionStatus(
  value: unknown
): AssistantConfirmationResolutionStatus | undefined {
  if (value === 'approved' || value === 'denied' || value === 'expired') {
    return value
  }
  return undefined
}

function parseUsage(value: unknown): AssistantUsage | undefined {
  const record = readRecord(value)
  if (!record) return undefined
  return {
    input_tokens: readNumber(record.input_tokens),
    output_tokens: readNumber(record.output_tokens),
    total_tokens: readNumber(record.total_tokens),
  }
}

export class AssistantStreamParseError extends Error {
  constructor() {
    super('Invalid assistant stream payload')
    this.name = 'AssistantStreamParseError'
  }
}

/**
 * Parse one SSE data payload. Returns null for payloads that are intentionally
 * ignored (empty keep-alives, [DONE], unknown forward-compatible event types)
 * and throws AssistantStreamParseError when the payload is malformed JSON.
 */
export function parseAssistantStreamEvent(
  data: string
): AssistantStreamEvent | null {
  const trimmed = data.trim()
  if (trimmed === '' || trimmed === '[DONE]') return null

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    throw new AssistantStreamParseError()
  }

  const record = readRecord(parsed)
  if (!record) throw new AssistantStreamParseError()

  switch (record.type) {
    case 'meta':
      return { type: 'meta', model: readString(record.model) }
    case 'delta': {
      const text = readString(record.text)
      if (text === undefined) return null
      return { type: 'delta', text }
    }
    case 'tool_start': {
      const toolCallId = readString(record.tool_call_id)
      const name = readString(record.name)
      if (!toolCallId || !name) return null
      return {
        type: 'tool_start',
        toolCallId,
        name,
        arguments: record.arguments,
      }
    }
    case 'tool_result': {
      const toolCallId = readString(record.tool_call_id)
      const name = readString(record.name)
      if (!toolCallId || !name) return null
      return {
        type: 'tool_result',
        toolCallId,
        name,
        ok: record.ok !== false,
        result: record.result,
        error: readString(record.error),
      }
    }
    case 'confirmation_required': {
      const confirmationId = readString(record.confirmation_id)
      if (!confirmationId) return null
      return {
        type: 'confirmation_required',
        confirmationId,
        operationId: readString(record.operation_id) ?? '',
        risk: readString(record.risk),
        summary: readString(record.summary),
        params: record.params,
        expiresAt: readNumber(record.expires_at),
      }
    }
    case 'confirmation_resolved': {
      const confirmationId = readString(record.confirmation_id)
      const status = readConfirmationResolutionStatus(record.status)
      if (!confirmationId || !status) return null
      return { type: 'confirmation_resolved', confirmationId, status }
    }
    case 'error':
      return {
        type: 'error',
        code: readString(record.code),
        message: readString(record.message) ?? readString(record.error),
        retryAfter: readNumber(record.retry_after),
      }
    case 'done':
      return { type: 'done', usage: parseUsage(record.usage) }
    default:
      return null
  }
}

export function parseAssistantStreamError(data?: string): AssistantStreamError {
  const fallback: AssistantStreamError = {
    fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
  }
  if (!data) return fallback

  let parsed: unknown
  try {
    parsed = JSON.parse(data)
  } catch {
    return fallback
  }

  const record = readRecord(parsed)
  if (!record) return fallback

  const nested = readRecord(record.error)
  const message =
    readString(record.message) ??
    readString(record.error) ??
    (nested ? readString(nested.message) : undefined)
  const code =
    readString(record.code) ?? (nested ? readString(nested.code) : undefined)
  return {
    code,
    message,
    retryAfter: readNumber(record.retry_after),
    fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
  }
}

export function isAssistantStreamClosed(readyState?: number): boolean {
  return readyState !== undefined && readyState >= STREAM_CLOSED_READY_STATE
}

export function getAssistantStreamStatusError(
  status?: number
): AssistantStreamError | null {
  if (status === undefined || status === 0 || status === 200) return null
  return {
    message: `HTTP ${status}`,
    fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
  }
}

export function resolveAssistantStreamError(
  error: AssistantStreamError,
  t: TFunction
): string {
  const message = error.message?.trim()
  if (message) return message
  return t(error.fallbackKey)
}
