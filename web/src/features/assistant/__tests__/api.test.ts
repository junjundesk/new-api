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
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { getAssistantConfig, normalizeAssistantConfig } from '../api'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))

afterEach(() => {
  vi.clearAllMocks()
})

describe('assistant config request', () => {
  it('loads the user profile config from the user endpoint by default', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          enabled: true,
          model: 'gpt-test',
          can_create_tokens: true,
          max_message_chars: 4000,
          max_history_messages: 20,
        },
      },
    })

    const config = await getAssistantConfig()

    expect(api.get).toHaveBeenCalledWith('/api/assistant/config')
    expect(config).toEqual({
      enabled: true,
      model: 'gpt-test',
      canCreateTokens: true,
      maxMessageChars: 4000,
      maxHistoryMessages: 20,
    })
  })

  it('loads the admin profile config from the root-only runtime-config endpoint', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          enabled: false,
          model: 'admin-gpt-test',
          can_create_tokens: false,
          max_message_chars: 2000,
          max_history_messages: 10,
        },
      },
    })

    const config = await getAssistantConfig('admin')

    expect(api.get).toHaveBeenCalledWith('/api/assistant/admin/runtime-config')
    expect(config.enabled).toBe(false)
    expect(config.model).toBe('admin-gpt-test')
    expect(config.maxMessageChars).toBe(2000)
    expect(config.maxHistoryMessages).toBe(10)
  })
})

describe('assistant config normalization', () => {
  it('falls back to safe defaults for a missing payload', () => {
    expect(normalizeAssistantConfig(undefined)).toEqual({
      enabled: false,
      canCreateTokens: false,
      maxMessageChars: 4000,
      maxHistoryMessages: 20,
    })
    expect(normalizeAssistantConfig({ success: false })).toEqual({
      enabled: false,
      canCreateTokens: false,
      maxMessageChars: 4000,
      maxHistoryMessages: 20,
    })
  })

  it('clamps invalid numeric limits and ignores a blank model name', () => {
    const config = normalizeAssistantConfig({
      enabled: 'yes',
      model: '   ',
      can_create_tokens: 1,
      max_message_chars: 100000,
      max_history_messages: -3,
    })

    expect(config.enabled).toBe(false)
    expect(config.canCreateTokens).toBe(false)
    expect(config.model).toBeUndefined()
    expect(config.maxMessageChars).toBe(4000)
    expect(config.maxHistoryMessages).toBe(20)
  })
})
