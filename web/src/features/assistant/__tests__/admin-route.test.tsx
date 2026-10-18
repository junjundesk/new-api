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
import type { ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { Route as AdminAssistantRoute } from '@/routes/_authenticated/assistant/admin/index'
import { useAuthStore } from '@/stores/auth-store'

vi.mock('sse.js', () => ({ SSE: class {} }))

// The generated route tree types the real route against its parent router
// context, which this standalone test router cannot reproduce. Only the guard
// and component are borrowed; both are still the real route implementation.
const adminAssistantOptions = AdminAssistantRoute.options as unknown as {
  beforeLoad?: () => unknown
  component?: () => ReactNode
}

const configMock = vi.hoisted(() => ({ getAssistantConfig: vi.fn() }))
vi.mock('@/features/assistant/api', () => configMock)

const clients: QueryClient[] = []

async function renderAdminRoute(role?: number) {
  if (role === undefined) {
    useAuthStore.getState().auth.reset()
  } else {
    useAuthStore.getState().auth.setUser({ id: 7, username: 'root', role })
  }

  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })

  const root = createRootRoute({ component: Outlet })
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
    component: Outlet,
  })
  const adminAssistant = createRoute({
    getParentRoute: () => authenticated,
    path: 'assistant/admin/',
    beforeLoad: () => {
      // Run the real route guard so a regression in beforeLoad fails here.
      if (!adminAssistantOptions.beforeLoad) {
        throw new Error('Admin assistant route has no guard')
      }
      adminAssistantOptions.beforeLoad()
    },
    component: adminAssistantOptions.component,
  })
  const forbidden = createRoute({
    getParentRoute: () => root,
    path: '403',
    component: () => <h1>Forbidden</h1>,
  })
  const router = createRouter({
    routeTree: root.addChildren([
      authenticated.addChildren([adminAssistant]),
      forbidden,
    ]),
    history: createMemoryHistory({ initialEntries: ['/assistant/admin'] }),
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
  configMock.getAssistantConfig.mockReset()
  configMock.getAssistantConfig.mockResolvedValue({
    enabled: true,
    model: 'admin-gpt-test',
    canCreateTokens: true,
    maxMessageChars: 4000,
    maxHistoryMessages: 20,
  })
})

afterEach(() => {
  cleanup()
  for (const client of clients.splice(0)) client.clear()
  useAuthStore.getState().auth.reset()
})

describe('admin assistant route guard', () => {
  it('redirects a regular user to 403 before the admin config loads', async () => {
    await renderAdminRoute(ROLE.USER)

    expect(
      await screen.findByRole('heading', { name: 'Forbidden' })
    ).toBeVisible()
    expect(configMock.getAssistantConfig).not.toHaveBeenCalled()
  })

  it('redirects a non-root admin to 403', async () => {
    await renderAdminRoute(ROLE.ADMIN)

    expect(
      await screen.findByRole('heading', { name: 'Forbidden' })
    ).toBeVisible()
    expect(configMock.getAssistantConfig).not.toHaveBeenCalled()
  })

  it('redirects a guest to 403', async () => {
    await renderAdminRoute()

    expect(
      await screen.findByRole('heading', { name: 'Forbidden' })
    ).toBeVisible()
    expect(configMock.getAssistantConfig).not.toHaveBeenCalled()
  })

  it('lets a super admin reach the admin assistant page', async () => {
    const router = await renderAdminRoute(ROLE.SUPER_ADMIN)

    expect(
      await screen.findByRole('heading', { name: 'Admin Assistant' })
    ).toBeVisible()
    expect(
      await screen.findByText(
        "This assistant uses the administrator's separate assistant profile."
      )
    ).toBeVisible()
    expect(configMock.getAssistantConfig).toHaveBeenCalledWith('admin')
    expect(router.state.location.pathname).toBe('/assistant/admin')
  })
})
