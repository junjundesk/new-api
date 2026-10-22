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
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { DEFAULT_BREAKDOWN_FILTERS } from '@/features/dashboard/constants'
import { resolveBreakdownTimeRange } from '@/features/dashboard/lib'
import type {
  BreakdownFilters,
  UserQuotaBreakdownData,
} from '@/features/dashboard/types'
import { dateToUnixTimestamp, getStartOfDay } from '@/lib/time'

import { UserConsumptionBreakdown } from '../user-consumption-breakdown'

vi.mock('@/features/dashboard/api', () => ({
  getUserQuotaBreakdown: vi.fn(),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (
    selector: (state: { auth: { user: { role: number } } }) => unknown
  ) => selector({ auth: { user: { role: 10 } } }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      let text = key
      if (options) {
        for (const [name, value] of Object.entries(options)) {
          text = text.replaceAll(`{{${name}}}`, String(value))
        }
      }
      return text
    },
    i18n: { resolvedLanguage: 'en', language: 'en' },
  }),
}))

const { getUserQuotaBreakdown } = await import('@/features/dashboard/api')

interface BreakdownRequest {
  start_timestamp: number
  end_timestamp: number
}

// The project's test runner exposes vi.fn() but not vi.mocked(), so the module
// mock is reached through a narrow local view of what it must provide.
interface BreakdownMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<[BreakdownRequest, boolean | undefined, number[]]> }
}

const mockedFetch = getUserQuotaBreakdown as unknown as BreakdownMock

function payload(overrides: Partial<UserQuotaBreakdownData> = {}): {
  success: true
  data: UserQuotaBreakdownData
} {
  return {
    success: true,
    data: {
      rows: [
        {
          user_id: 1,
          username: 'alice',
          use_group: 'vip',
          model_name: 'gpt-4.1',
          count: 42,
          quota: 100,
          token_used: 1234,
        },
        {
          user_id: 1,
          username: 'alice',
          use_group: 'default',
          model_name: 'gpt-5',
          count: 3,
          quota: 50,
          token_used: 100,
        },
        {
          user_id: 2,
          username: 'bob',
          use_group: 'default',
          model_name: 'gpt-5',
          count: 1,
          quota: 150,
          token_used: 10,
        },
      ],
      truncated: false,
      data_export_enabled: true,
      data_export_interval_minutes: 5,
      ...overrides,
    },
  }
}

function Harness() {
  const [filters, setFilters] = useState<BreakdownFilters>(
    DEFAULT_BREAKDOWN_FILTERS
  )
  return (
    <UserConsumptionBreakdown
      filters={filters}
      onFiltersChange={setFilters}
      excludedUserIDs={[10]}
    />
  )
}

function renderPanel() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, refetchOnWindowFocus: false },
    },
  })
  return render(
    <QueryClientProvider client={client}>
      <Harness />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  mockedFetch.mockReset()
})

afterEach(() => {
  cleanup()
})

describe('UserConsumptionBreakdown', () => {
  test('asks for the current local day by default', async () => {
    mockedFetch.mockResolvedValue(payload())

    renderPanel()

    await waitFor(() => expect(mockedFetch.mock.calls.length).toBe(1))
    const [params, isAdmin, excludedUserIDs] = mockedFetch.mock.calls[0]
    expect(params.start_timestamp).toBe(
      dateToUnixTimestamp(getStartOfDay(new Date()))
    )
    expect(
      Math.abs(params.end_timestamp - Math.floor(Date.now() / 1000))
    ).toBeLessThanOrEqual(5)
    expect(isAdmin).toBe(true)
    expect(excludedUserIDs).toEqual([10])
  })

  test('switching to Yesterday refetches the previous local day', async () => {
    mockedFetch.mockResolvedValue(payload())

    renderPanel()
    await screen.findByText('alice')

    await userEvent.click(screen.getByRole('tab', { name: 'Yesterday' }))

    await waitFor(() => expect(mockedFetch.mock.calls.length).toBe(2))
    expect(mockedFetch.mock.calls[1][0]).toEqual(
      resolveBreakdownTimeRange('yesterday')
    )
  })

  test('shows the empty state when the range has no consumption', async () => {
    mockedFetch.mockResolvedValue(payload({ rows: [] }))

    renderPanel()

    expect(await screen.findByText('No data available')).not.toBeNull()
  })

  test('warns when the server capped the row list', async () => {
    mockedFetch.mockResolvedValue(payload({ truncated: true }))

    renderPanel()

    expect(
      await screen.findByText(
        'Only the top 3 rows by cost are shown. Narrow the time range for complete data.'
      )
    ).not.toBeNull()
  })

  test('explains an empty panel when usage data collection is off', async () => {
    mockedFetch.mockResolvedValue(
      payload({ rows: [], data_export_enabled: false })
    )

    renderPanel()

    expect(
      await screen.findByText(
        'Usage data collection is disabled; enable data export to collect this data.'
      )
    ).not.toBeNull()
  })

  test('renders one total row per user with collapsed group and model details', async () => {
    mockedFetch.mockResolvedValue(payload())

    renderPanel()

    await screen.findByRole('button', { name: 'alice' })
    expect(screen.getByRole('button', { name: 'bob' })).not.toBeNull()
    // Alice's two group/model rows collapse into one user row: 42 + 3 requests.
    const aliceRow = screen
      .getByRole('button', { name: 'alice' })
      .closest('tr') as HTMLElement
    expect(within(aliceRow).getByText('45')).not.toBeNull()
    expect(screen.queryByText('vip')).toBeNull()
    expect(screen.queryByText('gpt-4.1')).toBeNull()
    // Alice and Bob each account for half of the filtered total.
    expect(screen.getAllByText('50%').length).toBe(2)
  })

  test('clicking a username expands and collapses group and model details', async () => {
    mockedFetch.mockResolvedValue(payload())

    renderPanel()
    await screen.findByRole('button', { name: 'alice' })
    expect(screen.queryByText('vip')).toBeNull()

    await userEvent.click(screen.getByRole('button', { name: 'alice' }))
    expect(await screen.findByText('vip')).not.toBeNull()
    expect(screen.getByText('gpt-4.1')).not.toBeNull()
    expect(screen.getByText('default')).not.toBeNull()
    expect(screen.getByText('gpt-5')).not.toBeNull()

    await userEvent.click(screen.getByRole('button', { name: 'alice' }))
    await waitFor(() => expect(screen.queryByText('vip')).toBeNull())
  })
})
