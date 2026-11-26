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
import { I18nextProvider } from 'react-i18next'
import { toast, Toaster } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { getRedemptionBonusRules, getRedemptionCategories } from '../../api'
import { RedemptionCategoryPanel } from '../redemption-category-panel'
import { RedemptionsProvider } from '../redemptions-provider'

const i18n = createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
const config = {
  version: 1,
  categories: [{ name: 'Standard', quota: 5000000, bonus_rate_bps: 250 }],
}

function CategoryPanel() {
  const categories = useQuery({
    queryKey: ['redemption-categories'],
    queryFn: getRedemptionCategories,
  })
  const rules = useQuery({
    queryKey: ['redemption-bonus-rules'],
    queryFn: getRedemptionBonusRules,
  })
  return (
    <RedemptionsProvider
      categories={categories.data}
      bonusRules={rules.data}
      categoryId={0}
    >
      <RedemptionCategoryPanel
        isLoading={categories.isLoading}
        isError={categories.isError}
        retry={() => void categories.refetch()}
        onSelect={() => {}}
      />
    </RedemptionsProvider>
  )
}

function renderPanel() {
  const user = userEvent.setup()
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false, gcTime: 0 },
    },
  })
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <CategoryPanel />
        <Toaster />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return user
}

afterEach(() => {
  cleanup()
  toast.dismiss()
  vi.restoreAllMocks()
})

test('export shows one read-only JSON document and copies it without requesting redemption codes', async () => {
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: { success: true, data: url.endsWith('/export') ? config : [] },
  }))
  const user = renderPanel()
  await user.click(screen.getByRole('button', { name: 'Export JSON' }))
  const dialog = within(
    screen.getByRole('dialog', { name: 'Export code categories' })
  )
  const textbox = dialog.getByRole<HTMLTextAreaElement>('textbox', {
    name: 'Category JSON',
  })
  await waitFor(() =>
    expect(textbox).toHaveValue(JSON.stringify(config, null, 2))
  )
  expect(textbox).toHaveAttribute('readonly')
  await user.click(dialog.getByRole('button', { name: 'Copy JSON' }))
  expect(await navigator.clipboard.readText()).toBe(textbox.value)
  expect(new Set(get.mock.calls.map(([url]) => url))).toEqual(
    new Set([
      '/api/redemption/categories',
      '/api/redemption/bonus-rules',
      '/api/redemption/categories/export',
    ])
  )
})

test('a failed export cannot be copied as an empty config and can be retried', async () => {
  let offline = true
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url.endsWith('/export') && offline) {
      return { data: { success: false, message: 'Temporarily unavailable' } }
    }
    return {
      data: {
        success: true,
        data: url.endsWith('/export') ? { version: 1, categories: [] } : [],
      },
    }
  })
  const user = renderPanel()
  await user.click(screen.getByRole('button', { name: 'Export JSON' }))
  const dialog = within(screen.getByRole('dialog'))
  expect(await dialog.findByRole('alert')).toHaveTextContent(
    'Temporarily unavailable'
  )
  expect(
    dialog.queryByRole('button', { name: 'Copy JSON' })
  ).not.toBeInTheDocument()
  offline = false
  await user.click(dialog.getByRole('button', { name: 'Retry' }))
  expect(await dialog.findByRole('button', { name: 'Copy JSON' })).toBeEnabled()
  expect(
    JSON.parse(dialog.getByRole<HTMLTextAreaElement>('textbox').value)
  ).toEqual({ version: 1, categories: [] })
})

test('invalid or code-bearing JSON is rejected before submission and can be corrected or cancelled', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data: [] } })
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: false } })
  const user = renderPanel()
  await user.click(screen.getByRole('button', { name: 'Import JSON' }))
  const dialog = within(
    screen.getByRole('dialog', { name: 'Import code categories' })
  )
  const submit = dialog.getByRole('button', { name: 'Confirm import' })
  const input = dialog.getByRole('textbox', { name: 'Category JSON' })
  expect(submit).toBeDisabled()
  await user.click(input)
  await user.paste('{bad')
  await user.click(submit)
  expect(dialog.getByRole('alert')).toHaveTextContent('Invalid JSON format')
  await user.clear(input)
  await user.paste(
    JSON.stringify({
      version: 1,
      categories: [{ ...config.categories[0], key: 'not-a-setting' }],
    })
  )
  await user.click(submit)
  expect(input).toHaveAttribute('aria-invalid', 'true')
  expect(dialog.getByRole('alert')).toHaveTextContent(
    'Do not include IDs or redemption codes.'
  )
  await user.clear(input)
  await user.paste(JSON.stringify(config))
  expect(dialog.queryByRole('alert')).not.toBeInTheDocument()
  await user.click(dialog.getByRole('button', { name: 'Cancel' }))
  expect(post).not.toHaveBeenCalled()
})

test('an import conflict preserves the draft for retry and successful import refreshes categories and bonuses', async () => {
  let imported = false
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    let data: unknown = []
    if (imported && url.endsWith('/categories')) {
      data = [{ id: 7, name: 'Standard', quota: 5000000 }]
    }
    if (imported && url.endsWith('/bonus-rules')) {
      data = [{ category_id: 7, bonus_rate_bps: 250 }]
    }
    return { data: { success: true, data } }
  })
  let finish!: () => void
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Conflicting settings' },
    })
    .mockImplementationOnce(async () => {
      await pending
      imported = true
      return { data: { success: true, data: { created: 1, skipped: 0 } } }
    })
  const user = renderPanel()
  await user.click(screen.getByRole('button', { name: 'Import JSON' }))
  const dialog = within(screen.getByRole('dialog'))
  const input = dialog.getByRole('textbox', { name: 'Category JSON' })
  await user.click(input)
  await user.paste(JSON.stringify(config))
  await user.click(dialog.getByRole('button', { name: 'Confirm import' }))
  expect(await dialog.findByRole('alert')).toHaveTextContent(
    'Conflicting settings'
  )
  expect(input).toHaveValue(JSON.stringify(config))
  await user.click(dialog.getByRole('button', { name: 'Confirm import' }))
  expect(dialog.getByRole('button', { name: 'Processing...' })).toBeDisabled()
  expect(input).toBeDisabled()
  expect(dialog.getByRole('button', { name: 'Cancel' })).toBeDisabled()
  finish()
  expect(
    await screen.findByText('Categories imported: 1 new, 0 unchanged')
  ).toBeVisible()
  expect(
    await screen.findByRole('button', { name: /^Standard.*2[.]5%/ })
  ).toBeVisible()
  expect(
    screen.getByRole('button', { name: 'Uncategorized', pressed: true })
  ).toBeVisible()
  expect(post.mock.lastCall?.slice(0, 2)).toEqual([
    '/api/redemption/categories/import',
    config,
  ])
})
