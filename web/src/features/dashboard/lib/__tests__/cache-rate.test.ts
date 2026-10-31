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

import { computeCacheRate } from '../stats'

describe('computeCacheRate', () => {
  test('weights the rate by input tokens instead of averaging rows', () => {
    // Naive row averaging would give 50%; token weighting gives 200/400.
    expect(
      computeCacheRate([
        { cache_tokens: 50, cache_input_tokens: 100 },
        { cache_tokens: 150, cache_input_tokens: 300 },
      ])
    ).toBeCloseTo(50, 5)
  })

  test('sums cache reads and input tokens across rows', () => {
    expect(
      computeCacheRate([
        { cache_tokens: 0, cache_input_tokens: 100 },
        { cache_tokens: 25, cache_input_tokens: 100 },
      ])
    ).toBeCloseTo(12.5, 5)
  })

  test('returns null when no row reports input tokens', () => {
    expect(computeCacheRate([])).toBeNull()
    expect(computeCacheRate([{ cache_tokens: 10 }])).toBeNull()
    expect(
      computeCacheRate([{ cache_tokens: 10, cache_input_tokens: 0 }])
    ).toBeNull()
  })

  test('skips rows whose cache reads exceed their input volume', () => {
    // Corrupted usage must not push the page rate above 100%.
    expect(
      computeCacheRate([
        { cache_tokens: 500, cache_input_tokens: 100 },
        { cache_tokens: 40, cache_input_tokens: 200 },
      ])
    ).toBeCloseTo(20, 5)
  })

  test('skips input volumes above the trusted bound', () => {
    expect(
      computeCacheRate([
        { cache_tokens: 900_000_000, cache_input_tokens: 2_000_000_000 },
        { cache_tokens: 10, cache_input_tokens: 100 },
      ])
    ).toBeCloseTo(10, 5)
  })

  test('reports a full hit rate when every input token came from cache', () => {
    expect(
      computeCacheRate([{ cache_tokens: 100, cache_input_tokens: 100 }])
    ).toBe(100)
  })
})
