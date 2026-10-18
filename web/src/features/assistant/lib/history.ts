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
import {
  ASSISTANT_LIMITS,
  ASSISTANT_MESSAGE_STATUS,
  ASSISTANT_STORAGE_KEY_PREFIX,
} from '../constants'
import type {
  AssistantChatMessage,
  AssistantChatTurn,
  AssistantMessagePart,
  AssistantMessageStatus,
  AssistantProfile,
  AssistantRole,
} from '../types'
import { getMessageText } from './messages'

export interface AssistantHistoryLimits {
  maxMessages: number
  maxChars: number
}

const DEFAULT_HISTORY_LIMITS: AssistantHistoryLimits = {
  maxMessages: ASSISTANT_LIMITS.MAX_HISTORY_MESSAGES,
  maxChars: ASSISTANT_LIMITS.MAX_HISTORY_CHARS,
}

/**
 * Build the history sent to POST /api/assistant/chat.
 *
 * Only completed, non-empty text messages are included; streaming, stopped and
 * errored messages are never sent back. The suffix is kept within both the
 * message-count and character budgets.
 */
export function buildAssistantRequestHistory(
  messages: readonly AssistantChatMessage[],
  limits: AssistantHistoryLimits = DEFAULT_HISTORY_LIMITS
): AssistantChatTurn[] {
  const maxMessages = Math.max(0, limits.maxMessages)
  const turns: AssistantChatTurn[] = []
  let remainingChars = Math.max(0, limits.maxChars)

  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (turns.length >= maxMessages || remainingChars <= 0) break
    const message = messages[index]
    if (message.status !== ASSISTANT_MESSAGE_STATUS.DONE) continue
    const content = getMessageText(message).trim()
    if (content === '') continue
    const clipped =
      content.length > remainingChars
        ? content.slice(content.length - remainingChars)
        : content
    turns.push({ role: message.role, content: clipped })
    remainingChars -= clipped.length
  }

  return turns.reverse()
}

/**
 * Build the sessionStorage key for one assistant conversation. The profile is
 * part of the key so the user and admin conversations never share history.
 */
export function getAssistantStorageKey(
  userId?: number,
  profile: AssistantProfile = 'user'
): string {
  const scope = `${ASSISTANT_STORAGE_KEY_PREFIX}:${profile}`
  return userId === undefined ? scope : `${scope}:${userId}`
}

function readRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function isAssistantRole(value: unknown): value is AssistantRole {
  return value === 'user' || value === 'assistant'
}

function isMessageStatus(value: unknown): value is AssistantMessageStatus {
  return (
    value === 'streaming' ||
    value === 'done' ||
    value === 'error' ||
    value === 'stopped'
  )
}

function isToolPart(value: Record<string, unknown>): boolean {
  return (
    typeof value.toolCallId === 'string' &&
    typeof value.name === 'string' &&
    (value.status === 'running' ||
      value.status === 'done' ||
      value.status === 'error')
  )
}

function isMessagePart(value: unknown): value is AssistantMessagePart {
  const record = readRecord(value)
  if (!record || typeof record.id !== 'string') return false
  if (record.type === 'text') return typeof record.text === 'string'
  if (record.type === 'tool') return isToolPart(record)
  return false
}

function isStoredMessage(value: unknown): value is AssistantChatMessage {
  const record = readRecord(value)
  if (!record) return false
  if (typeof record.id !== 'string') return false
  if (!isAssistantRole(record.role)) return false
  if (!isMessageStatus(record.status)) return false
  if (typeof record.createdAt !== 'number') return false
  if (!Array.isArray(record.parts)) return false
  if (!record.parts.every(isMessagePart)) return false
  return record.error === undefined || typeof record.error === 'string'
}

/**
 * Load the current user's chat history from sessionStorage. Corrupt or
 * unexpected data is ignored instead of crashing the page.
 */
export function loadStoredMessages(storageKey: string): AssistantChatMessage[] {
  if (storageKey === '' || typeof window === 'undefined') return []
  try {
    const raw = window.sessionStorage.getItem(storageKey)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    const messages = parsed.filter(isStoredMessage)
    return messages
      .filter((message) => message.status !== 'streaming')
      .slice(-ASSISTANT_LIMITS.MAX_STORED_MESSAGES)
  } catch {
    return []
  }
}

/**
 * Persist only settled messages. Streaming messages are intentionally skipped
 * so a mid-stream refresh never restores a half-written response, and the full
 * API key never enters this payload because it is never added to message state.
 */
export function saveStoredMessages(
  storageKey: string,
  messages: readonly AssistantChatMessage[]
): void {
  if (storageKey === '' || typeof window === 'undefined') return
  const settled = messages
    .filter((message) => message.status !== 'streaming')
    .slice(-ASSISTANT_LIMITS.MAX_STORED_MESSAGES)
  try {
    window.sessionStorage.setItem(storageKey, JSON.stringify(settled))
  } catch {
    /* Storage can be unavailable or full; chat keeps working in memory. */
  }
}

export function clearStoredMessages(storageKey: string): void {
  if (storageKey === '' || typeof window === 'undefined') return
  try {
    window.sessionStorage.removeItem(storageKey)
  } catch {
    /* Storage can be unavailable; clearing in-memory state is enough. */
  }
}
