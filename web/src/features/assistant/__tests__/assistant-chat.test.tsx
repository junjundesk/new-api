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
import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { Assistant } from '../index'
import { getAssistantStorageKey, saveStoredMessages } from '../lib/history'
import type { AssistantConfig, AssistantProfile } from '../types'

interface FakeSseInstance {
  url: string
  options: Record<string, unknown>
  closed: boolean
  streamed: boolean
  emit: (type: string, data?: string) => void
}

const sseMock = vi.hoisted(() => {
  type Listener = (event: { data?: string; readyState?: number }) => void

  const instances: FakeSSE[] = []

  class FakeSSE {
    readonly url: string
    readonly options: Record<string, unknown>
    readyState = 0
    status = 200
    closed = false
    streamed = false
    private listeners = new Map<string, Listener[]>()

    constructor(url: string, options: Record<string, unknown>) {
      this.url = url
      this.options = options
      instances.push(this)
    }

    addEventListener(type: string, listener: Listener): void {
      const current = this.listeners.get(type) ?? []
      current.push(listener)
      this.listeners.set(type, current)
    }

    stream(): void {
      this.streamed = true
      this.readyState = 1
    }

    close(): void {
      this.closed = true
      this.readyState = 2
    }

    emit(type: string, data?: string): void {
      for (const listener of this.listeners.get(type) ?? []) {
        listener({ data, readyState: this.readyState })
      }
    }
  }

  return { instances, SSE: FakeSSE }
})

vi.mock('sse.js', () => ({ SSE: sseMock.SSE }))
vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return {
    ...actual,
    getFreshAuthHeaders: async () => ({ Authorization: 'Bearer test-token' }),
  }
})

const configMock = vi.hoisted(() => ({
  getAssistantConfig: vi.fn(),
  confirmAssistantOperation: vi.fn(),
}))
vi.mock('@/features/assistant/api', () => configMock)

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

const clients: QueryClient[] = []

