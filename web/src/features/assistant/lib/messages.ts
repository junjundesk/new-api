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
import { ASSISTANT_MESSAGE_STATUS } from '../constants'
import type {
  AssistantChatMessage,
  AssistantMessagePart,
  AssistantMessageStatus,
  AssistantStreamEvent,
  AssistantTextPart,
  AssistantToolPart,
} from '../types'

let sequence = 0

function nextId(prefix: string): string {
  sequence += 1
  return `${prefix}-${Date.now().toString(36)}-${sequence.toString(36)}`
}

export function createAssistantMessageId(): string {
  return nextId('msg')
}

function createTextPart(text: string): AssistantTextPart {
  return { id: nextId('text'), type: 'text', text }
}

export function createUserMessage(text: string): AssistantChatMessage {
  return {
    id: createAssistantMessageId(),
    role: 'user',
    parts: [createTextPart(text)],
    status: ASSISTANT_MESSAGE_STATUS.DONE,
    createdAt: Date.now(),
  }
}

export function createAssistantMessage(): AssistantChatMessage {
  return {
    id: createAssistantMessageId(),
    role: 'assistant',
    parts: [],
    status: ASSISTANT_MESSAGE_STATUS.STREAMING,
    createdAt: Date.now(),
  }
}

export function appendAssistantText(
  message: AssistantChatMessage,
  text: string
): AssistantChatMessage {
  if (text === '') return message
  const lastPart = message.parts.at(-1)
  if (lastPart && lastPart.type === 'text') {
    const parts = [...message.parts]
    parts[parts.length - 1] = { ...lastPart, text: lastPart.text + text }
    return { ...message, parts }
  }
  return { ...message, parts: [...message.parts, createTextPart(text)] }
}

export function addAssistantToolStart(
  message: AssistantChatMessage,
  event: Extract<AssistantStreamEvent, { type: 'tool_start' }>
): AssistantChatMessage {
  const part: AssistantToolPart = {
    id: event.toolCallId,
    type: 'tool',
    toolCallId: event.toolCallId,
    name: event.name,
    arguments: event.arguments,
    status: 'running',
  }
  const existingIndex = message.parts.findIndex(
    (item) => item.type === 'tool' && item.toolCallId === event.toolCallId
  )
  if (existingIndex === -1) {
    return { ...message, parts: [...message.parts, part] }
  }
  const parts = [...message.parts]
  parts[existingIndex] = part
  return { ...message, parts }
}

export function addAssistantToolResult(
  message: AssistantChatMessage,
  event: Extract<AssistantStreamEvent, { type: 'tool_result' }>
): AssistantChatMessage {
  const existingIndex = message.parts.findIndex(
    (item) => item.type === 'tool' && item.toolCallId === event.toolCallId
  )
  if (existingIndex === -1) {
    const part: AssistantToolPart = {
      id: event.toolCallId,
      type: 'tool',
      toolCallId: event.toolCallId,
      name: event.name,
      status: event.ok ? 'done' : 'error',
      result: event.result,
      error: event.error,
    }
    return { ...message, parts: [...message.parts, part] }
  }
  const parts = message.parts.map((item) => {
    if (item.type !== 'tool' || item.toolCallId !== event.toolCallId) {
      return item
    }
    return {
      ...item,
      name: event.name,
      status: event.ok ? ('done' as const) : ('error' as const),
      result: event.result,
      error: event.error,
    }
  })
  return { ...message, parts }
}

export function finishAssistantMessage(
  message: AssistantChatMessage,
  status: Exclude<AssistantMessageStatus, 'streaming'>,
  error?: string
): AssistantChatMessage {
  return { ...message, status, error }
}

export function applyAssistantStreamEvent(
  message: AssistantChatMessage,
  event: AssistantStreamEvent
): AssistantChatMessage {
  switch (event.type) {
    case 'delta':
      return appendAssistantText(message, event.text)
    case 'tool_start':
      return addAssistantToolStart(message, event)
    case 'tool_result':
      return addAssistantToolResult(message, event)
    default:
      return message
  }
}

export function updateMessageById(
  messages: AssistantChatMessage[],
  messageId: string,
  update: (message: AssistantChatMessage) => AssistantChatMessage
): AssistantChatMessage[] {
  return messages.map((message) =>
    message.id === messageId ? update(message) : message
  )
}

export function getMessageText(message: AssistantChatMessage): string {
  const parts: AssistantMessagePart[] = message.parts
  return parts
    .filter((part): part is AssistantTextPart => part.type === 'text')
    .map((part) => part.text)
    .join('\n\n')
}

export function hasMessageContent(message: AssistantChatMessage): boolean {
  return getMessageText(message).trim() !== '' || message.parts.length > 0
}

export function hasRunningTool(message: AssistantChatMessage): boolean {
  return message.parts.some(
    (part) => part.type === 'tool' && part.status === 'running'
  )
}
