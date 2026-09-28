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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import type {
  PerfGroupsData,
  PerformanceGroupSummary,
} from '@/features/performance-metrics/types'

import { Performance } from '../index'
import { CUSTOM_ORDER_STORAGE_KEY, SORT_STORAGE_KEY } from '../lib/sort-storage'
import { makeGroup } from './fixtures'
import { installMemoryStorage, installReducedMotion } from './test-environment'

vi.mock('@/features/performance-metrics/api', () => ({
  getPerfMetricsGroups: vi.fn(),
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

const { getPerfMetricsGroups } =
  await import('@/features/performance-metrics/api')
const mockedFetch = vi.mocked(getPerfMetricsGroups)

const GROUP_NAMES = ['alpha', 'beta', 'gamma']

function envelope(groups: PerformanceGroupSummary[]): PerfGroupsData {
  return {
    success: true,
    data: {
      groups,
      cache_observed: true,
      cache_hit_rate: 0,
      start_ts: 0,
      end_ts: 86399,
      bucket_seconds: 3600,
    },
  }
}

function healthy(groups: string[]): PerformanceGroupSummary[] {
  return groups.map((group) =>
    makeGroup({ group, request_count: 5, success_count: 5, success_rate: 95 })
  )
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, refetchOnWindowFocus: false },
    },
  })
  return render(
    <QueryClientProvider client={client}>
      <Performance />
    </QueryClientProvider>
  )
}

/** Group keys in render order, read from the first label of every card. */
function cardOrder(container: HTMLElement): string[] {
  return [...container.querySelectorAll('article')].map((card) => {
    const text = card.textContent ?? ''
    return GROUP_NAMES.find((name) => text.startsWith(name)) ?? 'unknown'
  })
}

beforeEach(() => {
  installMemoryStorage().clear()
  installReducedMotion()
  mockedFetch.mockReset()
})

afterEach(() => {
  cleanup()
})

