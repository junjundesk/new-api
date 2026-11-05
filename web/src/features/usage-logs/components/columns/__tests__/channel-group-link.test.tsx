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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { flexRender } from '@tanstack/react-table'
import { act, cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, test } from 'vitest'

import type { UsageLog } from '../../../data/schema'
import {
  UsageLogsProvider,
  useUsageLogsContext,
} from '../../usage-logs-provider'
import { useCommonLogsColumns } from '../common-logs-columns'

const LOG_TYPE_CONSUME = 2
const LOG_TYPE_ERROR = 5

// A consume log served by channel #7, which belongs to both default and vip.
const VIP_CONSUME_LOG: Partial<UsageLog> = {
  type: LOG_TYPE_CONSUME,
  channel: 7,
  channel_name: 'primary-openai',
  channel_groups: ['default', 'vip'],
  group: 'vip',
  other: '',
}

function HideSensitiveValuesButton() {
  const { setSensitiveVisible } = useUsageLogsContext()
  return (
    <button type='button' onClick={() => setSensitiveVisible(false)}>
      Hide sensitive values
    </button>
  )
}

async function renderChannelCell(log: Partial<UsageLog>) {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })

  function ChannelCellProbe() {
    const columns = useCommonLogsColumns(true)
    const channelColumn = columns.find((column) => column.id === 'channel')
    return (
      <>
        <HideSensitiveValuesButton />
        {flexRender(channelColumn?.cell, { row: { original: log } } as never)}
      </>
    )
  }

  const rootRoute = createRootRoute({ component: Outlet })
  const logsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: ChannelCellProbe,
  })
  const channelsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: 'channels',
    component: () => <h1>Channels</h1>,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([logsRoute, channelsRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })

  render(
    <I18nextProvider i18n={i18n}>
      <UsageLogsProvider>
        <RouterProvider router={router} />
      </UsageLogsProvider>
    </I18nextProvider>
  )
  await act(async () => {
    await router.load()
  })
  return router
}

afterEach(() => {
  cleanup()
})

describe('usage log channel group link', () => {
  test('following the link opens the full channel list of the log group without a channel or status filter', async () => {
    const router = await renderChannelCell(VIP_CONSUME_LOG)

    await userEvent
      .setup()
      .click(
        await screen.findByRole('link', { name: 'View channels in group vip' })
      )

    expect(
      await screen.findByRole('heading', { name: 'Channels' })
    ).toBeVisible()
    expect(router.state.location.pathname).toBe('/channels')
    expect(router.state.location.search).toEqual({
      group: ['vip'],
      status: ['all'],
    })
  })

  test('pressing Enter on the focused link opens the group channel list', async () => {
    const router = await renderChannelCell(VIP_CONSUME_LOG)
    const link = await screen.findByRole('link', {
      name: 'View channels in group vip',
    })

    act(() => link.focus())
    expect(link).toHaveFocus()
    await userEvent.setup().keyboard('{Enter}')

    expect(
      await screen.findByRole('heading', { name: 'Channels' })
    ).toBeVisible()
    expect(router.state.location.search).toEqual({
      group: ['vip'],
      status: ['all'],
    })
  })

  test('the link sits on the channel name line', async () => {
    await renderChannelCell(VIP_CONSUME_LOG)

    const link = await screen.findByRole('link', {
      name: 'View channels in group vip',
    })

    expect(link.parentElement).toHaveTextContent('primary-openai')
    expect(link.parentElement).not.toHaveTextContent('#7')
  })

  test('an error log recording a chain name links a group the channel belongs to', async () => {
    const router = await renderChannelCell({
      type: LOG_TYPE_ERROR,
      channel: 7,
      channel_name: 'primary-openai',
      channel_groups: ['vip'],
      group: 'customer-priority',
      other: '',
    })

    await userEvent.setup().click(
      await screen.findByRole('link', {
        name: 'View channels in group vip',
      })
    )

    expect(
      await screen.findByRole('heading', { name: 'Channels' })
    ).toBeVisible()
    expect(router.state.location.search).toEqual({
      group: ['vip'],
      status: ['all'],
    })
  })

  test('a channel without known groups shows its name without a link', async () => {
    await renderChannelCell({ ...VIP_CONSUME_LOG, channel_groups: undefined })

    expect(await screen.findByText('primary-openai')).toBeVisible()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  test('hiding sensitive values removes the link so the group is not exposed', async () => {
    await renderChannelCell(VIP_CONSUME_LOG)
    await screen.findByRole('link', { name: 'View channels in group vip' })

    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Hide sensitive values' }))

    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
