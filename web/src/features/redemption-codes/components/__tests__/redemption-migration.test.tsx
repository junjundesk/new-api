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
import {
  act,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { toast, Toaster } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { Route } from '@/routes/_authenticated/redemption-codes/index'

import { Redemptions } from '../../index'
import type { Redemption, RedemptionCategory } from '../../types'

const tier = { id: 7, name: 'Standard', quota: 5000000 }
const i18n = createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })

async function renderPage(
  options: {
    entry?: string
    categories?: RedemptionCategory[]
    codes?: () => Redemption[]
  } = {}
) {
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/redemption/categories') {
      return { data: { success: true, data: options.categories ?? [tier] } }
    }
    if (url === '/api/redemption/bonus-rules') {
      return { data: { success: true, data: [] } }
    }
    const id = new URL(url, 'https://example.test').searchParams.get(
      'category_id'
    )
    const items = (options.codes?.() ?? []).filter(
      (code) => id === null || code.category_id === Number(id)
    )
    return { data: { success: true, data: { items, total: items.length } } }
  })
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })
  const root = createRootRoute({ component: Outlet })
  const auth = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
    component: Outlet,
  })
  const page = createRoute({
    getParentRoute: () => auth,
    path: 'redemption-codes/',
    validateSearch: Route.options.validateSearch,
    component: Redemptions,
  })
  const router = createRouter({
    routeTree: root.addChildren([auth.addChildren([page])]),
    history: createMemoryHistory({
      initialEntries: [options.entry ?? '/redemption-codes?category_id=0'],
    }),
  })
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <RouterProvider router={router} />
        <Toaster />
      </I18nextProvider>
    </QueryClientProvider>
  )
  await act(async () => {
    await router.load()
  })
  return userEvent.setup()
}

afterEach(() => {
  cleanup()
  toast.dismiss()
  vi.restoreAllMocks()
})

test('confirming the default exact rule refreshes Uncategorized and reports the migration result', async () => {
  const code: Redemption = {
    id: 12,
    key: 'legacy',
    name: 'Legacy ten',
    quota: tier.quota,
    category_id: 0,
    status: 1,
    user_id: 1,
    used_user_id: 0,
    created_time: 1,
    expired_time: 0,
    redeemed_time: 0,
  }
  const post = vi.spyOn(api, 'post').mockImplementation(async () => {
    code.category_id = tier.id
    return { data: { success: true, data: { migrated: 1, unmatched: 0 } } }
  })
  const user = await renderPage({
    entry: '/redemption-codes',
    codes: () => [code],
  })
  expect(
    screen.queryByRole('button', { name: 'Migrate codes' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Uncategorized' }))
  await screen.findAllByText(code.name)
  await user.click(await screen.findByRole('button', { name: 'Migrate codes' }))
  const dialog = within(
    screen.getByRole('alertdialog', { name: 'Migrate uncategorized codes' })
  )
  expect(
    dialog.getByRole('checkbox', { name: 'Allow fuzzy matching (round down)' })
  ).not.toBeChecked()
  await user.click(dialog.getByRole('button', { name: 'Confirm migration' }))
  expect(post.mock.lastCall?.slice(0, 2)).toEqual([
    '/api/redemption/migrate-uncategorized',
    { mode: 'exact' },
  ])
  expect(
    await screen.findByText('Redemption code migration: 1 moved, 0 unmatched')
  ).toBeVisible()
  await waitFor(() => expect(screen.queryAllByText(code.name)).toHaveLength(0))
  await user.click(screen.getByRole('button', { name: /^Standard/ }))
  await screen.findAllByText(code.name)
  expect(
    screen.queryByRole('button', { name: 'Migrate codes' })
  ).not.toBeInTheDocument()
})

test('opting into rounding is reversible until confirmation and resets when cancelled', async () => {
  const post = vi.spyOn(api, 'post')
  const user = await renderPage()
  const trigger = screen.getByRole('button', { name: 'Migrate codes' })
  await user.click(trigger)
  const dialog = within(screen.getByRole('alertdialog'))
  const checkbox = dialog.getByRole('checkbox', {
    name: 'Allow fuzzy matching (round down)',
  })
  checkbox.focus()
  await user.keyboard('[Space]')
  expect(checkbox).toBeChecked()
  await user.click(dialog.getByRole('button', { name: 'Cancel' }))
  await user.click(trigger)
  expect(
    screen.getByRole('checkbox', { name: 'Allow fuzzy matching (round down)' })
  ).not.toBeChecked()
  expect(post).not.toHaveBeenCalled()
})

test('a failed fuzzy migration keeps the rule selected for retry and cannot be submitted twice while pending', async () => {
  let finish!: () => void
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  const post = vi
    .spyOn(api, 'post')
    .mockImplementationOnce(async () => {
      await pending
      return { data: { success: false, message: 'Please retry' } }
    })
    .mockResolvedValueOnce({
      data: { success: true, data: { migrated: 0, unmatched: 2 } },
    })
  const user = await renderPage()
  await user.click(screen.getByRole('button', { name: 'Migrate codes' }))
  const dialog = within(screen.getByRole('alertdialog'))
  await user.click(
    dialog.getByRole('checkbox', { name: 'Allow fuzzy matching (round down)' })
  )
  await user.click(dialog.getByRole('button', { name: 'Confirm migration' }))
  expect(dialog.getByRole('button', { name: 'Processing...' })).toBeDisabled()
  expect(dialog.getByRole('button', { name: 'Cancel' })).toBeDisabled()
  expect(dialog.getByRole('checkbox')).toHaveAttribute('aria-disabled', 'true')
  finish()
  expect(await dialog.findByRole('alert')).toHaveTextContent('Please retry')
  expect(dialog.getByRole('checkbox')).toBeChecked()
  await user.click(dialog.getByRole('button', { name: 'Confirm migration' }))
  expect(post.mock.calls.map((call) => call[1])).toEqual([
    { mode: 'fuzzy' },
    { mode: 'fuzzy' },
  ])
  expect(
    await screen.findByText('Redemption code migration: 0 moved, 2 unmatched')
  ).toBeVisible()
})

test('without a destination category the dialog explains what is missing and disables migration', async () => {
  const user = await renderPage({ categories: [] })
  await user.click(screen.getByRole('button', { name: 'Migrate codes' }))
  const dialog = within(screen.getByRole('alertdialog'))
  expect(
    dialog.getByText('Create a category before migrating codes.')
  ).toBeVisible()
  expect(
    dialog.getByRole('button', { name: 'Confirm migration' })
  ).toBeDisabled()
})