function renderAssistant(profile: AssistantProfile = 'user') {
  useAuthStore.getState().auth.setUser({
    id: 42,
    username: 'tester',
    role: 1,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <Assistant profile={profile} />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

const enabledConfig: AssistantConfig = {
  enabled: true,
  model: 'gpt-test',
  canCreateTokens: true,
  maxMessageChars: 100000,
}

const adminEnabledConfig: AssistantConfig = {
  ...enabledConfig,
  model: 'admin-gpt-test',
}

function mockConfigs(configs?: {
  user?: AssistantConfig
  admin?: AssistantConfig
}): void {
  configMock.getAssistantConfig.mockImplementation((profile?: string) =>
    Promise.resolve(
      profile === 'admin'
        ? (configs?.admin ?? adminEnabledConfig)
        : (configs?.user ?? enabledConfig)
    )
  )
}

async function sendMessage(text: string): Promise<FakeSseInstance> {
  const user = userEvent.setup()
  const textarea = await screen.findByPlaceholderText(
    'Ask about your usage or API keys'
  )
  await user.type(textarea, text)
  await user.keyboard('{Enter}')
  await waitFor(() => expect(sseMock.instances).toHaveLength(1))
  return sseMock.instances[0]
}

beforeEach(() => {
  configMock.getAssistantConfig.mockReset()
  configMock.confirmAssistantOperation.mockReset()
  configMock.confirmAssistantOperation.mockResolvedValue(undefined)
  mockConfigs()
})

afterEach(() => {
  cleanup()
  for (const client of clients.splice(0)) client.clear()
  sseMock.instances.length = 0
  window.sessionStorage.clear()
  useAuthStore.getState().auth.reset()
})

describe('assistant chat page', () => {
  it('posts the composed message and renders streamed deltas', async () => {
    renderAssistant()

    const source = await sendMessage('How much quota do I have?')

    expect(source.url).toBe('/api/assistant/chat')
    expect(configMock.getAssistantConfig).toHaveBeenCalledWith('user')
    expect(screen.getByRole('heading', { name: 'AI Assistant' })).toBeVisible()
    expect(source.streamed).toBe(true)
    const payload = JSON.parse(String(source.options.payload)) as {
      message: string
      history: unknown[]
    }
    expect(payload.message).toBe('How much quota do I have?')
    expect(payload.history).toEqual([])
    expect(screen.getByText('How much quota do I have?')).toBeVisible()

    source.emit('message', '{"type":"delta","text":"You have "}')
    source.emit('message', '{"type":"delta","text":"plenty."}')

    await waitFor(() => {
      expect(screen.getByRole('log').textContent).toContain('You have plenty.')
    })
    expect(source.closed).toBe(false)

    source.emit('message', '{"type":"done"}')

    await waitFor(() => {
      expect(source.closed).toBe(true)
    })
    await waitFor(() => {
      expect(
        screen.queryByRole('button', { name: 'Stop' })
      ).not.toBeInTheDocument()
    })
  })

  it('posts admin conversations to the admin endpoint and uses the admin title', async () => {
    renderAssistant('admin')

    expect(
      await screen.findByRole('heading', { name: 'Admin Assistant' })
    ).toBeVisible()
    expect(configMock.getAssistantConfig).toHaveBeenCalledWith('admin')

    const source = await sendMessage('Which channels are failing?')

    expect(source.url).toBe('/api/assistant/admin/chat')
    expect(source.streamed).toBe(true)
  })

  it('keeps partial text when the stream is stopped', async () => {
    const user = userEvent.setup()
    renderAssistant()

    const source = await sendMessage('Summarize my usage')
    source.emit('message', '{"type":"delta","text":"Partial answer"}')
    await waitFor(() => {
      expect(screen.getByRole('log').textContent).toContain('Partial answer')
    })

    await user.click(screen.getByRole('button', { name: 'Stop' }))

    expect(source.closed).toBe(true)
    expect(screen.getByRole('log').textContent).toContain('Partial answer')
    expect(await screen.findByText('Generation stopped')).toBeVisible()
  })

  it('shows the disabled state when the assistant is not enabled', async () => {
    mockConfigs({ user: { ...enabledConfig, enabled: false } })
    renderAssistant()

    expect(await screen.findByText('AI Assistant is unavailable')).toBeVisible()
    expect(
      screen.queryByPlaceholderText('Ask about your usage or API keys')
    ).toBeNull()
    expect(sseMock.instances).toHaveLength(0)
  })

  it('shows the admin-specific disabled state when the admin profile is not enabled', async () => {
    mockConfigs({
      user: enabledConfig,
      admin: { ...adminEnabledConfig, enabled: false },
    })
    renderAssistant('admin')

    expect(await screen.findByText('AI Assistant is unavailable')).toBeVisible()
    expect(
      screen.getByText(
        'The administrator AI assistant is not configured. Configure it in System Settings > Operations > AI Assistant.'
      )
    ).toBeVisible()
    expect(
      screen.queryByText(
        'The administrator has not enabled the AI assistant on this site.'
      )
    ).toBeNull()
    expect(sseMock.instances).toHaveLength(0)
  })

  it('shows the admin profile note in the empty state', async () => {
    renderAssistant('admin')

    expect(
      await screen.findByText(
        "This assistant uses the administrator's separate assistant profile."
      )
    ).toBeVisible()
  })

  it('keeps admin and user conversation histories apart', async () => {
    saveStoredMessages(getAssistantStorageKey(42, 'user'), [
      {
        id: 'user-message',
        role: 'user',
        parts: [{ id: 'user-text', type: 'text', text: 'user-only history' }],
        status: 'done',
        createdAt: 1,
      },
    ])
    renderAssistant('admin')

    expect(await screen.findByText('Ask about your account')).toBeVisible()
    expect(screen.queryByText('user-only history')).toBeNull()
  })

  it('shows the audit warning on the admin assistant page', async () => {
    renderAssistant('admin')

    expect(
      await screen.findByText(
        'Administrator operations are audited, and write operations require your confirmation.'
      )
    ).toBeVisible()
  })

  it('shows a pending confirmation and approves it through the user endpoint', async () => {
    const user = userEvent.setup()
    renderAssistant()

    const source = await sendMessage('Create an API key for me')
    const expiresAt = Math.floor(Date.now() / 1000) + 120
    source.emit(
      'message',
      '{"type":"tool_start","tool_call_id":"call-1","name":"execute_operation","arguments":{"operation_id":"POST /api/token"}}'
    )
    source.emit(
      'message',
      `{"type":"confirmation_required","confirmation_id":"conf-1","operation_id":"POST /api/token","risk":"write","summary":"Create an API key","params":{"body":{"name":"test-key"}},"expires_at":${expiresAt}}`
    )

    expect(await screen.findByText('Confirmation required')).toBeVisible()
    expect(screen.getByText('Create an API key')).toBeVisible()
    expect(screen.getAllByText('POST /api/token').length).toBeGreaterThan(0)
    expect(source.closed).toBe(false)

    await user.click(screen.getByRole('button', { name: 'Approve' }))
    expect(configMock.confirmAssistantOperation).toHaveBeenCalledWith(
      'user',
      'conf-1',
      true
    )

    source.emit(
      'message',
      '{"type":"confirmation_resolved","confirmation_id":"conf-1","status":"approved"}'
    )
    expect(await screen.findByText('Approved')).toBeVisible()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()

    source.emit('message', '{"type":"done"}')
    await waitFor(() => expect(source.closed).toBe(true))
  })

  it('denies an admin confirmation through the admin endpoint', async () => {
    const user = userEvent.setup()
    renderAssistant('admin')

    const source = await sendMessage('Delete that channel')
    source.emit(
      'message',
      '{"type":"tool_start","tool_call_id":"call-1","name":"execute_operation","arguments":{"operation_id":"DELETE /api/channel/3"}}'
    )
    source.emit(
      'message',
      '{"type":"confirmation_required","confirmation_id":"conf-2","operation_id":"DELETE /api/channel/3","risk":"destructive","summary":"Delete channel 3"}'
    )

    expect(await screen.findByText('Destructive operation')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Deny' }))
    expect(configMock.confirmAssistantOperation).toHaveBeenCalledWith(
      'admin',
      'conf-2',
      false
    )
  })

  it('expires a pending confirmation when the user stops the run', async () => {
    const user = userEvent.setup()
    renderAssistant()

    const source = await sendMessage('Create an API key for me')
    source.emit(
      'message',
      '{"type":"confirmation_required","confirmation_id":"conf-3","operation_id":"POST /api/token","risk":"write","summary":"Create an API key"}'
    )
    expect(await screen.findByText('Confirmation required')).toBeVisible()

    await user.click(screen.getByRole('button', { name: 'Stop' }))

    expect(source.closed).toBe(true)
    expect(await screen.findByText('Expired')).toBeVisible()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(configMock.confirmAssistantOperation).not.toHaveBeenCalled()
  })

  it('hides administrator-only tool cards from the user transcript', async () => {
    renderAssistant()

    const source = await sendMessage('Query the database')
    source.emit(
      'message',
      '{"type":"tool_start","tool_call_id":"call-db","name":"admin_db_query","arguments":{"table":"users"}}'
    )
    source.emit(
      'message',
      '{"type":"tool_result","tool_call_id":"call-db","name":"admin_db_query","ok":false,"error":"not available"}'
    )
    source.emit('message', '{"type":"delta","text":"I cannot access that."}')

    await waitFor(() => {
      expect(screen.getByRole('log').textContent).toContain(
        'I cannot access that.'
      )
    })
    expect(screen.queryByText('Database query')).toBeNull()
    expect(screen.queryByText('admin_db_query')).toBeNull()
  })

  it('shows administrator tool cards on the admin transcript', async () => {
    renderAssistant('admin')

    const source = await sendMessage('Query the database')
    source.emit(
      'message',
      '{"type":"tool_start","tool_call_id":"call-db","name":"admin_db_query","arguments":{"table":"users"}}'
    )

    expect(await screen.findByText('Database query')).toBeVisible()
  })

  it('shows a retryable error when the configuration fails to load', async () => {
    configMock.getAssistantConfig.mockRejectedValue(new Error('offline'))
    renderAssistant()

    expect(
      await screen.findByText('Failed to load the AI assistant configuration')
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeEnabled()
  })
})
