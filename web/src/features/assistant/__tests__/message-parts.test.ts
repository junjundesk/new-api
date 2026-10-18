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
import { describe, expect, it } from 'vitest'

import {
  addAssistantToolResult,
  addAssistantToolStart,
  appendAssistantText,
  applyAssistantStreamEvent,
  createAssistantMessage,
  createUserMessage,
  expirePendingConfirmations,
  finishAssistantMessage,
  getMessageText,
  hasRunningTool,
  updateMessageById,
} from '../lib/messages'

describe('assistant message parts', () => {
  it('creates a completed user message with the given text', () => {
    const message = createUserMessage('Show my usage')
    expect(message.role).toBe('user')
    expect(message.status).toBe('done')
    expect(getMessageText(message)).toBe('Show my usage')
  })

  it('merges consecutive deltas into one text part', () => {
    let message = createAssistantMessage()
    message = appendAssistantText(message, 'Hello')
    message = appendAssistantText(message, ' world')

    expect(message.parts).toHaveLength(1)
    expect(getMessageText(message)).toBe('Hello world')
  })

  it('keeps text parts and tool parts in arrival order', () => {
    let message = createAssistantMessage()
    message = applyAssistantStreamEvent(message, {
      type: 'delta',
      text: 'Checking keys...',
    })
    message = applyAssistantStreamEvent(message, {
      type: 'tool_start',
      toolCallId: 'call-1',
      name: 'list_api_keys',
    })
    message = applyAssistantStreamEvent(message, {
      type: 'delta',
      text: 'Here they are.',
    })

    expect(message.parts.map((part) => part.type)).toEqual([
      'text',
      'tool',
      'text',
    ])
    expect(getMessageText(message)).toBe('Checking keys...\n\nHere they are.')
  })

  it('marks a running tool as completed when its result arrives', () => {
    let message = createAssistantMessage()
    message = addAssistantToolStart(message, {
      type: 'tool_start',
      toolCallId: 'call-1',
      name: 'create_api_key',
    })
    expect(hasRunningTool(message)).toBe(true)

    message = addAssistantToolResult(message, {
      type: 'tool_result',
      toolCallId: 'call-1',
      name: 'create_api_key',
      ok: true,
      result: { id: 1 },
    })

    expect(hasRunningTool(message)).toBe(false)
    expect(message.parts[0]).toMatchObject({
      type: 'tool',
      status: 'done',
      result: { id: 1 },
    })
  })

  it('appends a tool part when a result arrives without a start event', () => {
    let message = createAssistantMessage()
    message = addAssistantToolResult(message, {
      type: 'tool_result',
      toolCallId: 'call-9',
      name: 'list_api_keys',
      ok: false,
      error: 'Request failed',
    })

    expect(message.parts).toHaveLength(1)
    expect(message.parts[0]).toMatchObject({
      type: 'tool',
      status: 'error',
      error: 'Request failed',
    })
  })

  it('marks a failed tool as an error result', () => {
    let message = createAssistantMessage()
    message = addAssistantToolStart(message, {
      type: 'tool_start',
      toolCallId: 'call-2',
      name: 'create_api_key',
    })
    message = addAssistantToolResult(message, {
      type: 'tool_result',
      toolCallId: 'call-2',
      name: 'create_api_key',
      ok: false,
      error: 'Name already exists',
    })

    expect(message.parts[0]).toMatchObject({
      type: 'tool',
      status: 'error',
      error: 'Name already exists',
    })
  })

  it('preserves partial text when the stream is stopped or fails', () => {
    let message = createAssistantMessage()
    message = appendAssistantText(message, 'Partial answer')
    const stopped = finishAssistantMessage(message, 'stopped')
    const failed = finishAssistantMessage(message, 'error', 'Network down')

    expect(getMessageText(stopped)).toBe('Partial answer')
    expect(stopped.status).toBe('stopped')
    expect(getMessageText(failed)).toBe('Partial answer')
    expect(failed.error).toBe('Network down')
  })

  it('adds a pending confirmation part and resolves it from the stream', () => {
    let message = createAssistantMessage()
    message = applyAssistantStreamEvent(message, {
      type: 'confirmation_required',
      confirmationId: 'conf-1',
      operationId: 'DELETE /api/token/7',
      risk: 'destructive',
      summary: 'Delete the API key',
      params: { path_params: { id: 7 } },
      expiresAt: 1700000000,
    })

    expect(message.parts).toHaveLength(1)
    expect(message.parts[0]).toMatchObject({
      type: 'confirmation',
      confirmationId: 'conf-1',
      operationId: 'DELETE /api/token/7',
      status: 'pending',
    })

    message = applyAssistantStreamEvent(message, {
      type: 'confirmation_resolved',
      confirmationId: 'conf-1',
      status: 'denied',
    })

    expect(message.parts[0]).toMatchObject({
      type: 'confirmation',
      status: 'denied',
    })
  })

  it('expires any confirmation still pending when the run ends', () => {
    let message = createAssistantMessage()
    message = applyAssistantStreamEvent(message, {
      type: 'confirmation_required',
      confirmationId: 'conf-1',
      operationId: 'POST /api/token',
    })

    const expired = expirePendingConfirmations(message)

    expect(expired.parts[0]).toMatchObject({
      type: 'confirmation',
      status: 'expired',
    })
  })

  it('updates only the targeted message', () => {
    const first = createUserMessage('first')
    const second = createUserMessage('second')
    const updated = updateMessageById([first, second], second.id, (message) =>
      finishAssistantMessage(message, 'error', 'failed')
    )

    expect(updated[0]).toBe(first)
    expect(updated[1].status).toBe('error')
  })
})
