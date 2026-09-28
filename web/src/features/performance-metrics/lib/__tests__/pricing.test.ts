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
import { assert, describe, expect, test } from 'vitest'

import { computeGroupPricing, type PerfPricingSample } from '../pricing'

const CODEX: PerfPricingSample = {
  model_name: 'gpt-5-codex',
  coding_input_weight: 0.6,
  coding_cache_hit_rate: 80,
  coding_cache_observed: true,
  coding_cost_observed: true,
  coding_cost_input_weight: 0.6,
  coding_observed_input_price: 1.5,
  coding_cache0_input_price: 1.25,
  coding_cache100_input_price: 0.125,
}

const SONNET: PerfPricingSample = {
  model_name: 'claude-sonnet-4.5',
  coding_input_weight: 0.3,
  coding_cache_hit_rate: 90,
  coding_cache_observed: true,
  coding_cost_observed: true,
  coding_cost_input_weight: 0.24,
  coding_observed_input_price: 2,
  coding_cache0_input_price: 3,
  coding_cache100_input_price: 0.3,
}

describe('computeGroupPricing', () => {
  test('aggregates weighted prices and multipliers when coverage is reliable', () => {
    const pricing = computeGroupPricing({
      samples: [CODEX, SONNET],
      groupRatio: 1.5,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)

    expect(pricing.nominalRatio).toBe(1.5)
    expect(pricing.coverage).toBe('reliable')
    expect(pricing.observedCacheRate).toBeCloseTo(0.833333, 5)
    expect(pricing.pricedInputShare).toBeCloseTo(0.933333, 5)
    expect(pricing.estimatedInputPricePerMillion).toBeCloseTo(1.642857, 5)
    expect(pricing.referenceInputPricePerMillion).toBeCloseTo(0.3325, 5)
    expect(pricing.baseInputPricePerMillion).toBeCloseTo(1.75, 5)
    expect(pricing.baseCacheReadPricePerMillion).toBeCloseTo(0.175, 5)
    expect(pricing.observedInputMultiplier).toBeCloseTo(0.938776, 5)
    expect(pricing.effectiveRatio).toBeCloseTo(4.940924, 5)
  })

  test('keeps group multipliers per model and drops the model below 90% coverage', () => {
    const pricing = computeGroupPricing({
      samples: [CODEX, SONNET],
      groupRatio: 1.5,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)

    expect(pricing.models).toHaveLength(2)
    const codex = pricing.models[0]
    expect(codex.modelName).toBe('gpt-5-codex')
    expect(codex.inputWeight).toBe(0.6)
    expect(codex.pricedInputShare).toBe(1)
    expect(codex.referenceInputPricePerMillion).toBeCloseTo(0.2375, 5)
    expect(codex.effectiveRatio).toBeCloseTo(6.315789, 5)

    const sonnet = pricing.models[1]
    expect(sonnet.modelName).toBe('claude-sonnet-4.5')
    expect(sonnet.pricedInputShare).toBeCloseTo(0.8, 6)
    expect(sonnet.effectiveRatio).toBeNull()
  })

  test('returns prices but no multipliers when coverage is insufficient', () => {
    const pricing = computeGroupPricing({
      samples: [
        {
          model_name: 'm-a',
          coding_input_weight: 0.5,
          coding_cache_hit_rate: 50,
          coding_cache_observed: true,
          coding_cost_observed: true,
          coding_cost_input_weight: 0.5,
          coding_observed_input_price: 1,
          coding_cache0_input_price: 1,
          coding_cache100_input_price: 0.1,
        },
        {
          model_name: 'm-b',
          coding_input_weight: 0.1,
          coding_cache_hit_rate: 99,
          coding_cache_observed: true,
          coding_cost_observed: false,
          coding_cost_input_weight: 0,
          coding_observed_input_price: 9,
          coding_cache0_input_price: 9,
          coding_cache100_input_price: 9,
        },
      ],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)

    expect(pricing.coverage).toBe('insufficient')
    expect(pricing.pricedInputShare).toBeCloseTo(0.833333, 5)
    expect(pricing.observedCacheRate).toBeCloseTo(0.581667, 5)
    expect(pricing.effectiveRatio).toBeNull()
    expect(pricing.observedInputMultiplier).toBeNull()
    expect(pricing.estimatedInputPricePerMillion).toBe(1)
    expect(pricing.referenceInputPricePerMillion).toBeCloseTo(0.19, 6)
    expect(pricing.baseInputPricePerMillion).toBe(1)
    expect(pricing.baseCacheReadPricePerMillion).toBeCloseTo(0.1, 6)
    expect(pricing.models.map((model) => model.modelName)).toEqual(['m-a'])
  })

  test('skips samples whose cache rate, weight or observation flags are invalid', () => {
    const pricing = computeGroupPricing({
      samples: [
        {
          model_name: 'a',
          coding_input_weight: 0.2,
          coding_cache_hit_rate: 70,
          coding_cache_observed: true,
          coding_cost_observed: true,
          coding_cost_input_weight: 0.2,
          coding_observed_input_price: 1,
          coding_cache0_input_price: 0.5,
          coding_cache100_input_price: 0.05,
        },
        {
          model_name: 'b',
          coding_input_weight: 0.4,
          coding_cache_hit_rate: 60,
          coding_cache_observed: true,
          coding_cost_observed: false,
          coding_cost_input_weight: 0.4,
          coding_observed_input_price: 1,
          coding_cache0_input_price: 1,
          coding_cache100_input_price: 1,
        },
        {
          model_name: 'c',
          coding_input_weight: 0.5,
          coding_cache_hit_rate: 150,
          coding_cache_observed: true,
          coding_cost_observed: true,
          coding_cost_input_weight: 0.5,
          coding_observed_input_price: 1,
          coding_cache0_input_price: 1,
          coding_cache100_input_price: 1,
        },
        {
          model_name: 'd',
          coding_input_weight: 0.2,
          coding_cache_hit_rate: 55,
          coding_cache_observed: false,
          coding_cost_observed: true,
          coding_cost_input_weight: 0.2,
          coding_observed_input_price: 1,
          coding_cache0_input_price: 1,
          coding_cache100_input_price: 1,
        },
      ],
      groupRatio: 2,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)

    expect(pricing.observedCacheRate).toBeCloseTo(0.633333, 5)
    expect(pricing.pricedInputShare).toBeCloseTo(0.333333, 5)
    expect(pricing.referenceInputPricePerMillion).toBeCloseTo(0.095, 6)
    expect(pricing.baseInputPricePerMillion).toBeCloseTo(0.5, 6)
    expect(pricing.baseCacheReadPricePerMillion).toBeCloseTo(0.05, 6)
    expect(pricing.effectiveRatio).toBeNull()
    expect(pricing.observedInputMultiplier).toBeNull()
    expect(pricing.models.map((model) => model.modelName)).toEqual(['a'])
    expect(pricing.models[0].effectiveRatio).toBeCloseTo(10.526316, 5)
  })

  test('returns null without any cache-observed sample', () => {
    expect(
      computeGroupPricing({
        samples: [],
        groupRatio: 1,
        referenceCacheRate: 0.9,
      })
    ).toBeNull()
    expect(
      computeGroupPricing({
        samples: [
          {
            model_name: 'x',
            coding_input_weight: 0.4,
            coding_cache_observed: false,
          },
        ],
        groupRatio: 1,
        referenceCacheRate: 0.9,
      })
    ).toBeNull()
  })

  test('keeps absolute prices but drops reference values when the rate is empty', () => {
    const pricing = computeGroupPricing({
      samples: [CODEX],
      groupRatio: 1,
      referenceCacheRate: null,
    })
    assert(pricing !== null)

    expect(pricing.referenceCacheRate).toBeNull()
    expect(pricing.referenceInputPricePerMillion).toBeNull()
    expect(pricing.effectiveRatio).toBeNull()
    expect(pricing.models[0].referenceInputPricePerMillion).toBeNull()
    expect(pricing.models[0].effectiveRatio).toBeNull()
    expect(pricing.estimatedInputPricePerMillion).toBe(1.5)
    expect(pricing.observedInputMultiplier).toBeCloseTo(1.2, 6)
  })

  test('rejects a reference rate above 99.9% and invalid group ratios', () => {
    expect(
      computeGroupPricing({
        samples: [CODEX],
        groupRatio: 1,
        referenceCacheRate: 1,
      })
    ).toBeNull()
    expect(
      computeGroupPricing({
        samples: [CODEX],
        groupRatio: -1,
        referenceCacheRate: 0.9,
      })
    ).toBeNull()
    expect(
      computeGroupPricing({
        samples: [CODEX],
        groupRatio: Number.NaN,
        referenceCacheRate: 0.9,
      })
    ).toBeNull()
  })

  test('accepts the 1e-12 weight tolerance and clamps the share at one', () => {
    const pricing = computeGroupPricing({
      samples: [
        {
          ...CODEX,
          coding_cost_input_weight: 0.6 + 1e-13,
        },
      ],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)
    expect(pricing.pricedInputShare).toBe(1)
    expect(pricing.models[0].pricedInputShare).toBe(1)
    expect(pricing.coverage).toBe('reliable')
  })

  test('drops the cost side when the weight exceeds the tolerance', () => {
    const pricing = computeGroupPricing({
      samples: [{ ...CODEX, coding_cost_input_weight: 0.6 + 1e-6 }],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)
    expect(pricing.pricedInputShare).toBe(0)
    expect(pricing.coverage).toBe('insufficient')
    expect(pricing.estimatedInputPricePerMillion).toBeNull()
    expect(pricing.models).toEqual([])
  })

  test('ignores a missing cost observation flag', () => {
    const { coding_cost_observed: _omitted, ...withoutFlag } = CODEX
    const pricing = computeGroupPricing({
      samples: [withoutFlag],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)
    expect(pricing.coverage).toBe('insufficient')
    expect(pricing.models).toEqual([])
  })

  test('treats zero prices as observed and withholds both multipliers', () => {
    const pricing = computeGroupPricing({
      samples: [
        {
          ...CODEX,
          coding_observed_input_price: 0,
          coding_cache0_input_price: 0,
          coding_cache100_input_price: 0,
        },
      ],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)
    expect(pricing.coverage).toBe('reliable')
    expect(pricing.estimatedInputPricePerMillion).toBe(0)
    expect(pricing.baseInputPricePerMillion).toBe(0)
    expect(pricing.baseCacheReadPricePerMillion).toBe(0)
    expect(pricing.observedInputMultiplier).toBeNull()
    expect(pricing.effectiveRatio).toBeNull()
    expect(pricing.models[0].effectiveRatio).toBeNull()
  })

  test('treats exactly 90% coverage as reliable', () => {
    const pricing = computeGroupPricing({
      samples: [{ ...CODEX, coding_cost_input_weight: 0.54 }],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    assert(pricing !== null)
    expect(pricing.pricedInputShare).toBeCloseTo(0.9, 6)
    expect(pricing.coverage).toBe('reliable')
    expect(pricing.models[0].effectiveRatio).not.toBeNull()
  })

  test('accepts a 100% cache hit rate and rejects 100.5%', () => {
    const full = computeGroupPricing({
      samples: [{ ...CODEX, coding_cache_hit_rate: 100 }],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    assert(full !== null)
    expect(full.observedCacheRate).toBe(1)

    const over = computeGroupPricing({
      samples: [{ ...CODEX, coding_cache_hit_rate: 100.5 }],
      groupRatio: 1,
      referenceCacheRate: 0.9,
    })
    expect(over).toBeNull()
  })
})
