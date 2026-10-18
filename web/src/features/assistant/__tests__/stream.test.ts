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
import { describe, expect, it, vi } from 'vitest'

import { ASSISTANT_ERROR_KEYS } from '../constants'
import {
  AssistantStreamParseError,
  parseAssistantStreamError,
  parseAssistantStreamEvent,
} from '../lib/stream'
import {
  type AssistantEventSource,
  createAssistantStreamController,
} from '../lib/stream-controller'
import type { AssistantChatRequest } from '../types'

interface FakeSource extends AssistantEventSource {
  closed: boolean
  streamed: boolean
  emit: (type: string, data?: string) => void
}

function createFakeSource(): FakeSource {
  const listeners = new Map<
    string,
    Array<(event: { data?: string; readyState?: number }) => void>
  >()
  const fake: FakeSource = {
    readyState: 0,
    status: 200,
    closed: false,
    streamed: false,
    addEventListener: (type, listener) => {
      const current = listeners.get(type) ?? []
      current.push(listener)
      listeners.set(type, current)
    },
    close: () => {
      fake.closed = true
      fake.readyState = 2
    },
    stream: () => {
      fake.streamed = true
      fake.readyState = 1
    },
    emit: (type, data) => {
      for (const listener of listeners.get(type) ?? []) {
        listener({ data, readyState: fake.readyState })
      }
    },
  }
  return fake
}

const request: AssistantChatRequest = {
  message: 'How much quota have I used?',
  history: [],
}

function createHarness() {
  const source = createFakeSource()
  const onEvent = vi.fn()
  const onError = vi.fn()
  const onDone = vi.fn()
  const setStreaming = vi.fn()
  const createSource = vi.fn(() => source as AssistantEventSource)
  const controller = createAssistantStreamController({
    getHeaders: async () => ({ Authorization: 'Bearer test-token' }),
    createSource,
    onEvent,
    onError,
    onDone,
    setStreaming,
  })
  return {
    source,
    onEvent,
    onError,
    onDone,
    setStreaming,
    createSource,
    controller,
  }
}