describe('Performance page', () => {
  test('renders the subtitle, time range tabs and refresh control', async () => {
    mockedFetch.mockResolvedValue(envelope(healthy(GROUP_NAMES)))
    renderPage()

    expect(
      screen.getByText(
        'Live availability and latency for the groups your account can use.'
      )
    ).not.toBeNull()
    expect(screen.getByRole('tab', { name: 'Last hour' }).textContent).toBe(
      '1h'
    )
    expect(screen.getByRole('tab', { name: 'Last 24 hours' }).textContent).toBe(
      '24h'
    )
    expect(screen.getByRole('tab', { name: 'Last 7 days' }).textContent).toBe(
      '7d'
    )
    expect(screen.getByRole('button', { name: 'Refresh' })).not.toBeNull()

    // Both captions share one paragraph, so the matchers stay unanchored.
    expect(
      await screen.findByText(/Auto-refreshes every minute/)
    ).not.toBeNull()
    expect(screen.getByText(/Updated \d{2}:\d{2}:\d{2}/)).not.toBeNull()
    expect(screen.getByText('3 groups')).not.toBeNull()
    expect(mockedFetch).toHaveBeenCalledWith(24, 'en')
  })

  test('switches the requested window when another time range is chosen', async () => {
    mockedFetch.mockResolvedValue(envelope(healthy(GROUP_NAMES)))
    renderPage()
    await screen.findByText('3 groups')

    await userEvent
      .setup()
      .click(screen.getByRole('tab', { name: 'Last 7 days' }))
    await waitFor(() => expect(mockedFetch).toHaveBeenCalledWith(168, 'en'))
  })

  test('reads the persisted sort and writes it back', async () => {
    window.localStorage.setItem(SORT_STORAGE_KEY, 'latency')
    mockedFetch.mockResolvedValue(envelope(healthy(GROUP_NAMES)))
    renderPage()

    expect(await screen.findByLabelText('Sort groups')).not.toBeNull()
    expect(screen.getByLabelText('Sort groups').textContent).toContain(
      'Latency: low to high'
    )
    expect(window.localStorage.getItem(SORT_STORAGE_KEY)).toBe('latency')
  })

  test('reads the persisted custom order and reorders with the move controls', async () => {
    window.localStorage.setItem(SORT_STORAGE_KEY, 'custom')
    window.localStorage.setItem(CUSTOM_ORDER_STORAGE_KEY, '["beta","alpha"]')
    mockedFetch.mockResolvedValue(envelope(healthy(GROUP_NAMES)))
    const { container } = renderPage()

    expect(
      await screen.findByText(
        'Use Move up and Move down to save your group order on this device. Filters keep hidden groups in place.'
      )
    ).not.toBeNull()
    await waitFor(() =>
      expect(cardOrder(container)).toEqual(['beta', 'alpha', 'gamma'])
    )

    await userEvent.setup().click(screen.getByLabelText('Move beta down'))
    await waitFor(() =>
      expect(cardOrder(container)).toEqual(['alpha', 'beta', 'gamma'])
    )
    expect(window.localStorage.getItem(CUSTOM_ORDER_STORAGE_KEY)).toBe(
      '["alpha","beta","gamma"]'
    )
  })

  test('counts groups by status and filters the grid', async () => {
    mockedFetch.mockResolvedValue(
      envelope([
        makeGroup({ group: 'op', request_count: 8, success_rate: 95 }),
        makeGroup({ group: 'deg', request_count: 8, success_rate: 75 }),
        makeGroup({ group: 'crit', request_count: 8, success_rate: 40 }),
        makeGroup({ group: 'nodata', request_count: 0, success_rate: 0 }),
      ])
    )
    const { container } = renderPage()

    const chip = await screen.findByRole('button', { name: '1 Degraded' })
    expect(screen.getByRole('button', { name: '1 Operational' })).not.toBeNull()
    expect(screen.getByRole('button', { name: '1 Unavailable' })).not.toBeNull()
    expect(container.querySelectorAll('article')).toHaveLength(4)

    await userEvent.setup().click(chip)
    await waitFor(() =>
      expect(container.querySelectorAll('article')).toHaveLength(1)
    )
    expect(screen.getByText('1 of 4 groups')).not.toBeNull()

    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: '1 Degraded' }))
    await waitFor(() =>
      expect(container.querySelectorAll('article')).toHaveLength(4)
    )
    expect(screen.getByText('4 groups')).not.toBeNull()
  })

  test('drops the reference cache rate when the input is out of range', async () => {
    mockedFetch.mockResolvedValue(envelope(healthy(GROUP_NAMES)))
    renderPage()

    const trigger = await screen.findByRole('button', {
      name: 'Reference cache: 90%',
    })
    expect(trigger.getAttribute('aria-invalid')).toBe('false')

    const user = userEvent.setup()
    await user.click(trigger)
    const input = await screen.findByLabelText('Reference cache rate (%)')
    expect(input.getAttribute('aria-invalid')).toBe('false')

    await user.clear(input)
    await user.type(input, '150')

    const invalidTrigger = await screen.findByRole('button', {
      name: 'Reference cache rate',
    })
    expect(invalidTrigger.getAttribute('aria-invalid')).toBe('true')
    expect(input.getAttribute('aria-invalid')).toBe('true')
  })

  test('renders the empty state when the account has no groups', async () => {
    mockedFetch.mockResolvedValue(envelope([]))
    renderPage()

    expect(
      await screen.findByText('You have no usable groups yet.')
    ).not.toBeNull()
  })

  test('renders the failure state and retries the request', async () => {
    mockedFetch.mockRejectedValue(new Error('boom'))
    renderPage()

    expect(
      await screen.findByText('Failed to load performance data.')
    ).not.toBeNull()
    expect(mockedFetch).toHaveBeenCalledTimes(1)

    await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(mockedFetch).toHaveBeenCalledTimes(2))
  })
})
