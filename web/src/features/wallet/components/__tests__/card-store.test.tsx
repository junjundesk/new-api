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
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { TopupInfo, WalletCardStore } from '../../types'
import { RechargeFormCard } from '../recharge-form-card'
import { WalletCardStoreSection } from '../wallet-card-store-section'

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })
const original = useSystemConfigStore.getState().config
const store: WalletCardStore = {
  url: 'https://shop.example.test/cards?shop=4',
  show_tiers: true,
  tiers: [
    { id: 1, name: 'Standard', quota: 1000, bonus_quota: 250 },
    { id: 2, name: 'Basic', quota: 200, bonus_quota: 0 },
  ],
}
function cardStoreSection(config: WalletCardStore) {
  return (
    <I18nextProvider i18n={i18n}>
      <WalletCardStoreSection store={config} />
    </I18nextProvider>
  )
}

const gateway: TopupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: false,
  pay_methods: [{ name: 'Epay', type: 'epay' }],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [1],
  discount: {},
  enable_redemption: true,
}
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  useSystemConfigStore.setState({ config: original })
})

test('card mode replaces gateway checkout, shows face value and bonus separately and keeps code redemption usable', async () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaPerUnit: 100,
    },
  })
  const redeem = vi.fn(),
    user = userEvent.setup()
  const props = {
    presetAmounts: [],
    selectedPreset: null,
    onSelectPreset: vi.fn(),
    topupAmount: 1,
    onTopupAmountChange: vi.fn(),
    paymentAmount: 1,
    calculating: false,
    onPaymentMethodSelect: vi.fn(),
    paymentLoading: null,
    redemptionCode: 'code-from-shop',
    onRedemptionCodeChange: vi.fn(),
    onRedeem: redeem,
    redeeming: false,
    topupLink: 'https://old-shop.example.test/',
  }
  const page = (info: TopupInfo) => (
    <I18nextProvider i18n={i18n}>
      <RechargeFormCard {...props} topupInfo={info} />
    </I18nextProvider>
  )
  const { rerender } = render(page({ ...gateway, card_store: store }))
  const [standard, basic] = screen.getAllByRole('listitem')
  expect(within(standard).getByText('Standard')).toBeVisible()
  expect(within(standard).getByText('$10')).toBeVisible()
  expect(within(standard).getByText('$2.5 bonus')).toBeVisible()
  expect(within(basic).getByText('$2')).toBeVisible()
  expect(within(basic).queryByText(/bonus/i)).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Custom Amount')).toBeNull()
  expect(screen.queryByRole('link', { name: 'Get one here' })).toBeNull()
  const frame = screen.getByTitle('Card store checkout')
  expect(frame).toHaveAttribute('src', store.url)
  expect(frame).toHaveAttribute('referrerpolicy', 'no-referrer')
  // Checkout scripts need the store's real origin for its cookies and sessionStorage.
  const permissions = new Set(frame.getAttribute('sandbox')?.split(/\s+/))
  expect(permissions).toEqual(
    new Set(['allow-forms', 'allow-scripts', 'allow-same-origin'])
  )
  expect(screen.getByRole('link', { name: 'Open card store' })).toHaveAttribute(
    'rel',
    'noopener noreferrer'
  )
  rerender(
    page({ ...gateway, card_store: { ...store, show_tiers: false, tiers: [] } })
  )
  expect(
    screen.queryByRole('heading', { name: 'Redemption code tiers' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByText('No redemption code tiers configured yet.')
  ).not.toBeInTheDocument()
  expect(screen.getByTitle('Card store checkout')).toBe(frame)
  await user.click(screen.getByRole('button', { name: 'Redeem' }))
  expect(redeem).toHaveBeenCalledOnce()
  rerender(page(gateway))
  expect(screen.queryByTitle('Card store checkout')).toBeNull()
  expect(screen.getByLabelText('Custom Amount')).toBeVisible()
  expect(screen.getByRole('link', { name: 'Get one here' })).toBeVisible()
})

