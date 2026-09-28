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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { Performance } from '../index'
import { installMemoryStorage, installReducedMotion } from './test-environment'

/**
 * Shape captured from a live deployment: the window is populated, every group
 * has traffic but no successful requests yet, and the cost observation columns
 * are still empty because no Coding client has been seen since the rollout.
 * The page must render dashes for the pricing block instead of NaN or a crash.
 */
const LIVE_SHAPED_PAYLOAD = {
  success: true,
  data: {
    groups: [
      {
        group: 'default',
        description: '默认分组',
        ratio: 0.25,
        request_count: 2550,
        success_count: 0,
        avg_ttft_ms: 0,
        avg_latency_ms: 858,
        success_rate: 0,
        avg_tps: 0,
        cache_observed: false,
        cache_hit_rate: 0,
        coding_cache_observed: false,
        coding_cache_hit_rate: 0,
        series: [
          {
            ts: 1790542800,
            request_count: 14,
            avg_latency_ms: 1471,
            success_rate: 0,
          },
          {
            ts: 1790546400,
            request_count: 24,
            avg_latency_ms: 293,
            success_rate: 0,
          },
        ],
        models: [
          {
            model_name: 'doubao-seed-character-251128',
            request_count: 2308,
            success_rate: 0,
            avg_ttft_ms: 0,
            avg_latency_ms: 584,
            avg_tps: 0,
            cache_observed: false,
            cache_hit_rate: 0,
            coding_cache_observed: false,
            coding_cache_hit_rate: 0,
            coding_input_weight: null,
            coding_cost_input_weight: null,
            coding_cost_observed: false,
            coding_observed_input_price: null,
            coding_cache0_input_price: null,
            coding_cache100_input_price: null,
          },
        ],
      },
    ],
    cache_observed: false,
    cache_hit_rate: 0,
    start_ts: 1790542800,
    end_ts: 1790622000,
    bucket_seconds: 3600,
  },
}

vi.mock('@/features/performance-metrics/api', () => ({
  getPerfMetricsGroups: vi.fn(async () => LIVE_SHAPED_PAYLOAD),
}))

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

describe('live-shaped performance payload', () => {
  test('renders unavailable pricing as a dash instead of NaN', async () => {
    installMemoryStorage()
    installReducedMotion()
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: 0, refetchOnWindowFocus: false },
      },
    })
    render(
      <QueryClientProvider client={client}>
        <Performance />
      </QueryClientProvider>
    )

    await waitFor(() => expect(screen.getByText('default')).not.toBeNull())
    expect(
      screen.getAllByText('Observed input multiplier').length
    ).toBeGreaterThan(0)
    expect(
      screen.getAllByText('Reference-adjusted multiplier').length
    ).toBeGreaterThan(0)
    expect(screen.getByText('0/2550 requests succeeded')).not.toBeNull()
    expect(document.body.textContent ?? '').not.toContain('NaN')
  })
})
