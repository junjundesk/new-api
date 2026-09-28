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
/**
 * Input-price and multiplier estimation for the performance page.
 *
 * The upstream page ships this calc in the browser: the API only returns raw
 * per-model samples (weight + cache rate + three USD/1M prices) and every
 * derived number below is computed here. Keep the guards verbatim — a sample
 * is silently skipped instead of being coerced to zero.
 */

export type PerfPricingSample = {
  model_name: string
  /** Normalized share of the group's Coding input tokens, (0, 1]. */
  coding_input_weight?: number
  /** Coding cache hit rate as a percentage, 0..100. */
  coding_cache_hit_rate?: number
  coding_cache_observed?: boolean
  /** Must be exactly true to count as a cost observation. */
  coding_cost_observed?: boolean
  /** Cost-observed share of the group input, (0, coding_input_weight]. */
  coding_cost_input_weight?: number
  /** Settled input price, USD per 1M input tokens (group multiplier applied). */
  coding_observed_input_price?: number
  /** Input price with 0% cache reads, USD per 1M tokens. */
  coding_cache0_input_price?: number
  /** Input price with 100% cache reads, USD per 1M tokens. */
  coding_cache100_input_price?: number
}

export type PerfPricingCoverage = 'reliable' | 'insufficient'

export type PerfModelPricing = {
  modelName: string
  inputWeight: number
  pricedInputShare: number
  estimatedInputPricePerMillion: number
  referenceInputPricePerMillion: number | null
  effectiveRatio: number | null
}

export type PerfGroupPricing = {
  nominalRatio: number
  referenceCacheRate: number | null
  observedCacheRate: number
  effectiveRatio: number | null
  estimatedInputPricePerMillion: number | null
  referenceInputPricePerMillion: number | null
  baseInputPricePerMillion: number | null
  baseCacheReadPricePerMillion: number | null
  observedInputMultiplier: number | null
  pricedInputShare: number
  coverage: PerfPricingCoverage
  models: PerfModelPricing[]
}

/** Both group and model multipliers require this input cost coverage. */
export const PRICING_COVERAGE_THRESHOLD = 0.9

/** Maximum accepted reference cache rate; a full 100% is rejected upstream. */
export const MAX_REFERENCE_CACHE_RATE = 0.999

const WEIGHT_TOLERANCE = 1e-12

function isPositiveWeight(value: number | undefined): value is number {
  return value !== undefined && Number.isFinite(value) && value > 0
}

export function computeGroupPricing(input: {
  samples: PerfPricingSample[]
  groupRatio: number
  referenceCacheRate: number | null
}): PerfGroupPricing | null {
  const rate = input.referenceCacheRate
  const validRate =
    rate === null ||
    (Number.isFinite(rate) && rate >= 0 && rate <= MAX_REFERENCE_CACHE_RATE)
  if (
    !Number.isFinite(input.groupRatio) ||
    input.groupRatio < 0 ||
    !validRate
  ) {
    return null
  }

  let cacheWeight = 0
  let observedCacheWeight = 0
  let costWeight = 0
  let observedCost = 0
  let referenceCost = 0
  let cache0Cost = 0
  let cache100Cost = 0
  const models: PerfModelPricing[] = []

  for (const sample of input.samples) {
    const weight = sample.coding_input_weight
    if (!isPositiveWeight(weight) || weight > 1) continue
    if (!sample.coding_cache_observed) continue

    const hitRate = sample.coding_cache_hit_rate
    const hitRateFraction = hitRate === undefined ? Number.NaN : hitRate / 100
    if (
      !Number.isFinite(hitRateFraction) ||
      hitRateFraction < 0 ||
      hitRateFraction > 1
    ) {
      continue
    }

    cacheWeight += weight
    observedCacheWeight += weight * hitRateFraction

    const sampleCostWeight = sample.coding_cost_input_weight
    const observedPrice = sample.coding_observed_input_price
    const cache0Price = sample.coding_cache0_input_price
    const cache100Price = sample.coding_cache100_input_price

    if (sample.coding_cost_observed !== true) continue
    if (!isPositiveWeight(sampleCostWeight)) continue
    if (sampleCostWeight > weight + WEIGHT_TOLERANCE) continue
    if (
      observedPrice === undefined ||
      !Number.isFinite(observedPrice) ||
      observedPrice < 0
    ) {
      continue
    }
    if (
      cache0Price === undefined ||
      !Number.isFinite(cache0Price) ||
      cache0Price < 0
    ) {
      continue
    }
    if (
      cache100Price === undefined ||
      !Number.isFinite(cache100Price) ||
      cache100Price < 0
    ) {
      continue
    }

    const pricedInputShare = Math.min(sampleCostWeight / weight, 1)
    const referencePrice =
      rate === null ? null : (1 - rate) * cache0Price + rate * cache100Price
    const referenceContribution =
      referencePrice === null ? 0 : sampleCostWeight * referencePrice

    costWeight += sampleCostWeight
    observedCost += sampleCostWeight * observedPrice
    referenceCost += referenceContribution
    cache0Cost += sampleCostWeight * cache0Price
    cache100Cost += sampleCostWeight * cache100Price

    const hasModelMultiplier =
      pricedInputShare >= PRICING_COVERAGE_THRESHOLD &&
      referencePrice !== null &&
      referencePrice > 0

    models.push({
      modelName: sample.model_name,
      inputWeight: sampleCostWeight,
      pricedInputShare,
      estimatedInputPricePerMillion: observedPrice,
      referenceInputPricePerMillion: referencePrice,
      effectiveRatio: hasModelMultiplier
        ? observedPrice / referencePrice
        : null,
    })
  }

  if (cacheWeight <= 0) return null

  const pricedInputShare = Math.min(costWeight / cacheWeight, 1)
  const covered = pricedInputShare >= PRICING_COVERAGE_THRESHOLD
  const hasCostWeight = costWeight > 0

  return {
    nominalRatio: input.groupRatio,
    referenceCacheRate: rate,
    observedCacheRate: observedCacheWeight / cacheWeight,
    effectiveRatio:
      covered && referenceCost > 0 ? observedCost / referenceCost : null,
    estimatedInputPricePerMillion: hasCostWeight
      ? observedCost / costWeight
      : null,
    referenceInputPricePerMillion:
      hasCostWeight && rate !== null ? referenceCost / costWeight : null,
    baseInputPricePerMillion: hasCostWeight ? cache0Cost / costWeight : null,
    baseCacheReadPricePerMillion: hasCostWeight
      ? cache100Cost / costWeight
      : null,
    observedInputMultiplier:
      covered && cache0Cost > 0 ? observedCost / cache0Cost : null,
    pricedInputShare,
    coverage: covered ? 'reliable' : 'insufficient',
    models,
  }
}