test('published tiers fit the recharge panel from two mobile columns to eight wide columns without selection controls', () => {
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG, quotaPerUnit: 1 } })
  const amounts = [10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000]
  const tiers = amounts.map((amount) => ({
    id: amount,
    name: String(amount),
    quota: amount,
    bonus_quota: 0,
  }))
  render(
    cardStoreSection({ url: window.location.origin, show_tiers: true, tiers })
  )
  const section = screen.getByRole('region', { name: 'Card store' })
  const grid = within(section).getByRole('list', {
    name: 'Redemption code tiers',
  })
  expect(section).toHaveClass('@container')
  expect(grid).toHaveClass(
    'grid-cols-2',
    '@xs:grid-cols-3',
    '@md:grid-cols-4',
    '@4xl:grid-cols-8'
  )
  expect(within(grid).getAllByRole('listitem')).toHaveLength(11)
  expect(within(grid).queryByRole('button')).not.toBeInTheDocument()
  expect(within(grid).queryByRole('link')).not.toBeInTheDocument()
})

test('only names that repeat the displayed denomination are omitted, not distinct numerical tier names', () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      quotaPerUnit: 100,
      usdExchangeRate: 2,
    },
  })
  const tiers = [
    { id: 1, name: '20000', quota: 1000000, bonus_quota: 50000 },
    { id: 2, name: '20', quota: 2000, bonus_quota: 210 },
  ]
  render(
    cardStoreSection({ url: window.location.origin, show_tiers: true, tiers })
  )
  const [denomination, named] = screen.getAllByRole('listitem')
  expect(within(denomination).getByText('¥20,000')).toBeVisible()
  expect(within(denomination).queryByText('20000')).not.toBeInTheDocument()
  expect(within(denomination).getByText('¥1,000 bonus')).toBeVisible()
  expect(within(named).getByText('20')).toBeVisible()
  expect(within(named).getByText('¥40')).toBeVisible()
  expect(within(named).getByText('¥4.2 bonus')).toBeVisible()
})

test('older store data without explicit visibility does not publish tier details', () => {
  const url = new URL('/shop', window.location.origin).href
  render(cardStoreSection({ url, tiers: store.tiers }))
  expect(screen.queryByRole('list')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: 'Redemption code tiers' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Open card store' })).toHaveAttribute(
    'href',
    url
  )
})

test('switching to a wallet-origin store removes the scripted frame and keeps the standalone checkout link', () => {
  const origin = window.location.origin
  const section = (url: string) => cardStoreSection({ url, tiers: [] })
  const { rerender } = render(section(store.url))
  expect(screen.getByTitle('Card store checkout')).toBeInTheDocument()
  const sameOrigin = new URL('/cards?shop=4', origin).href
  rerender(section(sameOrigin))
  expect(screen.queryByTitle('Card store checkout')).toBeNull()
  const link = screen.getByRole('link', { name: 'Open card store' })
  expect(link).toHaveAttribute('href', sameOrigin)
  expect(link).toHaveAttribute('target', '_blank')
  expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  rerender(section(store.url))
  expect(screen.getByTitle('Card store checkout')).toHaveAttribute(
    'src',
    store.url
  )
})

test('an HTTP store on an HTTPS wallet offers an external checkout instead of mixed content, and unsafe URLs are not navigable', () => {
  const initialURL = window.location.href
  const happyWindow = window as unknown as Window & {
    happyDOM: { setURL(url: string): void }
  }
  happyWindow.happyDOM.setURL('https://wallet.test/wallet')
  try {
    const section = (url: string) =>
      cardStoreSection({ url, show_tiers: true, tiers: [] })
    const { rerender } = render(section('http://shop.test/'))
    expect(
      screen.queryByRole('heading', { name: 'Redemption code tiers' })
    ).not.toBeInTheDocument()
    expect(screen.queryByTitle('Card store checkout')).toBeNull()
    expect(
      screen.getByRole('link', { name: 'Open card store' })
    ).toHaveAttribute('href', 'http://shop.test/')
    for (const url of [
      'javascript:alert(1)',
      'https://user:pass@shop.test/',
      '//shop.test/cards',
    ]) {
      rerender(section(url))
      expect(screen.queryByRole('link')).toBeNull()
      expect(screen.queryByTitle('Card store checkout')).toBeNull()
      expect(screen.getByRole('alert')).toBeVisible()
    }
  } finally {
    happyWindow.happyDOM.setURL(initialURL)
  }
})
