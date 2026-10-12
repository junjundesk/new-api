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
import { ROLE } from '@/lib/roles'
import { Route as EmailRoute } from '@/routes/_authenticated/email/index'
import { Route as LegacyOperationsRoute } from '@/routes/_authenticated/system-settings/operations/$section'
import { useAuthStore } from '@/stores/auth-store'

import { Email } from '../index'
import { emailSearchSchema } from '../search'

const clients: QueryClient[] = []

async function renderEmail(role: number, entry = '/email') {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'operator', role })
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })
  clients.push(client)
  const root = createRootRoute({ component: Outlet })
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
    component: Outlet,
  })
  const email = createRoute({
    getParentRoute: () => authenticated,
    path: 'email/',
    validateSearch: emailSearchSchema,
    beforeLoad: (context) =>
      EmailRoute.options.beforeLoad?.(
        context as unknown as Parameters<
          NonNullable<typeof EmailRoute.options.beforeLoad>
        >[0]
      ),
    component: Email,
  })
  const forbidden = createRoute({
    getParentRoute: () => root,
    path: '403',
    component: () => <h1>Forbidden</h1>,
  })
  const legacy = createRoute({
    getParentRoute: () => authenticated,
    path: 'system-settings/operations/$section',
    beforeLoad: (context) =>
      LegacyOperationsRoute.options.beforeLoad?.(
        context as unknown as Parameters<
          NonNullable<typeof LegacyOperationsRoute.options.beforeLoad>
        >[0]
      ),
    component: () => <h1>Legacy settings</h1>,
  })
  const router = createRouter({
    routeTree: root.addChildren([
      authenticated.addChildren([email, legacy]),
      forbidden,
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
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [
        { key: 'SMTPServer', value: 'smtp.example.test' },
        { key: 'SMTPPort', value: '587' },
        { key: 'SMTPAccount', value: 'operator@example.test' },
        { key: 'SMTPFrom', value: 'operator@example.test' },
      ],
    },
  })
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

afterEach(() => {
  cleanup()
  for (const client of clients.splice(0)) client.clear()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})

describe('Email Center navigation', () => {
  test('a regular user is denied before email content loads', async () => {
    await renderEmail(ROLE.USER)
    expect(
      await screen.findByRole('heading', { name: 'Forbidden' })
    ).toBeVisible()
    expect(api.get).not.toHaveBeenCalled()
  })

  test('an admin gets only sending and a privileged deep link returns to sending without fetching options', async () => {
    const router = await renderEmail(ROLE.ADMIN, '/email?tab=smtp')
    expect(
      await screen.findByRole('tab', { name: 'Send Email' })
    ).toHaveAttribute('aria-selected', 'true')
    expect(
      screen.queryByRole('tab', { name: 'SMTP Email' })
    ).not.toBeInTheDocument()
    expect(router.state.location.search).toEqual({ tab: 'send' })
    expect(api.get).not.toHaveBeenCalled()
  })

  test('long tab labels can scroll horizontally without a vertical tab-strip scrollbar', async () => {
    await renderEmail(ROLE.SUPER_ADMIN)
    const tabs = await screen.findByRole('tablist', { name: 'Email Center' })
    expect(tabs.parentElement).toHaveClass(
      'overflow-x-auto',
      'overflow-y-hidden'
    )
    expect(
      screen.getByRole('tab', { name: 'Email Notifications' })
    ).toBeVisible()
  })

  test('a root user can reach SMTP by keyboard and navigate back to sending', async () => {
    const router = await renderEmail(ROLE.SUPER_ADMIN)
    const user = userEvent.setup()
    const send = await screen.findByRole('tab', { name: 'Send Email' })
    expect(api.get).not.toHaveBeenCalled()
    act(() => send.focus())
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('tab', { name: 'SMTP Email' })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(
      await screen.findByRole('textbox', { name: 'SMTP Host' })
    ).toHaveValue('smtp.example.test')
    expect(screen.getByRole('tab', { name: 'SMTP Email' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(router.state.location.search).toEqual({ tab: 'smtp' })
    await act(async () => {
      router.history.back()
    })
    expect(
      await screen.findByRole('textbox', { name: 'Subject' })
    ).toBeVisible()
    expect(screen.getByRole('tab', { name: 'Send Email' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  test('an unknown tab falls back to sending without requesting global settings', async () => {
    await renderEmail(ROLE.SUPER_ADMIN, '/email?tab=unknown')
    expect(
      await screen.findByRole('textbox', { name: 'Subject' })
    ).toBeVisible()
    expect(api.get).not.toHaveBeenCalled()
  })

  test('the previous SMTP settings URL redirects to the unified SMTP tab', async () => {
    const router = await renderEmail(
      ROLE.SUPER_ADMIN,
      '/system-settings/operations/email'
    )
    expect(
      await screen.findByRole('textbox', { name: 'SMTP Host' })
    ).toBeVisible()
    expect(router.state.location.pathname.replace(/\/$/, '')).toBe('/email')
    expect(router.state.location.search).toEqual({ tab: 'smtp' })
  })

  test('loading configuration fails closed and retry restores the settings form', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({
      data: { success: false, message: 'Unavailable' },
    })
    await renderEmail(ROLE.SUPER_ADMIN, '/email?tab=smtp')
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Failed to load email settings'
    )
    expect(
      screen.queryByRole('button', { name: 'Save SMTP settings' })
    ).not.toBeInTheDocument()
    await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))
    expect(
      await screen.findByRole('textbox', { name: 'SMTP Host' })
    ).toHaveValue('smtp.example.test')
  })

  test('switching away from an email draft asks for confirmation and Stay preserves the draft', async () => {
    await renderEmail(ROLE.SUPER_ADMIN)
    const user = userEvent.setup()
    await user.type(
      await screen.findByRole('textbox', { name: 'Subject' }),
      'Draft announcement'
    )
    await user.click(screen.getByRole('tab', { name: 'SMTP Email' }))
    expect(await screen.findByRole('alertdialog')).toHaveTextContent(
      'Unsaved changes'
    )
    await user.click(screen.getByRole('button', { name: 'Stay' }))
    expect(screen.getByRole('textbox', { name: 'Subject' })).toHaveValue(
      'Draft announcement'
    )
    expect(screen.getByRole('tab', { name: 'Send Email' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(api.get).not.toHaveBeenCalled()
  })

  test('SMTP edits survive a partial save and a later save can restore the original host', async () => {
    await renderEmail(ROLE.SUPER_ADMIN, '/email?tab=smtp')
    const user = userEvent.setup()
    const host = await screen.findByRole('textbox', { name: 'SMTP Host' })
    await user.clear(host)
    await user.type(host, 'smtp.updated.test')
    const port = screen.getByRole('spinbutton', { name: 'Port' })
    await user.clear(port)
    await user.type(port, '465')
    vi.mocked(api.put)
      .mockResolvedValueOnce({ data: { success: true } })
      .mockResolvedValueOnce({ data: { success: false } })
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: [
          { key: 'SMTPServer', value: 'smtp.updated.test' },
          { key: 'SMTPPort', value: '587' },
        ],
      },
    })
    const save = screen.getByRole('button', { name: 'Save SMTP settings' })
    await user.click(save)
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(save).toBeEnabled())
    expect(host).toHaveValue('smtp.updated.test')
    expect(port).toHaveValue(465)
    await user.click(screen.getByRole('tab', { name: 'Send Email' }))
    expect(await screen.findByRole('alertdialog')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Stay' }))
    await user.click(save)
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(4))
    await waitFor(() => expect(save).toBeEnabled())
    await user.clear(host)
    await user.type(host, 'smtp.example.test')
    await user.click(save)
    await waitFor(() =>
      expect(api.put).toHaveBeenLastCalledWith('/api/option/', {
        key: 'SMTPServer',
        value: 'smtp.example.test',
      })
    )
    await waitFor(() => expect(save).toBeEnabled())
    await user.click(screen.getByRole('tab', { name: 'Send Email' }))
    expect(
      await screen.findByRole('textbox', { name: 'Subject' })
    ).toBeVisible()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  test('saving SMTP from the new entry leaves an empty credential unchanged', async () => {
    await renderEmail(ROLE.SUPER_ADMIN, '/email?tab=smtp')
    const user = userEvent.setup()
    const host = await screen.findByRole('textbox', { name: 'SMTP Host' })
    await user.clear(host)
    await user.type(host, 'smtp.updated.test')
    await user.click(screen.getByRole('button', { name: 'Save SMTP settings' }))
    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith('/api/option/', {
        key: 'SMTPServer',
        value: 'smtp.updated.test',
      })
    )
    expect(api.put).toHaveBeenCalledTimes(1)
  })
})
