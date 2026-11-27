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
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { Route as ChannelsRoute } from '@/routes/_authenticated/channels/index'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import {
  channelSchema,
  type Channel,
  type GetChannelsParams,
} from '../../types'

const i18n = createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
const clients: QueryClient[] = []
const rootUser = { id: 1, username: 'root', role: ROLE.SUPER_ADMIN }
const config = {
  version: 1,
  channels: [
    { name: 'Primary', type: 1, key: 'fixture-primary', models: 'm' },
    {
      name: 'Pool',
      type: 1,
      key: 'fixture-one\nfixture-two',
      models: 'm',
      multi_key_mode: 'polling',
    },
  ],
}

function listChannel(id: number, name: string) {
  return channelSchema.parse({
    id,
    name,
    type: 1,
    key: '',
    status: 1,
    models: 'm',
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
}

function mockChannelList(
  list: (page: number) => { items: Channel[]; total: number }
) {
  vi.spyOn(api, 'get').mockImplementation(async (url, options) => {
    switch (url) {
      case '/api/channel':
        return {
          data: {
            success: true,
            data: list(
              (options?.params as GetChannelsParams | undefined)?.p ?? 1
            ),
          },
        }
      case '/api/group/':
        return { data: { success: true, data: ['default'] } }
      case '/api/channel/ops':
        return { data: { success: true, data: {} } }
      default:
        return { data: { success: true, data: [] } }
    }
  })
}

async function renderChannelPage(authUser: AuthUser) {
  const user = userEvent.setup()
  useAuthStore.getState().auth.setUser(authUser)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)
  const root = createRootRoute({ component: Outlet })
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
    component: Outlet,
  })
  const channels = createRoute({
    getParentRoute: () => authenticated,
    path: 'channels/',
    validateSearch: ChannelsRoute.options.validateSearch,
    component: ChannelsRoute.options.component,
  })
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren([channels])]),
    history: createMemoryHistory({ initialEntries: ['/channels?pageSize=1'] }),
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
  return user
}

afterEach(() => {
  cleanup()
  for (const client of clients.splice(0)) client.clear()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})

test('checking channels on different pages exports exactly those IDs and only reveals credentials after confirmation', async () => {
  mockChannelList((page) => ({
    items: [page === 2 ? listChannel(27, 'Pool') : listChannel(14, 'Primary')],
    total: 3,
  }))
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: config } })
  const user = await renderChannelPage(rootUser)
  await user.click(
    await screen.findByRole('switch', { name: 'Batch Operations' })
  )
  expect(screen.getByRole('button', { name: 'Export JSON' })).toBeDisabled()
  await user.click(await screen.findByRole('checkbox', { name: 'Select row' }))
  expect(screen.getByRole('button', { name: 'Export JSON (1)' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: 'Go to next page' }))
  await screen.findByText('Pool')
  await user.click(screen.getByRole('checkbox', { name: 'Select row' }))
  await user.click(screen.getByRole('button', { name: 'Export JSON (2)' }))
  const dialog = within(
    screen.getByRole('dialog', { name: 'Export selected channels' })
  )
  expect(
    dialog.getByText(/2 selected channels includes plaintext credentials/)
  ).toBeVisible()
  expect(dialog.queryByRole('textbox')).not.toBeInTheDocument()
  expect(post).not.toHaveBeenCalled()
  const generate = dialog.getByRole('button', { name: 'Generate JSON' })
  generate.focus()
  await user.keyboard('{Enter}')
  const output = await dialog.findByRole<HTMLTextAreaElement>('textbox', {
    name: 'Channel JSON',
  })
  expect(output).toHaveAttribute('readonly')
  expect(JSON.parse(output.value)).toEqual(config)
  expect(post.mock.lastCall?.slice(0, 2)).toEqual([
    '/api/channel/export',
    { ids: [14, 27] },
  ])
  await user.click(dialog.getByRole('button', { name: 'Copy JSON' }))
  expect(await navigator.clipboard.readText()).toBe(output.value)
  await user.keyboard('{Escape}')
  await user.click(screen.getByRole('button', { name: 'Export JSON (2)' }))
  expect(
    screen.queryByRole('textbox', { name: 'Channel JSON' })
  ).not.toBeInTheDocument()
})

test('invalid JSON stays editable, a server rejection preserves the draft, and a successful import refreshes the channel list', async () => {
  let imported = false
  mockChannelList(() => ({
    items: imported ? [listChannel(12, 'Primary')] : [],
    total: imported ? 2 : 0,
  }))
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Invalid settings for channel #2' },
    })
    .mockImplementationOnce(async () => {
      imported = true
      return { data: { success: true, data: 2 } }
    })
  const user = await renderChannelPage({
    id: 2,
    username: 'editor',
    role: ROLE.ADMIN,
    permissions: { admin_permissions: { channel: { sensitive_write: true } } },
  })
  await user.click(await screen.findByRole('button', { name: 'Import JSON' }))
  const dialog = within(screen.getByRole('dialog', { name: 'Import channels' }))
  const input = dialog.getByRole('textbox', { name: 'Channel JSON' })
  const submit = dialog.getByRole('button', { name: 'Confirm import' })
  expect(submit).toBeDisabled()
  await user.click(input)
  await user.paste('{')
  await user.click(submit)
  expect(input).toHaveAttribute('aria-invalid', 'true')
  expect(dialog.getByRole('alert')).toHaveTextContent('Invalid JSON format')
  await user.clear(input)
  await user.paste('{"version":1,"channels":[]}')
  await user.click(submit)
  expect(dialog.getByRole('alert')).toHaveTextContent('1 to 100 channels')
  expect(post).not.toHaveBeenCalled()
  await user.clear(input)
  await user.paste(JSON.stringify(config))
  await user.click(submit)
  expect(await dialog.findByRole('alert')).toHaveTextContent(
    'Invalid settings for channel #2'
  )
  expect(input).toHaveValue(JSON.stringify(config))
  await user.click(submit)
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(await screen.findByText('Primary')).toBeVisible()
  expect(post.mock.lastCall?.slice(0, 2)).toEqual([
    '/api/channel/import',
    config,
  ])
})

test('an admin without credential or creation access cannot export checked rows or import JSON', async () => {
  mockChannelList(() => ({ items: [listChannel(7, 'Read only')], total: 1 }))
  const user = await renderChannelPage({
    id: 3,
    username: 'reader',
    role: ROLE.ADMIN,
  })
  expect(
    await screen.findByRole('button', { name: 'Import JSON' })
  ).toBeDisabled()
  await user.click(screen.getByRole('switch', { name: 'Batch Operations' }))
  await user.click(await screen.findByRole('checkbox', { name: 'Select row' }))
  expect(screen.getByRole('button', { name: 'Export JSON (1)' })).toBeDisabled()
})
