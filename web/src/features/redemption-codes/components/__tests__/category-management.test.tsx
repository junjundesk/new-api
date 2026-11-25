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

import { getRedemptionBonusRules, getRedemptionCategories } from '../../api'
import type { RedemptionCategory } from '../../types'
import { RedemptionBonusRulesDialog } from '../redemption-bonus-rules-dialog'
import { RedemptionCategoryPanel } from '../redemption-category-panel'
import { RedemptionsProvider } from '../redemptions-provider'

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
const tier = { id: 7, name: 'Standard', quota: 50000000 }
const previousConfig = useSystemConfigStore.getState().config

function CategoryNavigation() {
  const [selected, select] = useState<number>()
  const categories = useQuery({
    queryKey: ['redemption-categories'],
    queryFn: getRedemptionCategories,
  })
  const bonusRules = useQuery({
    queryKey: ['redemption-bonus-rules'],
    queryFn: getRedemptionBonusRules,
  })
  const [showRules, setShowRules] = useState(false)
  return (
    <RedemptionsProvider
      categories={categories.data}
      bonusRules={bonusRules.data}
      categoryId={selected}
    >
      <RedemptionCategoryPanel
        isLoading={categories.isLoading}
        isError={categories.isError}
        retry={() => void categories.refetch()}
        onSelect={select}
      />
      <button type='button' onClick={() => setShowRules(true)}>
        Open bonus rules
      </button>
      {showRules && (
        <RedemptionBonusRulesDialog onClose={() => setShowRules(false)} />
      )}
    </RedemptionsProvider>
  )
}

function mockGet(data: {
  categories: () => RedemptionCategory[]
  rules?: () => unknown[]
}) {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.includes('bonus-rules')
        ? (data.rules?.() ?? [])
        : data.categories(),
    },
  }))
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

test('creating a category selects it in both navigation controls without asking for a bonus', async () => {
  let categories: RedemptionCategory[] = []
  mockGet({ categories: () => categories })
  const save = vi.spyOn(api, 'post').mockImplementation(async () => {
    categories = [tier]
    return { data: { success: true, data: tier } }
  })
  const user = renderCategories()
  await screen.findByText('No categories yet')
  await user.click(screen.getByRole('button', { name: 'New category' }))
  const dialog = within(screen.getByRole('dialog'))
  expect(dialog.queryByLabelText(/Bonus/)).toBeNull()
  await user.type(dialog.getByLabelText('Category name'), 'Standard')
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(save.mock.calls[0]?.[1]).toEqual({
    id: undefined,
    name: 'Standard',
    quota: 50000000,
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

test('bonus rules preview face value plus bonus and save the whole edited rule set', async () => {
  const premium = { id: 8, name: 'Premium', quota: 100000000 }
  let rules = [{ category_id: 7, bonus_rate_bps: 200 }]
  mockGet({ categories: () => [tier, premium], rules: () => rules })
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Please retry' },
    })
    .mockImplementation(async (_url, body) => {
      rules = (body as { rules: typeof rules }).rules
      return { data: { success: true, data: rules } }
    })
  const user = renderCategories()
  expect(
    (await screen.findByRole('button', { name: /^Standard/ })).textContent
  ).toContain('+2%')
  await user.click(screen.getByRole('button', { name: 'Open bonus rules' }))
  const dialog = within(screen.getByRole('dialog'))
  const previews = () =>
    dialog.getAllByRole('status').map((entry) => entry.textContent)
  expect(previews()).toEqual([
    `${formatQuota(50000000)}+${formatQuota(1000000)}=${formatQuota(51000000)}`,
  ])

  await user.click(dialog.getByRole('button', { name: 'Add rule' }))
  expect(
    dialog.getByRole<HTMLButtonElement>('button', { name: 'Add rule' }).disabled
  ).toBe(true)
  const percents = dialog.getAllByLabelText('Bonus rate')
  await user.clear(percents[1] as HTMLElement)
  await user.type(percents[1] as HTMLElement, '0')
  expect(previews()[1]).toBe(
    `${formatQuota(100000000)}+${formatQuota(0)}=${formatQuota(100000000)}`
  )
  await user.clear(percents[1] as HTMLElement)
  await user.type(percents[1] as HTMLElement, '12.5')
  expect(previews()).toEqual([
    `${formatQuota(50000000)}+${formatQuota(1000000)}=${formatQuota(51000000)}`,
    `${formatQuota(100000000)}+${formatQuota(12500000)}=${formatQuota(112500000)}`,
  ])

  await user.selectOptions(
    dialog.getAllByLabelText('Category')[1] as HTMLElement,
    '7'
  )
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  expect(dialog.getByText('This category already has a rule')).toBeDefined()
  expect(update).not.toHaveBeenCalled()

  await user.click(
    dialog.getAllByRole('button', { name: 'Delete rule' })[0] as HTMLElement
  )
  await user.selectOptions(dialog.getByLabelText('Category'), '8')
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  expect((await dialog.findByRole('alert')).textContent).toBe('Please retry')
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(update.mock.lastCall?.[0]).toBe('/api/redemption/bonus-rules')
  expect(update.mock.lastCall?.[1]).toEqual({
    rules: [{ category_id: 8, bonus_rate_bps: 1250 }],
  })
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: /^Premium/ }).textContent
    ).toContain('+12.5%')
  )
  expect(
    screen.getByRole('button', { name: /^Standard/ }).textContent
  ).not.toContain('%')
})
