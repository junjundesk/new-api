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
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { ActivityBenefit, ActivityOffer } from '../../types'
import { ActivityOfferCards } from '../activity-offer-cards'

const i18n = createInstance()
await i18n.init({
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
        Benefits: 'Benefits',
        'Whole group': 'Whole group',
        'Model {{model}}': 'Model {{model}}',
        '{{amount}} tokens': '{{amount}} tokens',
        'Unlimited tokens': 'Unlimited tokens',
      },
    },
  },
  interpolation: { escapeValue: false },
})

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

function benefitFixture(
  overrides: Partial<ActivityBenefit> = {}
): ActivityBenefit {
  return {
    key: 'benefit-1',
    group_name: 'hdfree',
    group_desc: 'Free Group',
    scope: 'group',
    model_name: '',
    rpm_limit: 0,
    token_total: 10_000_000,
    ...overrides,
  }
}

function renderCards(offers: ActivityOffer[], disabled = false) {
  const onRedeem = vi.fn()
  const view = render(
    <I18nextProvider i18n={i18n}>
      <ActivityOfferCards
        offers={offers}
        disabled={disabled}
        onRedeem={onRedeem}
      />
    </I18nextProvider>
  )
  return { container: view.container, onRedeem }
}

function renderedCards(container: HTMLElement): HTMLElement[] {
  return [...container.querySelectorAll<HTMLElement>('[data-slot="card"]')]
}

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

afterEach(cleanup)

describe('activity offer cards', () => {
  it('renders one card per offer with price, group, validity, ratio, rpm and remaining', () => {
    const { container } = renderCards([
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

    const cards = renderedCards(container)
    expect(cards).toHaveLength(2)

    const freeCard = cards[0] as HTMLElement
    expect(freeCard).toHaveTextContent('Free Group')
    expect(freeCard).toHaveTextContent('免费分组')
    expect(freeCard).toHaveTextContent('Free')
    expect(freeCard).toHaveTextContent('8 hours')
    expect(freeCard).toHaveTextContent('Fully free')
    expect(freeCard).toHaveTextContent('Remaining: Unlimited')

    const paidCard = cards[1] as HTMLElement
    expect(paidCard).toHaveTextContent('Paid Group')
    expect(paidCard).toHaveTextContent('$1')
    expect(paidCard).not.toHaveTextContent('Free')
    expect(paidCard).toHaveTextContent('5')
    expect(paidCard).toHaveTextContent('Group default ratio')
    expect(paidCard).toHaveTextContent('Remaining: 3')

    // 回归：付费金额必须与兑换按钮放在一起（页脚），而不是缩在标题右上角。
    const freeFooter = freeCard.querySelector('[data-slot="card-footer"]')
    const paidFooter = paidCard.querySelector('[data-slot="card-footer"]')
    const paidHeader = paidCard.querySelector('[data-slot="card-header"]')
    expect(freeFooter).not.toBeNull()
    expect(paidFooter).not.toBeNull()
    expect(freeFooter).toHaveTextContent('Free')
    expect(paidFooter).toHaveTextContent('$1')
    expect(
      paidFooter?.querySelector('button[aria-label="Redeem Paid Group"]')
    ).not.toBeNull()
    expect(paidHeader).not.toHaveTextContent('$1')
  })

  // 回归：指定模型的活动码必须展示具体模型 ID 与倍率，而不是只给「N 个模型」。
  it('lists the concrete model ids of a selected-models activity code', () => {
    renderCards([
      offerFixture({
        code_id: 31,
        name: 'Models Group',
        group_desc: '指定模型分组',
        ratio_mode: 'inherit',
        model_ratio_overrides: { 'gpt-5': 0, o3: 0.5 },
      }),
    ])

    expect(screen.getByText('Models (ratio)')).toBeVisible()
    expect(
      screen.getByText(/gpt-5: Free of charge, o3: 0\.5x ratio/)
    ).toBeVisible()
    expect(screen.queryByText(/specified models/)).not.toBeInTheDocument()
  })

  // 回归：benefit 类型活动码要逐条展示权益，而不是只显示 ratio 行。
  it('lists every benefit of a benefit offer with its group, scope and token total', () => {
    renderCards([
      offerFixture({
        code_id: 41,
        name: 'Benefit Pack',
        type: 'benefit',
        benefits: [
          benefitFixture(),
          benefitFixture({
            key: 'benefit-2',
            group_name: 'hdpro',
            group_desc: 'Pro Group',
            scope: 'model',
            model_name: 'gpt-5',
            token_total: 0,
          }),
        ],
      }),
    ])

    expect(
      screen.getByText('Free Group · Whole group · 10M tokens')
    ).toBeVisible()
    expect(
      screen.getByText('Pro Group · Model gpt-5 · Unlimited tokens')
    ).toBeVisible()
    expect(screen.getByText('Benefits')).toBeVisible()
    expect(screen.queryByText('Ratio')).not.toBeInTheDocument()
    expect(screen.queryByText('Models (ratio)')).not.toBeInTheDocument()
  })

  it('falls back to the ratio layout when a benefit offer has no benefits from an old API', () => {
    renderCards([
      offerFixture({ code_id: 42, name: 'Legacy Benefit', type: 'benefit' }),
    ])

    expect(screen.getByText('Fully free')).toBeVisible()
    expect(screen.queryByText('Benefits')).not.toBeInTheDocument()
  })

  it('lays the offers out as a single column grid that switches to two columns from the sm breakpoint', () => {
    const { container } = renderCards([
      offerFixture({ code_id: 1 }),
      offerFixture({ code_id: 2 }),
    ])

    const grid = container.querySelector('[data-slot="activity-offer-cards"]')
    expect(grid).not.toBeNull()
    expect(grid).toHaveClass('grid', 'grid-cols-1', 'sm:grid-cols-2')
  })

  it('renders no card when the site has no redeemable offer', () => {
    const { container } = renderCards([])

    expect(
      container.querySelector('[data-slot="activity-offer-cards"]')
    ).toBeNull()
    expect(renderedCards(container)).toHaveLength(0)
  })

  it('redeems only the offer whose card button was pressed', async () => {
    const user = userEvent.setup()
    const { onRedeem } = renderCards([
      offerFixture({ code_id: 11, name: 'Free Group' }),
      offerFixture({ code_id: 22, name: 'Paid Group', price_quota: 500000 }),
    ])

    await user.click(screen.getByRole('button', { name: 'Redeem Paid Group' }))

    expect(onRedeem).toHaveBeenCalledTimes(1)
    expect(onRedeem.mock.calls[0]?.[0]).toMatchObject({ code_id: 22 })
  })

  it('disables every redeem button while a redemption is already running', async () => {
    const user = userEvent.setup()
    const { onRedeem } = renderCards(
      [offerFixture({ code_id: 11, name: 'Free Group' })],
      true
    )

    const button = screen.getByRole('button', { name: 'Redeem Free Group' })
    expect(button).toBeDisabled()

    await user.click(button)
    expect(onRedeem).not.toHaveBeenCalled()
  })
})
