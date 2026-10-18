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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SSE } from 'sse.js'

import { getFreshAuthHeaders } from '@/lib/api'

import { confirmAssistantOperation } from '../api'
import { ASSISTANT_API_ENDPOINTS } from '../constants'
import {
  buildAssistantRequestHistory,
  clearStoredMessages,
  loadStoredMessages,
  saveStoredMessages,
} from '../lib/history'
import {
  applyAssistantStreamEvent,
  createAssistantMessage,
  createUserMessage,
  expirePendingConfirmations,
  finishAssistantMessage,
  updateMessageById,
} from '../lib/messages'
import { resolveAssistantStreamError } from '../lib/stream'
import {
  type AssistantEventSource,
  type AssistantStreamController,
  createAssistantStreamController,
} from '../lib/stream-controller'
import type {
  AssistantChatMessage,
  AssistantChatRequest,
  AssistantProfile,
  AssistantStreamError,
  AssistantStreamEvent,
} from '../types'

interface AssistantStreamCallbacks {
  onEvent: (event: AssistantStreamEvent) => void
  onError: (error: AssistantStreamError) => void
  onDone: () => void
}

export interface UseAssistantChatOptions {
  profile?: AssistantProfile
  storageKey: string
  maxMessageChars: number
  maxHistoryMessages: number
  maxHistoryChars: number
}

export interface UseAssistantChatResult {
  messages: AssistantChatMessage[]
  isStreaming: boolean
  activeModel: string | null
  sendMessage: (text: string) => void
  stopStreaming: () => void
  clearMessages: () => void
  confirmOperation: (confirmationId: string, approve: boolean) => Promise<void>
}

export function useAssistantChat(
  options: UseAssistantChatOptions
): UseAssistantChatResult {
  const { t } = useTranslation()
  const profile = options.profile ?? 'user'
  const storageKey = options.storageKey
  const maxMessageChars = options.maxMessageChars
  const maxHistoryMessages = options.maxHistoryMessages
  const maxHistoryChars = options.maxHistoryChars

  const [messages, setMessages] = useState<AssistantChatMessage[]>(() =>
    loadStoredMessages(storageKey)
  )
  const [isStreaming, setIsStreaming] = useState(false)
  const [activeModel, setActiveModel] = useState<string | null>(null)

  const isStreamingRef = useRef(false)
  const messagesRef = useRef(messages)
  const activeAssistantIdRef = useRef<string | null>(null)
  const callbacksRef = useRef<AssistantStreamCallbacks | null>(null)
  const loadedStorageKeyRef = useRef(storageKey)

  useEffect(() => {
    messagesRef.current = messages
  }, [messages])

  const setStreaming = useCallback((streaming: boolean): void => {
    isStreamingRef.current = streaming
    setIsStreaming(streaming)
  }, [])

  const chatEndpoint = ASSISTANT_API_ENDPOINTS[profile].CHAT
  const controller = useMemo<AssistantStreamController>(
    () =>
      createAssistantStreamController({
        getHeaders: getFreshAuthHeaders,
        createSource: (payload: AssistantChatRequest, headers) =>
          new SSE(chatEndpoint, {
            headers,
            method: 'POST',
            payload: JSON.stringify(payload),
          }) as unknown as AssistantEventSource,
        onEvent: (event) => callbacksRef.current?.onEvent(event),
        onError: (error) => callbacksRef.current?.onError(error),
        onDone: () => callbacksRef.current?.onDone(),
        setStreaming,
      }),
    [chatEndpoint, setStreaming]
  )

  useEffect(() => {
    loadedStorageKeyRef.current = storageKey
    controller.stop()
    setMessages(loadStoredMessages(storageKey))
    setActiveModel(null)
  }, [controller, storageKey])

  useEffect(() => {
    if (loadedStorageKeyRef.current !== storageKey) return
    if (isStreamingRef.current) return
    saveStoredMessages(storageKey, messages)
  }, [messages, storageKey])

  useEffect(
    () => () => {
      controller.dispose()
    },
    [controller]
  )

  const sendMessage = useCallback(
    (rawText: string): void => {
      const text = rawText.trim().slice(0, maxMessageChars)
      if (text === '' || isStreamingRef.current) return

      const history = buildAssistantRequestHistory(messagesRef.current, {
        maxMessages: maxHistoryMessages,
        maxChars: maxHistoryChars,
      })
      const userMessage = createUserMessage(text)
      const assistantMessage = createAssistantMessage()
      activeAssistantIdRef.current = assistantMessage.id

      callbacksRef.current = {
        onEvent: (event) => {
          if (event.type === 'meta') {
            if (event.model) setActiveModel(event.model)
            return
          }
          setMessages((current) =>
            updateMessageById(current, assistantMessage.id, (message) =>
              applyAssistantStreamEvent(message, event)
            )
          )
        },
        onError: (error) => {
          if (activeAssistantIdRef.current === assistantMessage.id) {
            activeAssistantIdRef.current = null
          }
          const errorText = resolveAssistantStreamError(error, t)
          setMessages((current) =>
            updateMessageById(current, assistantMessage.id, (message) =>
              finishAssistantMessage(
                expirePendingConfirmations(message),
                'error',
                errorText
              )
            )
          )
        },
        onDone: () => {
          if (activeAssistantIdRef.current === assistantMessage.id) {
            activeAssistantIdRef.current = null
          }
          setMessages((current) =>
            updateMessageById(current, assistantMessage.id, (message) =>
              finishAssistantMessage(
                expirePendingConfirmations(message),
                'done'
              )
            )
          )
        },
      }

      setMessages((current) => [...current, userMessage, assistantMessage])
      controller.send({ message: text, history }).catch(() => {
        /* The controller reports every failure through onError. */
      })
    },
    [controller, maxHistoryChars, maxHistoryMessages, maxMessageChars, t]
  )

  const stopStreaming = useCallback((): void => {
    const assistantId = activeAssistantIdRef.current
    activeAssistantIdRef.current = null
    controller.stop()
    if (!assistantId) return
    setMessages((current) =>
      updateMessageById(current, assistantId, (message) =>
        finishAssistantMessage(expirePendingConfirmations(message), 'stopped')
      )
    )
  }, [controller])

  const clearMessages = useCallback((): void => {
    activeAssistantIdRef.current = null
    controller.stop()
    setMessages([])
    setActiveModel(null)
    clearStoredMessages(storageKey)
  }, [controller, storageKey])

  const confirmOperation = useCallback(
    async (confirmationId: string, approve: boolean): Promise<void> => {
      await confirmAssistantOperation(profile, confirmationId, approve)
    },
    [profile]
  )

  return {
    messages,
    isStreaming,
    activeModel,
    sendMessage,
    stopStreaming,
    clearMessages,
    confirmOperation,
  }
}
