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
// ============================================================================
// Profiles
// ============================================================================

/**
 * The assistant runs two fully independent profiles. The user profile powers
 * /assistant, while the admin profile powers the root-only /assistant/admin
 * page with its own upstream API/key configuration.
 */
export type AssistantProfile = 'user' | 'admin'

// ============================================================================
// Chat message model
// ============================================================================

export type AssistantRole = 'user' | 'assistant'

export type AssistantMessageStatus = 'streaming' | 'done' | 'error' | 'stopped'

export type AssistantToolPartStatus = 'running' | 'done' | 'error'

export type AssistantConfirmationStatus =
  | 'pending'
  | 'approved'
  | 'denied'
  | 'expired'

export type AssistantConfirmationResolutionStatus = Exclude<
  AssistantConfirmationStatus,
  'pending'
>

export interface AssistantTextPart {
  id: string
  type: 'text'
  text: string
}

export interface AssistantToolPart {
  id: string
  type: 'tool'
  toolCallId: string
  name: string
  arguments?: unknown
  status: AssistantToolPartStatus
  result?: unknown
  error?: string
}

/**
 * A write/destructive operation the model wants to run. The backend emits the
 * confirmation request over SSE and blocks the run until the user approves or
 * denies it (or it expires).
 */
export interface AssistantConfirmationPart {
  id: string
  type: 'confirmation'
  confirmationId: string
  operationId: string
  risk: string
  summary: string
  /** Server-side redacted path/query/body arguments. */
  params?: unknown
  /** Unix timestamp (seconds) after which the confirmation expires. */
  expiresAt?: number
  status: AssistantConfirmationStatus
}

export type AssistantMessagePart =
  | AssistantTextPart
  | AssistantToolPart
  | AssistantConfirmationPart

export interface AssistantChatMessage {
  id: string
  role: AssistantRole
  parts: AssistantMessagePart[]
  status: AssistantMessageStatus
  createdAt: number
  error?: string
  /** Latest real upstream prompt token count for this conversation. */
  contextTokens?: number
}

// ============================================================================
// Configuration
// ============================================================================

export interface AssistantConfig {
  enabled: boolean
  model?: string
  canCreateTokens: boolean
  maxMessageChars: number
  contextWindow?: number
}

// ============================================================================
// Request / response payloads
// ============================================================================

export interface AssistantChatTurn {
  role: AssistantRole
  content: string
}

export interface AssistantChatRequest {
  message: string
  history: AssistantChatTurn[]
}

export interface AssistantUsage {
  input_tokens?: number
  output_tokens?: number
  total_tokens?: number
}

// ============================================================================
// SSE events (server contract)
// ============================================================================

export type AssistantStreamEvent =
  | { type: 'meta'; model?: string }
  | { type: 'delta'; text: string }
  | {
      type: 'tool_start'
      toolCallId: string
      name: string
      arguments?: unknown
    }
  | {
      type: 'tool_result'
      toolCallId: string
      name: string
      ok: boolean
      result?: unknown
      error?: string
    }
  | {
      type: 'confirmation_required'
      confirmationId: string
      operationId: string
      risk?: string
      summary?: string
      params?: unknown
      expiresAt?: number
    }
  | {
      type: 'confirmation_resolved'
      confirmationId: string
      status: AssistantConfirmationResolutionStatus
    }
  | { type: 'error'; code?: string; message?: string; retryAfter?: number }
  | {
      type: 'done'
      usage?: AssistantUsage
      context_tokens?: number
      context_window?: number
    }

export interface AssistantStreamError {
  code?: string
  /** Server-provided message, safe to display as-is when present. */
  message?: string
  /** i18n key used when the server provided no message. */
  fallbackKey: string
  retryAfter?: number
}

// ============================================================================
// Tool result data
// ============================================================================

export interface AssistantCreateApiKeyData {
  id?: number
  name: string
  maskedKey: string
  statusText?: string
  unlimitedQuota: boolean
  expiredTime?: number
  neverExpires: boolean
  group?: string
}
