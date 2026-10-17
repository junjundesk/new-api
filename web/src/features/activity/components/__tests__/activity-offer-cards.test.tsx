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
import assert from 'node:assert/strict'

import { Window } from 'happy-dom'

import type { ActivityOffer } from '../../types'

// Use Bun's runner at runtime while reusing the Node test types installed here.
const bunTestModule = 'bun:test'
const { afterEach, test } = (await import(bunTestModule)) as {
  afterEach: typeof import('node:test').afterEach
  test: typeof import('node:test').test
}

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'Node',
  'Element',
  'Event',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const i18n = (await import('i18next')).default
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const storeModule = await import('@/stores/system-config-store')
const { ActivityOfferCards } = await import('../activity-offer-cards')

await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Redeem: 'Redeem',
        Free: 'Free',
        Group: 'Group',
        Validity: 'Validity',
        Ratio: 'Ratio',
        'RPM limit': 'RPM limit',
        Unlimited: 'Unlimited',
        Remaining: 'Remaining',
        Permanent: 'Permanent',
        'Group default ratio': 'Group default ratio',
        'Fully free': 'Fully free',
        '{{count}} hours': '{{count}} hours',
        '{{count}} days': '{{count}} days',
        '{{count}} minutes': '{{count}} minutes',
        'Redeem {{name}}': 'Redeem {{name}}',
        'Limited-time group access granted by an activity code':
          'Limited-time group access granted by an activity code',
        'Models (ratio)': 'Models (ratio)',
        'All models in the group': 'All models in the group',
        'Free of charge': 'Free of charge',
        '{{ratio}}x ratio': '{{ratio}}x ratio',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let renderedHost: HTMLDivElement | null = null
let renderedRoot: ReturnType<typeof createRoot> | null = null
const redeemed: ActivityOffer[] = []

function offerFixture(overrides: Partial<ActivityOffer> = {}): ActivityOffer {
  return {
    code_id: 1,
    code: 'hdmfree',
    name: 'Free Group',
    description: '',
    group_name: 'hdfree',
    group_desc: 'Free Group',
    duration_seconds: 24 * 3600,
    rpm_limit: 0,
    ratio_mode: 'override',
    ratio_override: 0,
    price_quota: 0,
    remaining_total: -1,
    ...overrides,
  }
}

async function renderCards(
  offers: ActivityOffer[],
  disabled = false
): Promise<void> {
  const host = document.createElement('div')
  document.body.append(host)
  renderedHost = host
  const root = createRoot(host)
  renderedRoot = root
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityOfferCards
          offers={offers}
          disabled={disabled}
          onRedeem={(offer) => redeemed.push(offer)}
        />
      </I18nextProvider>
    )
  )
}

function renderedCards(): Element[] {
  return [...document.querySelectorAll('[data-slot="card"]')]
}

function redeemButton(accessibleName: string): HTMLButtonElement | undefined {
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find(
    (button) => button.getAttribute('aria-label') === accessibleName
  )
}

afterEach(async () => {
  await act(async () => renderedRoot?.unmount())
  renderedHost?.remove()
  renderedHost = null
  renderedRoot = null
  redeemed.length = 0
})

