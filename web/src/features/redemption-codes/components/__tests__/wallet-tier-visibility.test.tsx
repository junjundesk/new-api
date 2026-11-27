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
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { WalletTierVisibilityDialog } from '../wallet-tier-visibility-dialog'

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
const original = useSystemConfigStore.getState().config
const standard = { id: 4, name: 'Standard', quota: 1000, wallet_visible: true }
const promo = { id: 7, name: 'Promotion', quota: 1000, wallet_visible: false }
const premium = { id: 12, name: 'Premium', quota: 5000, wallet_visible: false }
let client: QueryClient

async function openPicker() {
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG, quotaPerUnit: 100 } })
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <WalletTierVisibilityDialog />
      </I18nextProvider>
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Choose wallet tiers' }))
  return {
    user,
    dialog: within(
      await screen.findByRole('dialog', { name: 'Choose wallet tiers' })
    ),
  }
}

afterEach(() => {
  cleanup()
  client.clear()
  vi.restoreAllMocks()
  useSystemConfigStore.setState({ config: original })
})

test('checking two tiers and unchecking another saves only changed IDs, even for equal face values', async () => {
  const unchanged = {
    id: 15,
    name: 'Member',
    quota: 6000,
    wallet_visible: true,
  }
  const before = [standard, promo, premium, unchanged]
  const after = [
    { ...standard, wallet_visible: false },
    { ...promo, wallet_visible: true },
    { ...premium, wallet_visible: true },
    unchanged,
  ]
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({ data: { success: true, data: before } })
    .mockResolvedValue({ data: { success: true, data: after } })
  const saveVisibility = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const { dialog, user } = await openPicker()
  const wasVisible = await dialog.findByRole('checkbox', {
    name: 'Standard $10',
  })
  expect(wasVisible).toBeChecked()
  const toPublish = dialog.getByRole('checkbox', { name: 'Promotion $10' })
  expect(toPublish).not.toBeChecked()
  expect(dialog.getByRole('button', { name: 'Save' })).toBeDisabled()
  await user.click(wasVisible)
  toPublish.focus()
  await user.keyboard('[Space]')
  await user.click(dialog.getByRole('checkbox', { name: 'Premium $50' }))
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(saveVisibility).toHaveBeenCalledOnce()
  expect(saveVisibility.mock.calls[0]?.slice(0, 2)).toEqual([
    '/api/redemption/wallet-visibility',
    { show: [7, 12], hide: [4] },
  ])
  await user.click(screen.getByRole('button', { name: 'Choose wallet tiers' }))
  expect(
    await screen.findByRole('checkbox', { name: 'Promotion $10' })
  ).toBeChecked()
  expect(
    screen.getByRole('checkbox', { name: 'Standard $10' })
  ).not.toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'Member $60' })).toBeChecked()
})

test('cancel discards an unsaved tier change', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [promo] },
  })
  const saveVisibility = vi.spyOn(api, 'put')
  const { dialog, user } = await openPicker()
  await user.click(
    await dialog.findByRole('checkbox', { name: 'Promotion $10' })
  )
  await user.click(dialog.getByRole('button', { name: 'Cancel' }))
  await user.click(screen.getByRole('button', { name: 'Choose wallet tiers' }))
  expect(
    await screen.findByRole('checkbox', { name: 'Promotion $10' })
  ).not.toBeChecked()
  expect(saveVisibility).not.toHaveBeenCalled()
})

test('when hiding the last visible tier fails the administrator can retry without selecting it again', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({ data: { success: true, data: [standard] } })
    .mockResolvedValue({
      data: { success: true, data: [{ ...standard, wallet_visible: false }] },
    })
  const saveVisibility = vi
    .spyOn(api, 'put')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Save unavailable' },
    })
    .mockResolvedValue({ data: { success: true } })
  const { dialog, user } = await openPicker()
  await user.click(
    await dialog.findByRole('checkbox', { name: 'Standard $10' })
  )
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  expect(await dialog.findByRole('alert')).toHaveTextContent('Save unavailable')
  expect(
    dialog.getByRole('checkbox', { name: 'Standard $10' })
  ).not.toBeChecked()
  await user.click(dialog.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(saveVisibility.mock.lastCall?.[1]).toEqual({ show: [], hide: [4] })
})

test('retrying a failed category read shows the empty catalog without allowing a save', async () => {
  vi.spyOn(api, 'get')
    .mockRejectedValueOnce(new Error('Catalog unavailable'))
    .mockResolvedValue({ data: { success: true, data: [] } })
  const { dialog, user } = await openPicker()
  expect(await dialog.findByRole('alert')).toHaveTextContent(
    'Catalog unavailable'
  )
  expect(dialog.getByRole('button', { name: 'Save' })).toBeDisabled()
  await user.click(dialog.getByRole('button', { name: 'Retry' }))
  expect(await dialog.findByText('No categories yet')).toBeVisible()
  expect(dialog.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(dialog.getByRole('button', { name: 'Save' })).toBeDisabled()
})