describe('assistant stream event parsing', () => {
  it('parses text deltas, tool events, errors and done payloads', () => {
    expect(parseAssistantStreamEvent('{"type":"delta","text":"Hi"}')).toEqual({
      type: 'delta',
      text: 'Hi',
    })
    expect(
      parseAssistantStreamEvent(
        '{"type":"tool_start","tool_call_id":"call-1","name":"list_api_keys","arguments":{"limit":5}}'
      )
    ).toEqual({
      type: 'tool_start',
      toolCallId: 'call-1',
      name: 'list_api_keys',
      arguments: { limit: 5 },
    })
    expect(
      parseAssistantStreamEvent(
        '{"type":"tool_result","tool_call_id":"call-1","name":"list_api_keys","ok":true,"result":{"total":0}}'
      )
    ).toEqual({
      type: 'tool_result',
      toolCallId: 'call-1',
      name: 'list_api_keys',
      ok: true,
      result: { total: 0 },
      error: undefined,
    })
    expect(
      parseAssistantStreamEvent(
        '{"type":"error","code":"daily_limit","message":"Slow down","retry_after":60}'
      )
    ).toEqual({
      type: 'error',
      code: 'daily_limit',
      message: 'Slow down',
      retryAfter: 60,
    })
    expect(
      parseAssistantStreamEvent(
        '{"type":"error","code":"assistant_busy","error":"Assistant is busy"}'
      )
    ).toEqual({
      type: 'error',
      code: 'assistant_busy',
      message: 'Assistant is busy',
      retryAfter: undefined,
    })
    expect(parseAssistantStreamEvent('{"type":"done"}')).toEqual({
      type: 'done',
      usage: undefined,
    })
  })

  it('parses confirmation required and resolved events', () => {
    expect(
      parseAssistantStreamEvent(
        '{"type":"confirmation_required","confirmation_id":"conf-1","operation_id":"POST /api/token","risk":"write","summary":"Create an API key","params":{"body":{"name":"test"}},"expires_at":1700000000}'
      )
    ).toEqual({
      type: 'confirmation_required',
      confirmationId: 'conf-1',
      operationId: 'POST /api/token',
      risk: 'write',
      summary: 'Create an API key',
      params: { body: { name: 'test' } },
      expiresAt: 1700000000,
    })
    expect(
      parseAssistantStreamEvent(
        '{"type":"confirmation_resolved","confirmation_id":"conf-1","status":"approved"}'
      )
    ).toEqual({
      type: 'confirmation_resolved',
      confirmationId: 'conf-1',
      status: 'approved',
    })
    expect(
      parseAssistantStreamEvent(
        '{"type":"confirmation_resolved","confirmation_id":"conf-1","status":"expired"}'
      )
    ).toEqual({
      type: 'confirmation_resolved',
      confirmationId: 'conf-1',
      status: 'expired',
    })
    expect(
      parseAssistantStreamEvent('{"type":"confirmation_required"}')
    ).toBeNull()
    expect(
      parseAssistantStreamEvent(
        '{"type":"confirmation_resolved","confirmation_id":"conf-1","status":"unknown"}'
      )
    ).toBeNull()
  })

  it('ignores keep-alives, done sentinels and unknown event types', () => {
    expect(parseAssistantStreamEvent('')).toBeNull()
    expect(parseAssistantStreamEvent('[DONE]')).toBeNull()
    expect(parseAssistantStreamEvent('{"type":"future_event"}')).toBeNull()
    expect(parseAssistantStreamEvent('{"type":"delta"}')).toBeNull()
  })

  it('throws a parse error for malformed JSON', () => {
    expect(() => parseAssistantStreamEvent('not-json')).toThrow(
      AssistantStreamParseError
    )
  })

  it('extracts nested transport error messages', () => {
    expect(
      parseAssistantStreamError(
        '{"error":{"code":"unauthorized","message":"Bad token"}}'
      )
    ).toEqual({
      code: 'unauthorized',
      message: 'Bad token',
      retryAfter: undefined,
      fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
    })
    expect(
      parseAssistantStreamError('{"success":false,"message":"Invalid params"}')
    ).toEqual({
      code: undefined,
      message: 'Invalid params',
      retryAfter: undefined,
      fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
    })
    expect(parseAssistantStreamError('')).toEqual({
      fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
    })
    expect(parseAssistantStreamError('<html>bad gateway</html>')).toEqual({
      fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
    })
  })
})