test('renders one card per offer showing price, group, validity, ratio, rpm and remaining redemptions', async () => {
  storeModule.useSystemConfigStore.getState().setConfig({
    currency: { ...storeModule.DEFAULT_CURRENCY_CONFIG },
  })
  await renderCards([
    // 8 小时渲染为小时文案（≥24h 会转成天数），断言更直白。
    offerFixture({
      code_id: 1,
      name: 'Free Group',
      group_desc: '免费分组',
      duration_seconds: 8 * 3600,
    }),
    offerFixture({
      code_id: 2,
      code: 'paid-24h',
      name: 'Paid Group',
      group_desc: '付费分组',
      duration_seconds: 24 * 3600,
      rpm_limit: 5,
      ratio_mode: 'inherit',
      price_quota: 500000,
      remaining_total: 3,
    }),
  ])

  const cards = renderedCards()
  assert.equal(cards.length, 2)

  const freeText = cards[0]?.textContent ?? ''
  assert.ok(freeText.includes('Free Group'), 'free offer name must render')
  assert.ok(freeText.includes('免费分组'), 'free offer group must render')
  assert.ok(freeText.includes('Free'), 'free offer must show the Free badge')
  assert.ok(freeText.includes('8 hours'), 'free offer validity must render')
  assert.ok(freeText.includes('Fully free'), 'zero ratio must render as free')
  assert.ok(
    freeText.includes('Remaining: Unlimited'),
    'unlimited remaining redemptions must render'
  )

  const paidText = cards[1]?.textContent ?? ''
  assert.ok(paidText.includes('Paid Group'), 'paid offer name must render')
  assert.ok(
    paidText.includes('$1'),
    'paid offer must show its price in wallet units'
  )
  assert.ok(
    !paidText.includes('Free'),
    'paid offer must not render the Free badge'
  )
  assert.ok(paidText.includes('5'), 'paid offer rpm limit must render')
  assert.ok(
    paidText.includes('Group default ratio'),
    'inherited ratio must render as the group default'
  )
  assert.ok(
    paidText.includes('Remaining: 3'),
    'limited remaining redemptions must render'
  )

  // 回归：付费金额必须与兑换按钮放在一起（页脚），而不是缩在标题右上角。
  const freeFooter = cards[0]?.querySelector('[data-slot="card-footer"]')
  const paidFooter = cards[1]?.querySelector('[data-slot="card-footer"]')
  const paidHeader = cards[1]?.querySelector('[data-slot="card-header"]')
  assert.ok(freeFooter, 'offer card footer must render')
  assert.ok(paidFooter, 'paid offer card footer must render')
  assert.ok(
    freeFooter.textContent?.includes('Free'),
    'the free label must sit in the footer next to the redeem action'
  )
  assert.ok(
    paidFooter.textContent?.includes('$1'),
    'the paid amount must sit in the footer next to the redeem action'
  )
  assert.ok(
    paidFooter.querySelector('button[aria-label="Redeem Paid Group"]'),
    'the price and the redeem button must share the footer'
  )
  assert.ok(
    !paidHeader?.textContent?.includes('$1'),
    'the paid amount must not stay in the card header corner'
  )
})

// 回归：指定模型的活动码必须展示具体模型 ID 与倍率，而不是只给「N 个模型」。
test('lists the concrete model ids of a selected-models activity code', async () => {
  storeModule.useSystemConfigStore.getState().setConfig({
    currency: { ...storeModule.DEFAULT_CURRENCY_CONFIG },
  })
  await renderCards([
    offerFixture({
      code_id: 31,
      name: 'Models Group',
      group_desc: '指定模型分组',
      ratio_mode: 'inherit',
      model_ratio_overrides: { 'gpt-5': 0, o3: 0.5 },
    }),
  ])

  const text = renderedCards()[0]?.textContent ?? ''
  assert.ok(text.includes('Models (ratio)'), 'the model scope label must render')
  assert.ok(
    text.includes('gpt-5: Free of charge'),
    'a zero ratio must read as free instead of 0x'
  )
  assert.ok(
    text.includes('o3: 0.5x ratio'),
    'a non-zero ratio must render as the multiplier with its unit'
  )
  assert.ok(
    !text.includes('specified models'),
    'the count-only copy must not be the model scope anymore'
  )
})

test('lays the offers out as a single column grid that switches to two columns from the sm breakpoint', async () => {
  await renderCards([offerFixture({ code_id: 1 }), offerFixture({ code_id: 2 })])

  const grid = document.querySelector('[data-slot="activity-offer-cards"]')
  assert.ok(grid, 'offer grid must render')
  assert.ok(grid.classList.contains('grid'), 'offers must use a grid container')
  assert.ok(
    grid.classList.contains('grid-cols-1'),
    'offers must stack in one column on small screens'
  )
  assert.ok(
    grid.classList.contains('sm:grid-cols-2'),
    'offers must use two columns from the sm breakpoint'
  )
})

test('renders no card when the site has no redeemable offer', async () => {
  await renderCards([])

  assert.equal(
    document.querySelector('[data-slot="activity-offer-cards"]'),
    null
  )
  assert.equal(renderedCards().length, 0)
})

test('redeems only the offer whose card button was pressed', async () => {
  await renderCards([
    offerFixture({ code_id: 11, name: 'Free Group' }),
    offerFixture({ code_id: 22, name: 'Paid Group', price_quota: 500000 }),
  ])

  const button = redeemButton('Redeem Paid Group')
  assert.ok(button, 'each card must expose a distinguishable redeem button')
  await act(async () => button.click())

  assert.deepEqual(
    redeemed.map((offer) => offer.code_id),
    [22]
  )
})

test('disables every redeem button while a redemption is already running', async () => {
  await renderCards([offerFixture({ code_id: 11, name: 'Free Group' })], true)

  const button = redeemButton('Redeem Free Group')
  assert.ok(button, 'redeem button must render')
  assert.equal(button.disabled, true)

  await act(async () => button.click())
  assert.deepEqual(redeemed, [])
})
