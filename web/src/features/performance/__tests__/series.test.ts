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
import { describe, expect, test } from 'vitest'

import type { PerformanceGroupSeriesPoint } from '@/features/performance-metrics/types'

import {
  bucketAvailability,
  getAvailabilityBarHeightClass,
} from '../lib/series'

function point(
  overrides: Partial<PerformanceGroupSeriesPoint> & { ts: number }
): PerformanceGroupSeriesPoint {
  return {
    request_count: 0,
    success_rate: 100,
    avg_latency_ms: 0,
    ...overrides,
  }
}

describe('bucketAvailability', () => {
  test('keeps one bar per hourly bucket over a 24 hour window', () => {
    const slots = bucketAvailability([], {
      bucketSeconds: 3600,
      startTs: 0,
      endTs: 86399,
    })
    expect(slots).toHaveLength(24)
    expect(slots[0]).toEqual({
      ts: 0,
      spanSeconds: 3600,
      requestCount: 0,
      successRate: null,
      avgLatencyMs: 0,
    })
    expect(slots[23].ts).toBe(82800)
  })

  test('compresses seven days into 42 bars of four buckets each', () => {
    const slots = bucketAvailability([], {
      bucketSeconds: 3600,
      startTs: 0,
      endTs: 604799,
    })
    expect(slots).toHaveLength(42)
    expect(slots[0].spanSeconds).toBe(14400)
  })

  test('weights success rate and latency by request count and skips empty buckets', () => {
    const slots = bucketAvailability(
      [
        point({
          ts: 0,
          request_count: 3,
          success_rate: 100,
          avg_latency_ms: 300,
        }),
        point({
          ts: 3600,
          request_count: 1,
          success_rate: 0,
          avg_latency_ms: 100,
        }),
        point({
          ts: 7200,
          request_count: 0,
          success_rate: 50,
          avg_latency_ms: 900,
        }),
      ],
      { bucketSeconds: 3600, startTs: 0, endTs: 86399 },
      1
    )

    expect(slots).toHaveLength(1)
    expect(slots[0].requestCount).toBe(4)
    expect(slots[0].successRate).toBe(75)
    expect(slots[0].avgLatencyMs).toBe(250)
    expect(slots[0].spanSeconds).toBe(86400)
  })

  test('rounds the weighted success rate to two decimals', () => {
    const slots = bucketAvailability(
      [point({ ts: 0, request_count: 3, success_rate: 33.33 })],
      { bucketSeconds: 3600, startTs: 0, endTs: 3599 }
    )
    expect(slots[0].successRate).toBe(33.33)
  })

  test('returns no bars when the window is inverted', () => {
    expect(
      bucketAvailability([], {
        bucketSeconds: 3600,
        startTs: 7200,
        endTs: 3600,
      })
    ).toEqual([])
  })

  test('falls back to hourly buckets when the bucket size is unusable', () => {
    const slots = bucketAvailability([], {
      bucketSeconds: 0,
      startTs: 0,
      endTs: 86399,
    })
    expect(slots).toHaveLength(24)
    expect(slots[0].spanSeconds).toBe(3600)
  })
})

describe('getAvailabilityBarHeightClass', () => {
  test('maps each success rate band to its bar height', () => {
    expect(getAvailabilityBarHeightClass(null)).toBe('h-[30%]')
    expect(getAvailabilityBarHeightClass(99.9)).toBe('h-full')
    expect(getAvailabilityBarHeightClass(99)).toBe('h-[88%]')
    expect(getAvailabilityBarHeightClass(95)).toBe('h-[72%]')
    expect(getAvailabilityBarHeightClass(90)).toBe('h-[55%]')
    expect(getAvailabilityBarHeightClass(89.99)).toBe('h-[42%]')
    expect(getAvailabilityBarHeightClass(0)).toBe('h-[42%]')
  })
})
