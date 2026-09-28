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

import { computeGroupPricing } from '@/features/performance-metrics/lib/pricing'

import {
  DEFAULT_REFERENCE_CACHE_RATE_INPUT,
  parseReferenceCacheRate,
} from '../lib/reference-cache-rate'

describe('parseReferenceCacheRate', () => {
  test('the default input is the documented 90 percent', () => {
    expect(DEFAULT_REFERENCE_CACHE_RATE_INPUT).toBe('90')
    expect(parseReferenceCacheRate(DEFAULT_REFERENCE_CACHE_RATE_INPUT)).toBe(
      0.9
    )
  })

  test('accepts the whole valid percentage range', () => {
    expect(parseReferenceCacheRate('0')).toBe(0)
    expect(parseReferenceCacheRate('12.5')).toBe(0.125)
    expect(parseReferenceCacheRate('99.9')).toBeCloseTo(0.999, 12)
  })

  test('rejects an empty value instead of falling back to 90 percent', () => {
    expect(parseReferenceCacheRate('')).toBeNull()
    expect(parseReferenceCacheRate('   ')).toBeNull()
  })

  test('rejects non numeric, negative and out of range values', () => {
    expect(parseReferenceCacheRate('abc')).toBeNull()
    expect(parseReferenceCacheRate('90abc')).toBeNull()
    expect(parseReferenceCacheRate('-1')).toBeNull()
    expect(parseReferenceCacheRate('100')).toBeNull()
    expect(parseReferenceCacheRate('99.91')).toBeNull()
  })

  test('the largest accepted percentage yields a usable pricing input', () => {
    const rate = parseReferenceCacheRate('99.9')
    expect(rate).not.toBeNull()
    expect(
      computeGroupPricing({
        samples: [
          {
            model_name: 'm',
            coding_input_weight: 0.5,
            coding_cache_hit_rate: 90,
            coding_cache_observed: true,
            coding_cost_observed: true,
            coding_cost_input_weight: 0.5,
            coding_observed_input_price: 1,
            coding_cache0_input_price: 1,
            coding_cache100_input_price: 0.1,
          },
        ],
        groupRatio: 1,
        referenceCacheRate: rate,
      })
    ).not.toBeNull()
  })
})
