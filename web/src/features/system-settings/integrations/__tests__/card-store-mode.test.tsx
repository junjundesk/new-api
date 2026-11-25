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
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { PaymentSettingsSection } from '../payment-settings-section'

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
const defaults = {
  PayAddress: '',
  EpayId: '',
  EpayKey: '',
  Price: 1,
  MinTopUp: 1,
  CustomCallbackAddress: '',
  PayMethods: '[]',
  AmountOptions: '[]',
  AmountDiscount: '{}',
  StripeApiSecret: '',
  StripeWebhookSecret: '',
  StripePriceId: '',
  StripeUnitPrice: 1,
  StripeMinTopUp: 1,
  StripePromotionCodesEnabled: false,
  CreemApiKey: '',
  CreemWebhookSecret: '',
  CreemTestMode: false,
  CreemProducts: '[]',
}
const waffo = {
  WaffoEnabled: false,
  WaffoApiKey: '',
  WaffoPrivateKey: '',
  WaffoPublicCert: '',
  WaffoSandboxPublicCert: '',
  WaffoSandboxApiKey: '',
  WaffoSandboxPrivateKey: '',
  WaffoSandbox: false,
  WaffoMerchantId: '',
  WaffoCurrency: 'USD',
  WaffoUnitPrice: 1,
  WaffoMinTopUp: 1,
  WaffoNotifyUrl: '',
  WaffoReturnUrl: '',
  WaffoPayMethods: '[]',
}
let client: QueryClient | undefined
let actions: HTMLDivElement | undefined
function renderPayment(enabled: boolean, confirmed: boolean) {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  actions = document.createElement('div')
  document.body.append(actions)
  const root = createRootRoute({
    component: () => (
      <SettingsPageProvider actionsContainer={actions ?? null}>
        <PaymentSettingsSection
          defaultValues={{
            ...defaults,
            CardStoreEnabled: enabled,
            CardStoreURL: enabled ? 'http://shop.test/cards' : '',
          }}
          waffoDefaultValues={waffo}
          waffoPancakeDefaultValues={{
            WaffoPancakeMerchantID: '',
            WaffoPancakePrivateKey: '',
            WaffoPancakeReturnURL: '',
          }}
          complianceDefaults={{
            confirmed,
            termsVersion: confirmed ? 'v1' : '',
            confirmedAt: 0,
            confirmedBy: 0,
          }}
        />
      </SettingsPageProvider>
    ),
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <RouterProvider router={router} />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return userEvent.setup()
}
afterEach(() => {
  cleanup()
  actions?.remove()
  client?.clear()
  vi.restoreAllMocks()
})

test('enabling card store mode requires a safe URL and saves both fields together without changing gateway credentials', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const user = renderPayment(false, true)
  const mode = await screen.findByRole('switch', { name: 'Card store mode' })
  expect(mode).not.toBeChecked()
  expect(screen.queryByLabelText('Card store URL')).toBeNull()
  await user.click(mode)
  const url = screen.getByLabelText('Card store URL')
  await user.type(url, 'javascript:alert(1)')
  await user.click(screen.getByRole('button', { name: 'Save all settings' }))
  expect(
    await screen.findByText(
      'Enter an absolute HTTP or HTTPS store URL without credentials.'
    )
  ).toBeVisible()
  expect(update).not.toHaveBeenCalled()
  await user.clear(url)
  await user.type(url, 'https://shop.test/cards?id=1')
  await user.click(screen.getByRole('button', { name: 'Save all settings' }))
  await waitFor(() =>
    expect(update).toHaveBeenCalledWith('/api/option/', {
      key: 'payment_setting.card_store',
      value: JSON.stringify({
        enabled: true,
        url: 'https://shop.test/cards?id=1',
      }),
    })
  )
  expect(update).toHaveBeenCalledOnce()
  expect(
    screen.getByRole('link', { name: 'Manage code tiers and bonuses' })
  ).toHaveAttribute('href', '/redemption-codes')
})

test('without payment compliance the card store switch cannot activate checkout', async () => {
  const user = renderPayment(false, false)
  const mode = await screen.findByRole('switch', { name: 'Card store mode' })
  expect(mode).toHaveAttribute('aria-disabled', 'true')
  await user.click(mode)
  expect(screen.queryByLabelText('Card store URL')).toBeNull()
})

test('disabling a saved card store preserves its address for the next switch', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const user = renderPayment(true, true)
  expect(await screen.findByLabelText('Card store URL')).toHaveValue(
    'http://shop.test/cards'
  )
  await user.click(screen.getByRole('switch', { name: 'Card store mode' }))
  await user.click(screen.getByRole('button', { name: 'Save all settings' }))
  await waitFor(() =>
    expect(update).toHaveBeenCalledWith('/api/option/', {
      key: 'payment_setting.card_store',
      value: JSON.stringify({ enabled: false, url: 'http://shop.test/cards' }),
    })
  )
})
