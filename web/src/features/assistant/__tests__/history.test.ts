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
import { afterEach, describe, expect, it } from 'vitest'

import { ASSISTANT_STORAGE_KEY_PREFIX } from '../constants'
import {
  buildAssistantRequestHistory,
  clearStoredMessages,
  getAssistantStorageKey,
  loadStoredMessages,
  saveStoredMessages,
} from '../lib/history'
import type {
  AssistantChatMessage,
  AssistantConfirmationStatus,
  AssistantMessageStatus,
} from '../types'

interface MessageOptions {
  id: string
  role?: 'user' | 'assistant'
  text?: string
  status?: AssistantMessageStatus
}

function createMessage(options: MessageOptions): AssistantChatMessage {
  return {
    id: options.id,
    role: options.role ?? 'user',
    parts: [
      { id: `${options.id}-text`, type: 'text', text: options.text ?? '' },
    ],
    status: options.status ?? 'done',
    createdAt: 0,
  }
}

function createConfirmationMessage(
  confirmationStatus: AssistantConfirmationStatus
): AssistantChatMessage {
  return {
    id: 'confirmation-message',
    role: 'assistant',
    parts: [
      { id: 'confirmation-text', type: 'text', text: 'Please confirm' },
      {
        id: 'confirmation-conf-1',
        type: 'confirmation',
        confirmationId: 'conf-1',
        operationId: 'POST /api/token',
        risk: 'write',
        summary: 'Create an API key',
        params: { body: { name: 'test' } },
        expiresAt: 1700000000,
        status: confirmationStatus,
      },
    ],
    status: 'done',
    createdAt: 0,
  }
}

const STORAGE_KEY = getAssistantStorageKey(42)

afterEach(() => {
  window.sessionStorage.clear()
})

describe('assistant request history', () => {
  it('keeps only completed non-empty turns in chronological order', () => {
    const messages = [
      createMessage({ id: '1', text: 'First question' }),
      createMessage({ id: '2', role: 'assistant', text: 'First answer' }),
      createMessage({
        id: '3',
        text: 'Streaming question',
        status: 'streaming',
      }),
      createMessage({
        id: '4',
        role: 'assistant',
        text: 'Failed answer',
        status: 'error',
      }),
      createMessage({ id: '5', text: '   ' }),
      createMessage({ id: '6', text: 'Stopped question', status: 'stopped' }),
      createMessage({ id: '7', text: 'Second question' }),
    ]

    expect(buildAssistantRequestHistory(messages)).toEqual([
      { role: 'user', content: 'First question' },
      { role: 'assistant', content: 'First answer' },
      { role: 'user', content: 'Second question' },
    ])
  })

  it('keeps only the newest turns within the message budget', () => {
    const messages = [
      createMessage({ id: '1', text: 'one' }),
      createMessage({ id: '2', text: 'two' }),
      createMessage({ id: '3', text: 'three' }),
    ]

    expect(
      buildAssistantRequestHistory(messages, { maxMessages: 2, maxChars: 100 })
    ).toEqual([
      { role: 'user', content: 'two' },
      { role: 'user', content: 'three' },
    ])
  })

  it('keeps the newest characters within the character budget', () => {
    const messages = [
      createMessage({ id: '1', text: 'aaaa' }),
      createMessage({ id: '2', role: 'assistant', text: 'bbbbbb' }),
    ]

    expect(
      buildAssistantRequestHistory(messages, { maxMessages: 20, maxChars: 8 })
    ).toEqual([
      { role: 'user', content: 'aa' },
      { role: 'assistant', content: 'bbbbbb' },
    ])
  })
})

describe('assistant message persistence', () => {
  it('persists settled messages and drops streaming ones', () => {
    const messages = [
      createMessage({ id: '1', text: 'done message' }),
      createMessage({ id: '2', text: 'half written', status: 'streaming' }),
    ]

    saveStoredMessages(STORAGE_KEY, messages)

    const stored = loadStoredMessages(STORAGE_KEY)
    expect(stored).toHaveLength(1)
    expect(stored[0].id).toBe('1')
  })

  it('ignores corrupt or unexpected stored payloads', () => {
    window.sessionStorage.setItem(STORAGE_KEY, 'not-json')
    expect(loadStoredMessages(STORAGE_KEY)).toEqual([])

    window.sessionStorage.setItem(STORAGE_KEY, '{"unexpected":true}')
    expect(loadStoredMessages(STORAGE_KEY)).toEqual([])
  })

  it('clears the stored conversation for the current user only', () => {
    saveStoredMessages(STORAGE_KEY, [createMessage({ id: '1', text: 'hi' })])
    saveStoredMessages(getAssistantStorageKey(7), [
      createMessage({ id: '2', text: 'other user' }),
    ])

    clearStoredMessages(STORAGE_KEY)

    expect(loadStoredMessages(STORAGE_KEY)).toEqual([])
    expect(loadStoredMessages(getAssistantStorageKey(7))).toHaveLength(1)
  })

  it('keeps user and admin conversations in separate keys', () => {
    saveStoredMessages(getAssistantStorageKey(42, 'user'), [
      createMessage({ id: '1', text: 'user history' }),
    ])
    saveStoredMessages(getAssistantStorageKey(42, 'admin'), [
      createMessage({ id: '2', text: 'admin history' }),
    ])

    expect(
      loadStoredMessages(getAssistantStorageKey(42, 'user')).map(
        (message) => message.id
      )
    ).toEqual(['1'])
    expect(
      loadStoredMessages(getAssistantStorageKey(42, 'admin')).map(
        (message) => message.id
      )
    ).toEqual(['2'])
  })

  it('restores a pending confirmation as expired after a reload', () => {
    saveStoredMessages(STORAGE_KEY, [createConfirmationMessage('pending')])

    const stored = loadStoredMessages(STORAGE_KEY)

    expect(stored).toHaveLength(1)
    expect(stored[0].parts[1]).toMatchObject({
      type: 'confirmation',
      confirmationId: 'conf-1',
      status: 'expired',
    })
  })

  it('never persists a pending confirmation as approvable', () => {
    saveStoredMessages(STORAGE_KEY, [createConfirmationMessage('pending')])

    const raw = window.sessionStorage.getItem(STORAGE_KEY) ?? ''

    expect(raw).toContain('"status":"expired"')
    expect(raw).not.toContain('"status":"pending"')
  })

  it('keeps an already resolved confirmation status after a reload', () => {
    saveStoredMessages(STORAGE_KEY, [createConfirmationMessage('approved')])

    const stored = loadStoredMessages(STORAGE_KEY)

    expect(stored[0].parts[1]).toMatchObject({
      type: 'confirmation',
      status: 'approved',
    })
  })

  it('keeps confirmation parts out of the request history', () => {
    expect(
      buildAssistantRequestHistory([createConfirmationMessage('denied')])
    ).toEqual([{ role: 'assistant', content: 'Please confirm' }])
  })

  it('scopes storage keys per user and per profile', () => {
    expect(getAssistantStorageKey()).toBe(
      `${ASSISTANT_STORAGE_KEY_PREFIX}:user`
    )
    expect(getAssistantStorageKey(42)).toBe(
      `${ASSISTANT_STORAGE_KEY_PREFIX}:user:42`
    )
    expect(getAssistantStorageKey(42, 'admin')).toBe(
      `${ASSISTANT_STORAGE_KEY_PREFIX}:admin:42`
    )
    expect(getAssistantStorageKey(42, 'admin')).not.toBe(
      getAssistantStorageKey(42, 'user')
    )
  })
})
