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

import {
  confirmAssistantOperation,
  getAssistantConfig,
  normalizeAssistantConfig,
} from '../api'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }))

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
          max_message_chars: 100000,
          context_window: 500000,
        },
      },
    })

    const config = await getAssistantConfig()

    expect(api.get).toHaveBeenCalledWith('/api/assistant/config')
    expect(config).toEqual({
      enabled: true,
      model: 'gpt-test',
      canCreateTokens: true,
      maxMessageChars: 100000,
      contextWindow: 500000,
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
        },
      },
    })

    const config = await getAssistantConfig('admin')

    expect(api.get).toHaveBeenCalledWith('/api/assistant/admin/runtime-config')
    expect(config.enabled).toBe(false)
    expect(config.model).toBe('admin-gpt-test')
    expect(config.maxMessageChars).toBe(2000)
  })
})

describe('assistant confirmation request', () => {
  it('posts a user approval to the user confirm endpoint', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: { success: true } })

    await confirmAssistantOperation('user', 'conf-1', true)

    expect(api.post).toHaveBeenCalledWith('/api/assistant/confirm', {
      confirmation_id: 'conf-1',
      approve: true,
    })
  })

  it('posts an admin denial to the admin confirm endpoint', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: { success: true } })

    await confirmAssistantOperation('admin', 'conf-2', false)

    expect(api.post).toHaveBeenCalledWith('/api/assistant/admin/confirm', {
      confirmation_id: 'conf-2',
      approve: false,
    })
  })

  it('surfaces the server message when the confirmation is rejected', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: {
        success: false,
        message: 'confirmation not found or expired',
      },
    })

    await expect(
      confirmAssistantOperation('user', 'conf-3', true)
    ).rejects.toThrow('confirmation not found or expired')
  })
})

describe('assistant config normalization', () => {
  it('falls back to safe defaults for a missing payload', () => {
    expect(normalizeAssistantConfig(undefined)).toEqual({
      enabled: false,
      canCreateTokens: false,
      maxMessageChars: 100000,
    })
    expect(normalizeAssistantConfig({ success: false })).toEqual({
      enabled: false,
      canCreateTokens: false,
      maxMessageChars: 100000,
    })
  })

  it('clamps invalid numeric limits and ignores a blank model name', () => {
    const config = normalizeAssistantConfig({
      enabled: 'yes',
      model: '   ',
      can_create_tokens: 1,
      max_message_chars: 100000,
    })

    expect(config.enabled).toBe(false)
    expect(config.canCreateTokens).toBe(false)
    expect(config.model).toBeUndefined()
    expect(config.maxMessageChars).toBe(100000)
  })
})
