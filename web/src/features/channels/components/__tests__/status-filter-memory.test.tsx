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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { Route as ChannelsRoute } from '@/routes/_authenticated/channels/index'

import { ChannelsProvider } from '../channels-provider'
import { ChannelsTable } from '../channels-table'

const STATUS_FILTER_STORAGE_KEY = 'channel-status-filter'

const clients: QueryClient[] = []

async function renderChannelsTable(entry: string) {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)

  const rootRoute = createRootRoute({ component: Outlet })
  const authenticatedRoute = createRoute({
    getParentRoute: () => rootRoute,
    id: '_authenticated',
    component: Outlet,
  })
  const channelsRoute = createRoute({
    getParentRoute: () => authenticatedRoute,
    path: 'channels/',
    validateSearch: ChannelsRoute.options.validateSearch,
    component: () => (
      <ChannelsProvider>
        <ChannelsTable />
      </ChannelsProvider>
    ),
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      authenticatedRoute.addChildren([channelsRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: [entry] }),
  })

  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <RouterProvider router={router} />
      </I18nextProvider>
    </QueryClientProvider>
  )
  await act(async () => {
    await router.load()
  })
  return router
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockImplementation(((url: string) => {
    if (url === '/api/group/') {
      return Promise.resolve({
        data: { success: true, data: ['default', 'vip'] },
      })
    }
    return Promise.resolve({
      data: { success: true, data: { items: [], total: 0, type_counts: {} } },
    })
  }) as never)
})

afterEach(() => {
  cleanup()
  for (const client of clients.splice(0)) client.clear()
  vi.restoreAllMocks()
})

describe('channels status filter memory', () => {
  test('changing the group after arriving with an explicit status keeps the remembered status', async () => {
    localStorage.setItem(STATUS_FILTER_STORAGE_KEY, 'enabled')
    const router = await renderChannelsTable(
      `/channels?group=${encodeURIComponent('["vip"]')}&status=${encodeURIComponent('["all"]')}`
    )
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: /^Group\b/ }))
    await user.click(await screen.findByRole('option', { name: 'default' }))

    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({
        group: ['default'],
      })
    )
    expect(localStorage.getItem(STATUS_FILTER_STORAGE_KEY)).toBe('enabled')
  })

  test('choosing a status remembers it for the next visit', async () => {
    const router = await renderChannelsTable('/channels')
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: /^Status\b/ }))
    await user.click(await screen.findByRole('option', { name: 'Disabled' }))

    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({
        status: ['disabled'],
      })
    )
    expect(localStorage.getItem(STATUS_FILTER_STORAGE_KEY)).toBe('disabled')
  })
})
