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
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { act, cleanup, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'

import { useSidebarView } from '@/hooks/use-sidebar-view'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

function SidebarProbe() {
  const { navGroups } = useSidebarView()
  return (
    <ul>
      {navGroups.flatMap((group) =>
        group.items.map((item) => (
          <li key={`${group.id ?? group.title}:${item.title}`}>{item.title}</li>
        ))
      )}
    </ul>
  )
}

const clients: QueryClient[] = []

async function renderSidebar(role: number, assistantEnabled: boolean) {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'operator', role })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)
  client.setQueryData(['status'], { assistant_enabled: assistantEnabled })
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })

  const root = createRootRoute({ component: Outlet })
  const index = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: SidebarProbe,
  })
  const router = createRouter({
    routeTree: root.addChildren([index]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
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
}

afterEach(() => {
  cleanup()
  for (const client of clients.splice(0)) client.clear()
  useAuthStore.getState().auth.reset()
})

describe('admin assistant sidebar entry', () => {
  it('shows the admin assistant to a root user even while the user assistant is disabled', async () => {
    await renderSidebar(ROLE.SUPER_ADMIN, false)

    expect(screen.getByText('Admin Assistant')).toBeVisible()
    expect(screen.queryByText('AI Assistant')).toBeNull()
  })

  it('shows both assistant entries to a root user when the user assistant is enabled', async () => {
    await renderSidebar(ROLE.SUPER_ADMIN, true)

    expect(screen.getByText('Admin Assistant')).toBeVisible()
    expect(screen.getByText('AI Assistant')).toBeVisible()
  })

  it('hides the admin assistant from a non-root admin', async () => {
    await renderSidebar(ROLE.ADMIN, true)

    expect(screen.queryByText('Admin Assistant')).toBeNull()
    expect(screen.getByText('AI Assistant')).toBeVisible()
  })

  it('hides the admin assistant from a regular user', async () => {
    await renderSidebar(ROLE.USER, true)

    expect(screen.queryByText('Admin Assistant')).toBeNull()
    expect(screen.getByText('AI Assistant')).toBeVisible()
  })
})
