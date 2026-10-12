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
import { cleanup, render } from '@testing-library/react'
import { createInstance } from 'i18next'
import type { ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ModelCard } from '../components/model-card'
import { ModelDetailsContent } from '../components/model-details'
import { PricingTable } from '../components/pricing-table'
import type { PricingModel } from '../types'

vi.mock('@lobehub/icons', () => ({}))
vi.mock('@lobehub/fluent-emoji', () => ({}))
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))
vi.mock('@visactor/vchart', () => ({
  default: class {},
  registerAll: () => {},
}))

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

const clients: QueryClient[] = []
function renderUI(element: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { enabled: false, retry: false } },
  })
  clients.push(client)
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>{element}</I18nextProvider>
    </QueryClientProvider>
  )
}
afterEach(() => {
  cleanup()
  for (const client of clients.splice(0)) client.clear()
})

/** Input $4/1M, output $12/1M, cache read $2/1M at 1x. */
const baseModel: PricingModel = {
  id: 1,
  model_name: 'discounted-model',
  quota_type: 0,
  model_ratio: 2,
  completion_ratio: 3,
  cache_ratio: 0.5,
  enable_groups: ['default'],
  group_ratio: { default: 2 },
  billing_ratios: { default: { min: 0.8, max: 0.8, source: 'channel_model' } },
}

function originalPrices(container: HTMLElement): string[] {
  const nodes = container.querySelectorAll('[data-slot="original-price"]')
  return [...nodes].map((node) => node.textContent ?? '')
}

describe('original price display', () => {
  it('crosses out the 1x price next to the ratio price in the table', () => {
    const { container } = renderUI(
      <PricingTable models={[baseModel]} selectedGroup='default' />
    )
    // Price column: input/output at 0.8x, then the cache read column.
    expect(originalPrices(container)).toEqual(['$4 / $12', '$2'])
  })

  it('draws the crossed-out price with a red strikethrough line', () => {
    const { container } = renderUI(
      <ModelCard model={baseModel} onClick={vi.fn()} selectedGroup='default' />
    )
    const struck = container.querySelectorAll('[data-slot="original-price"]')
    expect(struck.length).toBeGreaterThan(0)
    for (const node of struck) {
      expect(node).toHaveClass('line-through')
      expect(node).toHaveClass('decoration-red-500')
    }
  })

  it('shows only one price when the ratio already is the original price', () => {
    const originalModel: PricingModel = {
      ...baseModel,
      billing_ratios: { default: { min: 1, max: 1, source: 'group' } },
    }
    const table = renderUI(
      <PricingTable models={[originalModel]} selectedGroup='default' />
    )
    expect(originalPrices(table.container)).toEqual([])
    expect(table.container.textContent).toContain('$4')
    expect(table.container.textContent).toContain('$12')
    table.unmount()

    const card = renderUI(
      <ModelCard
        model={originalModel}
        onClick={vi.fn()}
        selectedGroup='default'
      />
    )
    expect(originalPrices(card.container)).toEqual([])
  })

  it('strikes the original price when the ratio surcharges the model', () => {
    const surcharged: PricingModel = {
      ...baseModel,
      billing_ratios: { default: { min: 1.5, max: 1.5, source: 'user_group' } },
    }
    const { container } = renderUI(
      <ModelCard model={surcharged} onClick={vi.fn()} selectedGroup='default' />
    )
    expect(originalPrices(container)).toEqual(['$4', '$12', '$2'])
    expect(container.textContent).toContain('$6')
    expect(container.textContent).toContain('$18')
  })

  it('shows the original price for cache reads in the details pricing section', () => {
    const { container } = renderUI(
      <ModelDetailsContent
        model={baseModel}
        selectedGroup='default'
        groupRatio={{ default: 2 }}
        usableGroup={{ default: { desc: 'Default', ratio: 2 } }}
        autoGroups={[]}
        endpointMap={{}}
        tokenUnit='M'
        priceRate={1}
        usdExchangeRate={1}
      />
    )
    expect(originalPrices(container)).toContain('$2')
  })

  it('shows the original price for dynamic pricing entries', () => {
    const dynamicModel: PricingModel = {
      ...baseModel,
      billing_mode: 'tiered_expr',
      billing_expr: 'tier("standard", p * 4 + c * 12)',
    }
    const { container } = renderUI(
      <ModelCard
        model={dynamicModel}
        onClick={vi.fn()}
        selectedGroup='default'
      />
    )
    expect(originalPrices(container)).toEqual(['$4', '$12'])
  })

  it('keeps the original price at the price font size instead of shrinking it', () => {
    const card = renderUI(
      <ModelCard model={baseModel} onClick={vi.fn()} selectedGroup='default' />
    )
    const cardStruck = card.container.querySelectorAll(
      '[data-slot="original-price"]'
    )
    expect(cardStruck.length).toBeGreaterThan(0)
    for (const node of cardStruck) {
      expect(node).toHaveClass('text-sm')
      expect(node).not.toHaveClass('text-xs')
    }
    card.unmount()

    const table = renderUI(
      <PricingTable models={[baseModel]} selectedGroup='default' />
    )
    const tableStruck = table.container.querySelector(
      '[data-slot="original-price"]'
    )
    expect(tableStruck).not.toBeNull()
    expect(tableStruck).toHaveClass('text-sm')
  })
})
