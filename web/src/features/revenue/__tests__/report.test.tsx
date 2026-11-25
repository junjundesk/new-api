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
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { RevenuePage } from '..'
import type { RevenueReport } from '../api'

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
const original = useSystemConfigStore.getState().config
const clients: QueryClient[] = []
const empty: RevenueReport = {
  as_of: 100,
  days: [],
  items: [],
  summary: {
    total: '0',
    cost: '0',
    profit: '0',
    unpriced: '0',
    online: '0',
    admin: '0',
    redeemed: '0',
    credited: '0',
  },
}
function renderReport() {
  useSystemConfigStore
    .getState()
    .setConfig({
      currency: {
        displayInCurrency: true,
        quotaDisplayType: 'USD',
        quotaPerUnit: 100,
        usdExchangeRate: 1,
        customCurrencySymbol: '¤',
        customCurrencyExchangeRate: 1,
      },
    })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <RevenuePage />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return userEvent.setup()
}
afterEach(() => {
  cleanup()
  clients.splice(0).forEach((c) => c.clear())
  vi.restoreAllMocks()
  useSystemConfigStore.setState({ config: original })
})

test('only today is selected initially and switching ranges replaces the daily rows and inflow totals', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (_url, config) => {
    const params = config?.params as { range?: string } | undefined
    if (!params?.range) throw new Error('Expected an explicit report range')
    const range = params.range
    if (range === 'today') return { data: { success: true, data: empty } }
    const dates: Record<string, string> = {
      yesterday: '2026-01-01',
      '7d': '2025-12-27',
      '30d': '2025-12-04',
      all: '2024-01-01',
    }
    const totals = {
      ...empty.summary,
      online: '200',
      admin: '300',
      credited: '500',
    }
    return {
      data: {
        success: true,
        data: {
          summary: totals,
          days: [{ ...totals, date: dates[range] }],
          items: [],
          as_of: 100,
        },
      },
    }
  })
  const user = renderReport()
  expect(
    await screen.findByText('No usage or wallet credits in this period.')
  ).toBeVisible()
  expect(
    screen.getByRole('button', { name: 'Today (live)', pressed: true })
  ).toBeVisible()
  for (const [label, date] of [
    ['Yesterday', '2026-01-01'],
    ['Last 7 days', '2025-12-27'],
    ['Last 30 days', '2025-12-04'],
    ['All time', '2024-01-01'],
  ]) {
    await user.click(screen.getByRole('button', { name: label }))
    const row = await screen.findByRole('row', { name: new RegExp(date) })
    expect(
      screen.getByRole('button', { name: label, pressed: true })
    ).toBeVisible()
    expect(within(row).getByText('$3')).toBeVisible()
    expect(within(row).getByText('$5')).toBeVisible()
  }
  await user.click(screen.getByRole('button', { name: 'Today (live)' }))
  expect(
    await screen.findByText('No usage or wallet credits in this period.')
  ).toBeVisible()
})

test('an invalid model multiplier cannot save, while an explicit zero override persists independently of the group rate', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/groups')
        ? { available_groups: ['vip'], groups: [] }
        : empty,
    },
  }))
  const save = vi
    .spyOn(api, 'put')
    .mockImplementation(async (_url, body) => ({
      data: { success: true, data: body },
    }))
  const user = renderReport()
  await user.click(screen.getByRole('button', { name: 'Bind cost groups' }))
  const dialog = within(await screen.findByRole('dialog'))
  await user.click(
    await dialog.findByRole('button', { name: 'Add group binding' })
  )
  await user.type(dialog.getByLabelText('Bound usage group'), 'vip')
  const rate = dialog.getByLabelText('Group cost multiplier')
  await user.clear(rate)
  await user.type(rate, '0.6')
  await user.click(dialog.getByRole('button', { name: 'Add model override' }))
  await user.type(dialog.getByLabelText('Model ID'), 'preview')
  const override = dialog.getByLabelText('Model cost multiplier')
  await user.type(override, '-1')
  await user.click(dialog.getByRole('button', { name: 'Save cost groups' }))
  expect(await dialog.findByRole('alert')).toBeVisible()
  expect(save).not.toHaveBeenCalled()
  await user.clear(override)
  await user.type(override, '0')
  await user.click(dialog.getByRole('button', { name: 'Save cost groups' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(save).toHaveBeenCalledWith('/api/revenue/groups', {
    groups: [
      {
        group: 'vip',
        cost_ratio: '0.6',
        models: [{ model: 'preview', cost_ratio: '0' }],
      },
    ],
  })
})

test('a failed read is reported instead of presenting zeros as a successful daily report', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({
      data: { success: false, message: 'storage unavailable' },
    })
    .mockResolvedValue({ data: { success: true, data: empty } })
  const user = renderReport()
  expect(await screen.findByRole('alert')).toHaveTextContent('Unable to load')
  expect(screen.queryByText('Total wallet inflows')).toBeNull()
  await user.click(screen.getByRole('button', { name: 'Refresh' }))
  expect(
    await screen.findByText('No usage or wallet credits in this period.')
  ).toBeVisible()
})
