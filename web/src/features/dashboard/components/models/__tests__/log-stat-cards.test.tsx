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
import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { LogStatCards } from '../log-stat-cards'

vi.mock('@/features/dashboard/api', () => ({
  getUserQuotaDates: vi.fn(),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (
    selector: (state: { auth: { user: { role: number } } }) => unknown
  ) => selector({ auth: { user: { role: 10 } } }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { resolvedLanguage: 'en', language: 'en' },
  }),
}))

const { getUserQuotaDates } = await import('@/features/dashboard/api')

interface QuotaDatesMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
}

const mockedFetch = getUserQuotaDates as unknown as QuotaDatesMock

function payload(rows: unknown[]) {
  return { success: true, data: rows }
}

// The card value lives next to its title inside the same tile, so scoping the
// assertion to that tile proves the rate landed on the cache card rather than
// on another stat.
async function findCacheCard() {
  const title = await screen.findByText('Total Cache Rate')
  const card = title.parentElement?.parentElement
  if (!card) throw new Error('cache rate card not found')
  return within(card)
}

// The dashboard preference readers touch localStorage; happy-dom's storage is
// shadowed by Node's experimental implementation here, so the test supplies a
// minimal store instead of depending on the runner's global.
beforeEach(() => {
  mockedFetch.mockReset()
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => undefined,
    removeItem: () => undefined,
    clear: () => undefined,
    key: () => null,
    length: 0,
  })
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('LogStatCards cache rate', () => {
  test('renders the total cache rate from cached and input tokens', async () => {
    mockedFetch.mockResolvedValue(
      payload([
        {
          created_at: 1,
          quota: 10,
          count: 1,
          token_used: 100,
          cache_tokens: 40,
          cache_input_tokens: 100,
        },
        {
          created_at: 2,
          quota: 10,
          count: 1,
          token_used: 300,
          cache_tokens: 160,
          cache_input_tokens: 300,
        },
      ])
    )

    render(<LogStatCards />)

    const card = await findCacheCard()
    // 200 cache reads over 400 input tokens.
    expect(await card.findByText('50%')).toBeDefined()
  })

  test('shows a placeholder when no row reports input tokens', async () => {
    mockedFetch.mockResolvedValue(
      payload([{ created_at: 1, quota: 10, count: 1, token_used: 100 }])
    )

    render(<LogStatCards />)

    const card = await findCacheCard()
    expect(await card.findByText('--')).toBeDefined()
  })
})
