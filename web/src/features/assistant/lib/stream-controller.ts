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
import { ASSISTANT_ERROR_KEYS } from '../constants'
import type {
  AssistantChatRequest,
  AssistantStreamError,
  AssistantStreamEvent,
} from '../types'
import {
  getAssistantStreamStatusError,
  isAssistantStreamClosed,
  parseAssistantStreamError,
  parseAssistantStreamEvent,
} from './stream'

export interface AssistantEventSourceEvent {
  data?: string
  readyState?: number
  responseCode?: number
}

export interface AssistantEventSource {
  readyState?: number
  status?: number
  addEventListener: (
    type: string,
    listener: (event: AssistantEventSourceEvent) => void
  ) => void
  close: () => void
  stream: () => void
}

export interface AssistantStreamControllerRuntime {
  getHeaders: () => Promise<Record<string, string>>
  createSource: (
    payload: AssistantChatRequest,
    headers: Record<string, string>
  ) => AssistantEventSource
  onEvent: (event: AssistantStreamEvent) => void
  onError: (error: AssistantStreamError) => void
  onDone: () => void
  setStreaming: (streaming: boolean) => void
}

export interface AssistantStreamController {
  send: (payload: AssistantChatRequest) => Promise<void>
  stop: () => void
  dispose: () => void
}

/**
 * Owns one SSE connection at a time. Mirrors the playground stream controller:
 * a generation counter makes late events from superseded requests harmless, and
 * every terminal path (done, in-stream error, transport error, forced close)
 * closes the connection exactly once.
 */
export function createAssistantStreamController(
  runtime: AssistantStreamControllerRuntime
): AssistantStreamController {
  let source: AssistantEventSource | null = null
  let generation = 0

  const closeActiveSource = (target: AssistantEventSource): void => {
    target.close()
    if (source === target) {
      source = null
      runtime.setStreaming(false)
    }
  }

  const send = async (payload: AssistantChatRequest): Promise<void> => {
    const requestGeneration = generation + 1
    generation = requestGeneration
    const previousSource = source
    source = null
    previousSource?.close()
    runtime.setStreaming(false)

    let headers: Record<string, string>
    try {
      headers = await runtime.getHeaders()
    } catch (error: unknown) {
      if (generation !== requestGeneration) return
      runtime.onError({
        message: error instanceof Error ? error.message : undefined,
        fallbackKey: ASSISTANT_ERROR_KEYS.STREAM_START_FAILED,
      })
      return
    }
    if (generation !== requestGeneration) return

    let nextSource: AssistantEventSource
    try {
      nextSource = runtime.createSource(payload, headers)
    } catch {
      runtime.onError({
        fallbackKey: ASSISTANT_ERROR_KEYS.STREAM_START_FAILED,
      })
      return
    }

    source = nextSource
    runtime.setStreaming(true)
    let completed = false

    const isCurrent = (): boolean =>
      generation === requestGeneration && source === nextSource

    const finish = (handler: () => void): void => {
      if (!isCurrent() || completed) return
      completed = true
      closeActiveSource(nextSource)
      handler()
    }

    const fail = (error: AssistantStreamError): void => {
      finish(() => runtime.onError(error))
    }

    nextSource.addEventListener('message', (event) => {
      if (!isCurrent() || completed) return

      let streamEvent: AssistantStreamEvent | null
      try {
        streamEvent = parseAssistantStreamEvent(event.data ?? '')
      } catch {
        fail({ fallbackKey: ASSISTANT_ERROR_KEYS.STREAM_PARSE_FAILED })
        return
      }
      if (!streamEvent) return

      if (streamEvent.type === 'done') {
        finish(() => runtime.onDone())
        return
      }
      if (streamEvent.type === 'error') {
        fail({
          code: streamEvent.code,
          message: streamEvent.message,
          retryAfter: streamEvent.retryAfter,
          fallbackKey: ASSISTANT_ERROR_KEYS.REQUEST_FAILED,
        })
        return
      }
      runtime.onEvent(streamEvent)
    })

    nextSource.addEventListener('error', (event) => {
      if (!isCurrent() || completed) return
      const error = parseAssistantStreamError(event.data)
      if (error.message === undefined && event.responseCode !== undefined) {
        error.message = `HTTP ${event.responseCode}`
      }
      fail(error)
    })

    nextSource.addEventListener('readystatechange', () => {
      if (!isCurrent() || completed) return
      if (!isAssistantStreamClosed(nextSource.readyState)) return
      const statusError = getAssistantStreamStatusError(nextSource.status)
      if (statusError) {
        fail(statusError)
        return
      }
      fail({ fallbackKey: ASSISTANT_ERROR_KEYS.CONNECTION_CLOSED })
    })

    try {
      if (!isCurrent()) return
      nextSource.stream()
    } catch {
      fail({ fallbackKey: ASSISTANT_ERROR_KEYS.STREAM_START_FAILED })
    }
  }

  const cancel = (notify: boolean): void => {
    generation += 1
    const activeSource = source
    source = null
    activeSource?.close()
    if (notify) runtime.setStreaming(false)
  }

  const stop = (): void => cancel(true)
  const dispose = (): void => cancel(false)

  return { send, stop, dispose }
}
