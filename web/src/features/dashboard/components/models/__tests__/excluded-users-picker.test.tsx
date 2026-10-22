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
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import type { ExcludedUser } from '@/features/dashboard/types'

import { ExcludedUsersPicker } from '../excluded-users-picker'

vi.mock('@/features/users/api', () => ({
  searchUsers: vi.fn(),
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
  }),
}))

const { searchUsers } = await import('@/features/users/api')

interface SearchUsersMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<[{ keyword?: string }]> }
}

const mockedSearch = searchUsers as unknown as SearchUsersMock

function searchPayload(items: Array<{ id: number; username: string }>) {
  return {
    success: true,
    data: {
      items: items.map((item) => ({
        ...item,
        display_name: item.username,
        quota: 0,
        used_quota: 0,
        request_count: 0,
        group: 'default',
        status: 1,
        role: 1,
      })),
      total: items.length,
      page: 1,
      page_size: 20,
    },
  }
}

function Harness(props: { initial?: ExcludedUser[] }) {
  const [selected, setSelected] = useState<ExcludedUser[]>(props.initial ?? [])
  return <ExcludedUsersPicker selected={selected} onChange={setSelected} />
}

function renderPicker(initial?: ExcludedUser[]) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, refetchOnWindowFocus: false },
    },
  })
  return render(
    <QueryClientProvider client={client}>
      <Harness initial={initial} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  mockedSearch.mockReset()
})

afterEach(() => {
  cleanup()
})

describe('ExcludedUsersPicker', () => {
  test('searching then picking a user shows it as an applied chip', async () => {
    mockedSearch.mockResolvedValue(
      searchPayload([
        { id: 10, username: 'admin' },
        { id: 2, username: 'bob' },
      ])
    )

    renderPicker()
    await userEvent.type(
      screen.getByRole('textbox', { name: 'Search users to exclude' }),
      'adm'
    )

    const adminOption = await screen.findByRole('button', { name: /admin/ })
    await userEvent.click(adminOption)

    await waitFor(() =>
      expect(mockedSearch.mock.calls.length).toBeGreaterThan(0)
    )
    expect(mockedSearch.mock.calls.at(-1)?.[0].keyword).toBe('adm')
    expect(screen.getByRole('button', { name: 'Remove admin' })).not.toBeNull()
  })

  test('removing a selected user drops it from the applied list', async () => {
    mockedSearch.mockResolvedValue(searchPayload([]))

    renderPicker([{ id: 10, username: 'admin' }])

    await userEvent.click(screen.getByRole('button', { name: 'Remove admin' }))

    expect(screen.queryByRole('button', { name: 'Remove admin' })).toBeNull()
  })

  test('shows an empty state when the search has no matches', async () => {
    mockedSearch.mockResolvedValue(searchPayload([]))

    renderPicker()
    await userEvent.type(
      screen.getByRole('textbox', { name: 'Search users to exclude' }),
      'nobody'
    )

    expect(await screen.findByText('No users found')).not.toBeNull()
  })
})
