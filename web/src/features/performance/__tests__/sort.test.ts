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
import { beforeEach, describe, expect, test } from 'vitest'

import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'

import { mergeCustomOrder, moveCustomGroup, sortGroups } from '../lib/sort'
import {
  CUSTOM_ORDER_STORAGE_KEY,
  SORT_STORAGE_KEY,
  readStoredCustomOrder,
  readStoredGroupSort,
  writeStoredCustomOrder,
  writeStoredGroupSort,
} from '../lib/sort-storage'
import { makeGroup } from './fixtures'
import { installMemoryStorage } from './test-environment'

beforeEach(() => {
  installMemoryStorage().clear()
})

function pricing(overrides: Partial<PerfGroupPricing>): PerfGroupPricing {
  return {
    nominalRatio: 1,
    referenceCacheRate: 0.9,
    observedCacheRate: 0.5,
    effectiveRatio: null,
    estimatedInputPricePerMillion: null,
    referenceInputPricePerMillion: null,
    baseInputPricePerMillion: null,
    baseCacheReadPricePerMillion: null,
    observedInputMultiplier: null,
    pricedInputShare: 0,
    coverage: 'insufficient',
    models: [],
    ...overrides,
  }
}

describe('sort persistence', () => {
  test('reads the default request volume sort when nothing is stored', () => {
    expect(readStoredGroupSort()).toBe('requests')
  })

  test('reads a stored sort only when it matches a known option', () => {
    window.localStorage.setItem(SORT_STORAGE_KEY, 'latency')
    expect(readStoredGroupSort()).toBe('latency')

    window.localStorage.setItem(SORT_STORAGE_KEY, 'not-a-sort')
    expect(readStoredGroupSort()).toBe('requests')
  })

  test('writes the selected sort under the documented storage key', () => {
    writeStoredGroupSort('coding-cache')
    expect(window.localStorage.getItem(SORT_STORAGE_KEY)).toBe('coding-cache')
  })

  test('reads a custom order and drops empty, non string and duplicate entries', () => {
    window.localStorage.setItem(
      CUSTOM_ORDER_STORAGE_KEY,
      JSON.stringify(['vip', '', 'vip', 7, 'default'])
    )
    expect(readStoredCustomOrder()).toEqual(['vip', 'default'])
  })

  test('reads an empty custom order for malformed storage', () => {
    window.localStorage.setItem(CUSTOM_ORDER_STORAGE_KEY, '{not json')
    expect(readStoredCustomOrder()).toEqual([])

    window.localStorage.setItem(
      CUSTOM_ORDER_STORAGE_KEY,
      JSON.stringify({ a: 1 })
    )
    expect(readStoredCustomOrder()).toEqual([])
  })

  test('writes the custom order as a JSON array', () => {
    writeStoredCustomOrder(['b', 'a'])
    expect(window.localStorage.getItem(CUSTOM_ORDER_STORAGE_KEY)).toBe(
      '["b","a"]'
    )
  })
})

describe('mergeCustomOrder', () => {
  test('keeps saved groups first and appends the rest in name order', () => {
    expect(mergeCustomOrder(['gamma'], ['beta', 'alpha', 'gamma'])).toEqual([
      'gamma',
      'alpha',
      'beta',
    ])
  })

  test('deduplicates both the saved order and the appended remainder', () => {
    expect(mergeCustomOrder(['a', 'a'], ['a', 'b', 'b'])).toEqual(['a', 'b'])
  })
})

describe('moveCustomGroup', () => {
  test('swaps a group with its next visible neighbour', () => {
    expect(
      moveCustomGroup({
        saved: ['a', 'b', 'c'],
        allGroups: ['a', 'b', 'c'],
        visibleGroups: ['a', 'b', 'c'],
        group: 'a',
        delta: 1,
      })
    ).toEqual(['b', 'a', 'c'])
  })

  test('keeps filtered out groups in place while swapping', () => {
    expect(
      moveCustomGroup({
        saved: ['a', 'b', 'c', 'd'],
        allGroups: ['a', 'b', 'c', 'd'],
        visibleGroups: ['a', 'c'],
        group: 'a',
        delta: 1,
      })
    ).toEqual(['c', 'b', 'a', 'd'])
  })

  test('does nothing at the visible boundaries', () => {
    const saved = ['a', 'b', 'c']
    expect(
      moveCustomGroup({
        saved,
        allGroups: saved,
        visibleGroups: ['a', 'c'],
        group: 'c',
        delta: 1,
      })
    ).toEqual(['a', 'b', 'c'])
    expect(
      moveCustomGroup({
        saved,
        allGroups: saved,
        visibleGroups: ['a', 'c'],
        group: 'b',
        delta: -1,
      })
    ).toEqual(['a', 'b', 'c'])
  })
})

