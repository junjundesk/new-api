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
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from '@tanstack/react-query'
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useState } from 'react'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { getRedemptionCategories } from '../../api'
import type { RedemptionCategory } from '../../types'
import { RedemptionCategoryPanel } from '../redemption-category-panel'
import { RedemptionsProvider } from '../redemptions-provider'

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
const tier = { id: 7, name: 'Standard', quota: 50000000, bonus_rate_bps: 200 }
const previousConfig = useSystemConfigStore.getState().config

function CategoryNavigation() {
  const [selected, select] = useState<number>()
  const categories = useQuery({
    queryKey: ['redemption-categories'],
    queryFn: getRedemptionCategories,
  })
  return (
    <RedemptionsProvider categories={categories.data} categoryId={selected}>
      <RedemptionCategoryPanel
        isLoading={categories.isLoading}
        isError={categories.isError}
        retry={() => void categories.refetch()}
        onSelect={select}
      />
    </RedemptionsProvider>
  )
}

function renderCategories() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  useSystemConfigStore.getState().setConfig({
    currency: {
      displayInCurrency: true,
      quotaDisplayType: 'USD',
      quotaPerUnit: 500000,
      usdExchangeRate: 1,
      customCurrencySymbol: '¤',
      customCurrencyExchangeRate: 1,
    },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <CategoryNavigation />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return userEvent.setup()
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  useSystemConfigStore.setState({ config: previousConfig })
})

test('creating a tier previews its bonus and selects the saved category in both navigation controls', async () => {
  let categories: RedemptionCategory[] = []
  vi.spyOn(api, 'get').mockImplementation(async () => ({
    data: { success: true, data: categories },
  }))
  const save = vi.spyOn(api, 'post').mockImplementation(async () => {
    categories = [tier]
    return { data: { success: true, data: tier } }
  })
  const user = renderCategories()
  await screen.findByText('No categories yet')
  await user.click(screen.getByRole('button', { name: 'New category' }))
  const dialog = within(screen.getByRole('dialog'))
  await user.type(dialog.getByLabelText('Category name'), 'Standard')
  const percent = dialog.getByLabelText('Bonus rate (%)')
  await user.clear(percent)
  await user.type(percent, '2')
  expect(dialog.getByRole('status').textContent).toBe(formatQuota(1000000))
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(save.mock.calls[0]?.[1]).toMatchObject({
    name: 'Standard',
    quota: 50000000,
    bonus_rate_bps: 200,
  })
  expect(
    screen.getByRole('button', { name: /Standard/, pressed: true })
  ).toBeDefined()
  const select = screen.getByRole<HTMLSelectElement>('combobox', {
    name: 'Code categories',
  })
  expect(select.value).toBe('7')
  await user.selectOptions(select, '0')
  expect(
    screen.getByRole('button', { name: 'Uncategorized', pressed: true })
  ).toBeDefined()
})

test('a rejected rate change keeps the form open and a successful retry can turn the bonus off', async () => {
  let saved = tier
  vi.spyOn(api, 'get').mockImplementation(async () => ({
    data: { success: true, data: [saved] },
  }))
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Please retry' },
    })
    .mockImplementation(async () => {
      saved = { ...tier, bonus_rate_bps: 0 }
      return { data: { success: true, data: saved } }
    })
  const user = renderCategories()
  await user.click(await screen.findByRole('button', { name: 'Edit Standard' }))
  const dialog = within(screen.getByRole('dialog'))
  expect(
    dialog.getByLabelText<HTMLInputElement>('Face value (USD)').readOnly
  ).toBe(true)
  await user.clear(dialog.getByLabelText('Bonus rate (%)'))
  await user.type(dialog.getByLabelText('Bonus rate (%)'), '0')
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  expect((await dialog.findByRole('alert')).textContent).toBe('Please retry')
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(update.mock.lastCall?.[0]).toBe('/api/redemption/categories/7')
  expect(update.mock.lastCall?.[1]).toMatchObject({
    quota: 50000000,
    bonus_rate_bps: 0,
  })
  expect(
    screen.getByRole('button', { name: /Standard/, pressed: true }).textContent
  ).toContain('+0%')
})
