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
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'

import { GroupCard } from '../components/group-card'
import { makeGroup, makeModel } from './fixtures'
import { installReducedMotion } from './test-environment'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      let text = key
      if (options) {
        for (const [name, value] of Object.entries(options)) {
          text = text.replaceAll(`{{${name}}}`, String(value))
        }
      }
      return text
    },
    i18n: { resolvedLanguage: 'en', language: 'en' },
  }),
}))

const WINDOW = { bucketSeconds: 3600, startTs: 0, endTs: 86399 }

const MODEL = makeModel({
  model_name: 'gpt-5-codex',
  request_count: 1234,
  success_rate: 98,
  avg_ttft_ms: 120,
  avg_latency_ms: 250,
  avg_tps: 42.5,
  cache_observed: true,
  cache_hit_rate: 88.5,
  coding_cache_observed: true,
  coding_cache_hit_rate: 61.25,
})

const HEALTHY_GROUP = makeGroup({
  group: 'vip',
  name: 'VIP',
  description: 'Priority access',
  ratio: 1.5,
  request_count: 10,
  success_count: 9,
  success_rate: 95,
  avg_latency_ms: 250,
  avg_ttft_ms: 120,
  avg_tps: 42.5,
  cache_observed: true,
  cache_hit_rate: 88.5,
  coding_cache_observed: true,
  coding_cache_hit_rate: 61.25,
  models: [MODEL],
})

const PRICING: PerfGroupPricing = {
  nominalRatio: 1.5,
  referenceCacheRate: 0.9,
  observedCacheRate: 0.885,
  effectiveRatio: 1.234,
  estimatedInputPricePerMillion: 1.5,
  referenceInputPricePerMillion: 0.7,
  baseInputPricePerMillion: 1.6,
  baseCacheReadPricePerMillion: 0.2,
  observedInputMultiplier: 0.938,
  pricedInputShare: 0.833,
  coverage: 'insufficient',
  models: [],
}

beforeEach(() => {
  installReducedMotion()
})

afterEach(() => {
  cleanup()
})

describe('GroupCard', () => {
  test('renders the group identity, health badge and headline metrics', () => {
    render(
      <GroupCard
        summary={HEALTHY_GROUP}
        pricing={PRICING}
        window={WINDOW}
        entranceDelayMs={60}
        onOpenModels={() => undefined}
      />
    )

    expect(screen.getByText('VIP')).not.toBeNull()
    expect(screen.getByText('\u00d71.5')).not.toBeNull()
    expect(screen.getByText('Priority access')).not.toBeNull()
    expect(screen.getByText('Operational')).not.toBeNull()
    expect(screen.getByText('250ms')).not.toBeNull()
    expect(screen.getByText('120ms')).not.toBeNull()
    expect(screen.getByText('42.5 t/s')).not.toBeNull()
    expect(screen.getByText('9/10 requests succeeded')).not.toBeNull()
    expect(screen.getByText('95.00%')).not.toBeNull()
    expect(screen.getByText('88.50%')).not.toBeNull()
    expect(screen.getByText('61.25%')).not.toBeNull()
    expect(screen.getByText('1 models with traffic')).not.toBeNull()
    expect(screen.getByText('View models')).not.toBeNull()
  })

  test('renders both multipliers and the cost coverage note', () => {
    render(
      <GroupCard
        summary={HEALTHY_GROUP}
        pricing={PRICING}
        window={WINDOW}
        entranceDelayMs={0}
        onOpenModels={() => undefined}
      />
    )

    expect(screen.getByText('0.938\u00d7')).not.toBeNull()
    expect(screen.getByText('1.234\u00d7')).not.toBeNull()
    expect(screen.getByText('Cost coverage: 83.3%')).not.toBeNull()
    expect(
      screen.getByText('Pricing coverage is insufficient').className
    ).toContain('sr-only')
  })

  test('renders dashes instead of NaN when the backend omits the pricing fields', () => {
    const { container } = render(
      <GroupCard
        summary={makeGroup({
          group: 'legacy',
          request_count: 5,
          success_count: 5,
          success_rate: 100,
          models: [makeModel({ model_name: 'gpt-4o' })],
        })}
        pricing={null}
        window={WINDOW}
        entranceDelayMs={0}
        onOpenModels={() => undefined}
      />
    )

    const values = [...container.querySelectorAll('dd')].map(
      (node) => node.textContent
    )
    expect(values).toEqual(['0.00%', '\u2014', '\u2014', '\u2014'])
    expect(container.textContent).not.toContain('NaN')
    expect(screen.queryByText('Cost coverage: 0.0%')).toBeNull()
  })

  test('hides the description when it only repeats the group key', () => {
    render(
      <GroupCard
        summary={makeGroup({
          group: 'default',
          description: 'default',
          request_count: 1,
          success_count: 1,
          success_rate: 100,
        })}
        pricing={null}
        window={WINDOW}
        entranceDelayMs={0}
        onOpenModels={() => undefined}
      />
    )

    expect(screen.queryByText('default', { selector: 'p' })).toBeNull()
  })

  test('renders the no traffic state without a percentage', () => {
    render(
      <GroupCard
        summary={makeGroup({
          group: 'idle',
          request_count: 0,
          success_rate: 0,
        })}
        pricing={null}
        window={WINDOW}
        entranceDelayMs={0}
        onOpenModels={() => undefined}
      />
    )

    expect(screen.getByText('No requests in this window')).not.toBeNull()
    expect(screen.getByText('No data')).not.toBeNull()
    expect(screen.getByText('0.00%', { selector: 'dd' })).not.toBeNull()
  })

  test('opens the model breakdown on click and from the keyboard', () => {
    const onOpenModels = vi.fn()
    const { container } = render(
      <GroupCard
        summary={HEALTHY_GROUP}
        pricing={PRICING}
        window={WINDOW}
        entranceDelayMs={0}
        onOpenModels={onOpenModels}
      />
    )
    const card = container.querySelector('article')
    expect(card).not.toBeNull()
    expect(card?.getAttribute('aria-haspopup')).toBe('dialog')

    fireEvent.click(card as HTMLElement)
    expect(onOpenModels).toHaveBeenCalledTimes(1)

    fireEvent.keyDown(card as HTMLElement, { key: 'Enter' })
    fireEvent.keyDown(card as HTMLElement, { key: ' ' })
    expect(onOpenModels).toHaveBeenCalledTimes(3)

    fireEvent.keyDown(card as HTMLElement, { key: 'Escape' })
    expect(onOpenModels).toHaveBeenCalledTimes(3)
  })
})