describe('sortGroups', () => {
  const emptyPricing = new Map<string, PerfGroupPricing | null>()

  test('sorts by request volume descending with the tie broken by name', () => {
    const groups = [
      makeGroup({ group: 'beta', request_count: 5 }),
      makeGroup({ group: 'alpha', request_count: 5 }),
      makeGroup({ group: 'gamma', request_count: 9 }),
    ]
    const sorted = sortGroups({
      groups,
      sortKey: 'requests',
      customOrder: [],
      pricingByGroup: emptyPricing,
    })
    expect(sorted.map((group) => group.group)).toEqual([
      'gamma',
      'alpha',
      'beta',
    ])
  })

  test('sorts by success rate descending', () => {
    const groups = [
      makeGroup({ group: 'a', success_rate: 50 }),
      makeGroup({ group: 'b', success_rate: 99 }),
      makeGroup({ group: 'c', success_rate: 75 }),
    ]
    const sorted = sortGroups({
      groups,
      sortKey: 'success',
      customOrder: [],
      pricingByGroup: emptyPricing,
    })
    expect(sorted.map((group) => group.group)).toEqual(['b', 'c', 'a'])
  })

  test('sorts by latency ascending and keeps groups without latency last', () => {
    const groups = [
      makeGroup({ group: 'a', avg_latency_ms: 0 }),
      makeGroup({ group: 'b', avg_latency_ms: 800 }),
      makeGroup({ group: 'c', avg_latency_ms: 120 }),
    ]
    const sorted = sortGroups({
      groups,
      sortKey: 'latency',
      customOrder: [],
      pricingByGroup: emptyPricing,
    })
    expect(sorted.map((group) => group.group)).toEqual(['c', 'b', 'a'])
  })

  test('places missing multipliers last in both directions', () => {
    const groups = [
      makeGroup({ group: 'a' }),
      makeGroup({ group: 'b' }),
      makeGroup({ group: 'c' }),
    ]
    const pricingByGroup = new Map<string, PerfGroupPricing | null>([
      ['a', pricing({ observedInputMultiplier: 0.5 })],
      ['b', null],
      ['c', pricing({ observedInputMultiplier: 2 })],
    ])

    const descending = sortGroups({
      groups,
      sortKey: 'observed-desc',
      customOrder: [],
      pricingByGroup,
    })
    expect(descending.map((group) => group.group)).toEqual(['c', 'a', 'b'])

    const ascending = sortGroups({
      groups,
      sortKey: 'observed-asc',
      customOrder: [],
      pricingByGroup,
    })
    expect(ascending.map((group) => group.group)).toEqual(['a', 'c', 'b'])
  })

  test('only ranks Coding cache when the group observed it', () => {
    const groups = [
      makeGroup({
        group: 'a',
        coding_cache_observed: false,
        coding_cache_hit_rate: 99,
      }),
      makeGroup({
        group: 'b',
        coding_cache_observed: true,
        coding_cache_hit_rate: 40,
      }),
      makeGroup({
        group: 'c',
        coding_cache_observed: true,
        coding_cache_hit_rate: 80,
      }),
    ]
    const sorted = sortGroups({
      groups,
      sortKey: 'coding-cache',
      customOrder: [],
      pricingByGroup: emptyPricing,
    })
    expect(sorted.map((group) => group.group)).toEqual(['c', 'b', 'a'])
  })

  test('follows the stored custom order and appends unknown groups', () => {
    const groups = [
      makeGroup({ group: 'alpha' }),
      makeGroup({ group: 'beta' }),
      makeGroup({ group: 'gamma' }),
    ]
    const sorted = sortGroups({
      groups,
      sortKey: 'custom',
      customOrder: ['gamma', 'alpha'],
      pricingByGroup: emptyPricing,
    })
    expect(sorted.map((group) => group.group)).toEqual([
      'gamma',
      'alpha',
      'beta',
    ])
  })

  test('sorts by the display name, falling back to the group key', () => {
    const groups = [
      makeGroup({ group: 'z', name: 'Beta team' }),
      makeGroup({ group: 'a', name: 'Gamma team' }),
      makeGroup({ group: 'm', name: 'alpha squad' }),
      makeGroup({ group: 'delta' }),
    ]
    const sorted = sortGroups({
      groups,
      sortKey: 'name',
      customOrder: [],
      pricingByGroup: emptyPricing,
    })
    expect(sorted.map((group) => group.group)).toEqual(['m', 'z', 'delta', 'a'])
  })
})