describe('assistant stream controller', () => {
  it('forwards deltas and completes exactly once on the done event', async () => {
    const harness = createHarness()

    await harness.controller.send(request)

    expect(harness.createSource).toHaveBeenCalledWith(request, {
      Authorization: 'Bearer test-token',
    })
    expect(harness.source.streamed).toBe(true)
    expect(harness.setStreaming.mock.calls.at(-1)).toEqual([true])

    harness.source.emit('message', '{"type":"delta","text":"Hello"}')
    expect(harness.onEvent).toHaveBeenCalledWith({
      type: 'delta',
      text: 'Hello',
    })

    harness.source.emit('message', '{"type":"done"}')
    harness.source.emit('message', '{"type":"delta","text":"late"}')

    expect(harness.onDone).toHaveBeenCalledTimes(1)
    expect(harness.onError).not.toHaveBeenCalled()
    expect(harness.source.closed).toBe(true)
    expect(harness.setStreaming.mock.calls.at(-1)).toEqual([false])
    expect(harness.onEvent).toHaveBeenCalledTimes(1)
  })

  it('keeps the connection open while a confirmation is pending', async () => {
    const harness = createHarness()

    await harness.controller.send(request)
    harness.source.emit(
      'message',
      '{"type":"confirmation_required","confirmation_id":"conf-1","operation_id":"POST /api/token","risk":"write","summary":"Create an API key","expires_at":1700000000}'
    )

    expect(harness.onEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'confirmation_required',
        confirmationId: 'conf-1',
      })
    )
    expect(harness.source.closed).toBe(false)
    expect(harness.setStreaming.mock.calls.at(-1)).toEqual([true])
    expect(harness.onDone).not.toHaveBeenCalled()
    expect(harness.onError).not.toHaveBeenCalled()

    harness.source.emit(
      'message',
      '{"type":"confirmation_resolved","confirmation_id":"conf-1","status":"approved"}'
    )
    expect(harness.source.closed).toBe(false)

    harness.source.emit('message', '{"type":"done"}')
    expect(harness.onDone).toHaveBeenCalledTimes(1)
    expect(harness.source.closed).toBe(true)
  })

  it('cancels a pending confirmation when the stream is stopped', async () => {
    const harness = createHarness()

    await harness.controller.send(request)
    harness.source.emit(
      'message',
      '{"type":"confirmation_required","confirmation_id":"conf-1","operation_id":"POST /api/token"}'
    )
    harness.controller.stop()

    expect(harness.source.closed).toBe(true)
    expect(harness.setStreaming.mock.calls.at(-1)).toEqual([false])

    harness.source.emit(
      'message',
      '{"type":"confirmation_resolved","confirmation_id":"conf-1","status":"expired"}'
    )
    expect(harness.onEvent).toHaveBeenCalledTimes(1)
    expect(harness.onError).not.toHaveBeenCalled()
  })

  it('reports stream errors and closes the connection', async () => {
    const harness = createHarness()

    await harness.controller.send(request)
    harness.source.emit(
      'message',
      '{"type":"error","code":"busy","message":"Assistant is busy"}'
    )
    harness.source.emit('message', '{"type":"done"}')

    expect(harness.onError).toHaveBeenCalledTimes(1)
    expect(harness.onError.mock.calls[0][0]).toMatchObject({
      code: 'busy',
      message: 'Assistant is busy',
    })
    expect(harness.onDone).not.toHaveBeenCalled()
    expect(harness.source.closed).toBe(true)
  })

  it('closes the connection on stop and ignores subsequent events', async () => {
    const harness = createHarness()

    await harness.controller.send(request)
    harness.controller.stop()

    expect(harness.source.closed).toBe(true)
    expect(harness.setStreaming.mock.calls.at(-1)).toEqual([false])

    harness.source.emit('message', '{"type":"delta","text":"late"}')
    expect(harness.onEvent).not.toHaveBeenCalled()
    expect(harness.onError).not.toHaveBeenCalled()
  })

  it('treats a closed transport without a terminal event as an error', async () => {
    const harness = createHarness()

    await harness.controller.send(request)
    harness.source.readyState = 2
    harness.source.emit('readystatechange')

    expect(harness.onError).toHaveBeenCalledTimes(1)
    expect(harness.onError.mock.calls[0][0].fallbackKey).toBe(
      ASSISTANT_ERROR_KEYS.CONNECTION_CLOSED
    )
    expect(harness.source.closed).toBe(true)
  })

  it('reports malformed event payloads without hanging the request', async () => {
    const harness = createHarness()

    await harness.controller.send(request)
    harness.source.emit('message', 'not-json')

    expect(harness.onError).toHaveBeenCalledTimes(1)
    expect(harness.onError.mock.calls[0][0].fallbackKey).toBe(
      ASSISTANT_ERROR_KEYS.STREAM_PARSE_FAILED
    )
    expect(harness.source.closed).toBe(true)
  })

  it('ignores events from a superseded connection', async () => {
    const first = createFakeSource()
    const second = createFakeSource()
    const sources = [first, second]
    const onEvent = vi.fn()
    const onError = vi.fn()
    const controller = createAssistantStreamController({
      getHeaders: async () => ({}),
      createSource: () => sources.shift() ?? second,
      onEvent,
      onError,
      onDone: vi.fn(),
      setStreaming: vi.fn(),
    })

    await controller.send(request)
    await controller.send(request)

    first.emit('message', '{"type":"delta","text":"stale"}')
    second.emit('message', '{"type":"delta","text":"fresh"}')

    expect(onEvent).toHaveBeenCalledTimes(1)
    expect(onEvent).toHaveBeenCalledWith({ type: 'delta', text: 'fresh' })
    expect(onError).not.toHaveBeenCalled()
  })
})
