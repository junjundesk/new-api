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
import type {
  AssistantMessageStatus,
  AssistantProfile,
  AssistantToolPartStatus,
} from './types'

export const ASSISTANT_API_ENDPOINTS = {
  user: {
    CONFIG: '/api/assistant/config',
    CHAT: '/api/assistant/chat',
  },
  admin: {
    CONFIG: '/api/assistant/admin/runtime-config',
    CHAT: '/api/assistant/admin/chat',
  },
} as const satisfies Record<AssistantProfile, { CONFIG: string; CHAT: string }>

export const ASSISTANT_LIMITS = {
  MAX_MESSAGE_CHARS: 100000,
  MAX_HISTORY_MESSAGES: 100,
  MAX_HISTORY_CHARS: 400000,
  MAX_STORED_MESSAGES: 100,
} as const

export const ASSISTANT_STORAGE_KEY_PREFIX = 'assistant_chat_messages'

export const ASSISTANT_MESSAGE_STATUS = {
  STREAMING: 'streaming',
  DONE: 'done',
  ERROR: 'error',
  STOPPED: 'stopped',
} as const satisfies Record<string, AssistantMessageStatus>

export const ASSISTANT_TOOL_STATUS = {
  RUNNING: 'running',
  DONE: 'done',
  ERROR: 'error',
} as const satisfies Record<string, AssistantToolPartStatus>

// i18n keys: render through t(ASSISTANT_ERROR_KEYS.xxx)
export const ASSISTANT_ERROR_KEYS = {
  CONFIG_LOAD_FAILED: 'Failed to load the AI assistant configuration',
  REQUEST_FAILED: 'The assistant request failed',
  STREAM_START_FAILED: 'Failed to start the assistant request',
  STREAM_PARSE_FAILED: 'Failed to read the assistant response',
  CONNECTION_CLOSED: 'The connection closed before the assistant finished',
  REVEAL_KEY_FAILED: 'Failed to load the full API key',
} as const
