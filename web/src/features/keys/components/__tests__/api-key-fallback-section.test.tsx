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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { TokenFallbackModel } from '../../types'
import { ApiKeyFallbackSection } from '../api-key-fallback-section'
import type { ApiKeyGroupOption } from '../api-key-group-combobox'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      let text = key
      for (const [name, value] of Object.entries(options ?? {})) {
        text = text.replaceAll(`{{${name}}}`, String(value))
      }
      return text
    },
  }),
}))

vi.mock('../../api', () => ({
  getFallbackModelOptions: vi.fn(),
}))

const groupOptions: ApiKeyGroupOption[] = [
  { value: 'vip', label: 'vip', ratio: 2 },
]

const draft: TokenFallbackModel[] = [
  { id: 'fallback-1', group: '', model: '', source_model: '' },
]

function renderSection(props: {
  enabled: boolean
  value?: TokenFallbackModel[]
  enableBlockedReason?: string
  onEnabledChange?: (enabled: boolean) => void
}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ApiKeyFallbackSection
        enabled={props.enabled}
        onEnabledChange={props.onEnabledChange ?? vi.fn()}
        value={props.value ?? []}
        onChange={vi.fn()}
        groupOptions={groupOptions}
        sourceModels={['gpt-4o']}
        showValidation={false}
        enableBlockedReason={props.enableBlockedReason}
      />
    </QueryClientProvider>
  )
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('ApiKeyFallbackSection', () => {
  it('blocks enabling fallback while a model-based chain is selected', () => {
    const onEnabledChange = vi.fn()
    renderSection({
      enabled: false,
      enableBlockedReason:
        'Disable fallback before selecting this chain; your draft will be kept.',
      onEnabledChange,
    })

    const toggle = screen.getByRole('switch', { name: 'Enable fallback' })
    expect(toggle.getAttribute('aria-checked')).toBe('false')
    expect(toggle.getAttribute('aria-disabled')).toBe('true')
    expect(screen.getByRole('status').textContent).toBe(
      'Disable fallback before selecting this chain; your draft will be kept.'
    )

    fireEvent.click(toggle)
    expect(onEnabledChange).not.toHaveBeenCalled()
  })

  it('lets the user turn an already-enabled draft off while blocked', () => {
    const onEnabledChange = vi.fn()
    renderSection({
      enabled: true,
      value: draft,
      enableBlockedReason:
        'Disable fallback before selecting this chain; your draft will be kept.',
      onEnabledChange,
    })

    const toggle = screen.getByRole('switch', { name: 'Enable fallback' })
    expect(toggle.getAttribute('aria-checked')).toBe('true')
    expect(toggle.getAttribute('aria-disabled')).not.toBe('true')
    expect(screen.getByText('1 / 5 fallback models')).toBeTruthy()

    fireEvent.click(toggle)
    expect(onEnabledChange.mock.calls[0][0]).toBe(false)
  })

  it('keeps the switch usable when no model-based chain is selected', () => {
    const onEnabledChange = vi.fn()
    renderSection({ enabled: false, onEnabledChange })

    const toggle = screen.getByRole('switch', { name: 'Enable fallback' })
    expect(toggle.getAttribute('aria-disabled')).not.toBe('true')
    expect(screen.queryByRole('status')).toBeNull()

    fireEvent.click(toggle)
    expect(onEnabledChange.mock.calls[0][0]).toBe(true)
  })
})
