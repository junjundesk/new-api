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
import { readFileSync } from 'node:fs'

import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'

import { ModelBreakdownDialog } from '../components/model-breakdown-dialog'
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
  models: [
    {
      modelName: 'gpt-5-codex',
      inputWeight: 0.6,
      pricedInputShare: 0.833,
      estimatedInputPricePerMillion: 1.5,
      referenceInputPricePerMillion: 0.7,
      effectiveRatio: 1.234,
    },
  ],
}

const SUMMARY = makeGroup({
  group: 'vip',
  name: 'VIP',
  ratio: 1.5,
  request_count: 10,
  success_count: 9,
  success_rate: 95,
  avg_latency_ms: 250,
  avg_ttft_ms: 120,
  avg_tps: 42.5,
  cache_observed: true,
  cache_hit_rate: 88.5,
  models: [
    makeModel({
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
    }),
  ],
})

beforeEach(() => {
  installReducedMotion()
})

afterEach(() => {
  cleanup()
})

describe('ModelBreakdownDialog', () => {
  test('renders the group header and every model table column', () => {
    render(
      <ModelBreakdownDialog
        summary={SUMMARY}
        pricing={PRICING}
        window={WINDOW}
        hours={24}
        onClose={() => undefined}
      />
    )

    expect(screen.getByRole('dialog')).not.toBeNull()
    expect(screen.getByText('VIP')).not.toBeNull()
    expect(
      screen.getByText('Per-model availability within this group')
    ).not.toBeNull()
    expect(screen.getByText('Model breakdown')).not.toBeNull()
    expect(screen.getByText('1 models with traffic')).not.toBeNull()

    for (const header of [
      'Model',
      'Requests',
      'Success rate',
      'Cache hit',
      'Coding cache',
      'Reference-adjusted multiplier',
      'Average TTFT',
      'Average latency',
      'TPS',
    ]) {
      expect(screen.getByRole('columnheader', { name: header })).not.toBeNull()
    }
  })

  test('renders formatted model values and the coding cost coverage', () => {
    render(
      <ModelBreakdownDialog
        summary={SUMMARY}
        pricing={PRICING}
        window={WINDOW}
        hours={24}
        onClose={() => undefined}
      />
    )

    // The dialog portal lives outside the render container.
    const cells = [...document.querySelectorAll('tbody td')].map(
      (cell) => cell.textContent ?? ''
    )
    expect(cells[0]).toContain('gpt-5-codex')
    expect(cells[0]).toContain('Cost coverage: 83.3%')
    expect(cells[1]).toBe('1,234')
    expect(cells[2]).toBe('98.00%')
    // The mobile sub line repeats the Coding cache value inside the same cell.
    expect(cells[3]).toBe('88.50%Coding cache 61.25%')
    expect(cells[4]).toBe('61.25%')
    expect(cells[5]).toBe('1.234\u00d7')
    expect(cells[6]).toBe('120ms')
    expect(cells[7]).toBe('250ms')
    expect(cells[8]).toBe('42.5 t/s')
  })

  test('shows the empty model state without rendering a table', () => {
    render(
      <ModelBreakdownDialog
        summary={makeGroup({
          group: 'empty',
          request_count: 3,
          success_count: 3,
        })}
        pricing={null}
        window={WINDOW}
        hours={24}
        onClose={() => undefined}
      />
    )

    expect(
      screen.getByText('No model traffic in this group yet.')
    ).not.toBeNull()
    expect(screen.queryByRole('table')).toBeNull()
    expect(
      screen.getByText('No valid Coding input was observed in this window.')
    ).not.toBeNull()
  })

  test('reports the close request when the user dismisses the dialog', async () => {
    const onClose = vi.fn()
    render(
      <ModelBreakdownDialog
        summary={SUMMARY}
        pricing={PRICING}
        window={WINDOW}
        hours={24}
        onClose={onClose}
      />
    )

    await userEvent.setup().click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalled()
  })

  test('keeps the popup centred while the pop animation runs', () => {
    render(
      <ModelBreakdownDialog
        summary={SUMMARY}
        pricing={PRICING}
        window={WINDOW}
        hours={24}
        onClose={() => undefined}
      />
    )

    const dialog = screen.getByRole('dialog')
    // Tailwind centres the popup with the `translate` property, which lives
    // beside `transform`: any keyframe that animates transform must not add a
    // second translation, or the popup lands off centre.
    expect(dialog.className).toContain('-translate-x-1/2')
    expect(dialog.className).toContain('-translate-y-1/2')
    expect(dialog.className).toContain('perf-dialog-pop')

    const css = readFileSync('src/styles/index.css', 'utf8')
    const keyframes = css.slice(css.indexOf('@keyframes perf-dialog-pop'))
    const body = keyframes.slice(0, keyframes.indexOf('}'))
    expect(body).not.toContain('transform:')
    expect(body).not.toContain('translate(')
    expect(body).toContain('scale')
  })
})
