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
package perfmetrics

import (
	"math"

	hosttypes "github.com/QuantumNous/new-api/types"
)

// Per-bucket Coding cost numerators are integers in units of 1e-6 USD per 1M
// tokens: the numerator of a request is "observed input tokens × USD/1M tokens
// × 1e6". Aggregating the integers keeps the published unit price exact
// (sum(numerator) / sum(tokens) / 1e6 == USD/1M tokens) while letting the
// database column stay a plain 64-bit integer on every dialect.

// basePriceFactor converts a settlement model ratio into USD per 1M tokens.
// The project defines a model ratio of 1 as $0.002/1K tokens = $2/1M tokens
// (common.QuotaPerUnit is 500_000, i.e. $1 = 500_000 quota), so the dollar price
// of a ratio is ratio × 2.
const basePriceFactor = 2.0

// microUSDPerUnit is the fixed-point scale of the stored cost numerators.
const microUSDPerUnit = 1e6

// CodingPriceObservation is the input-side cost observation of one settled
// request, in USD per 1M input tokens.
type CodingPriceObservation struct {
	Observable   bool
	ObservedCost float64 // 结算价（含分组倍率与请求级倍率）
	Cache0Cost   float64 // 0% 缓存参考价（不含分组倍率）
	Cache100Cost float64 // 100% 缓存参考价（纯缓存读，不含分组倍率）
}

// CodingCostAmounts is the same observation expressed as the integer numerators
// stored per bucket: CodingObservedCost is already multiplied by the observed
// token count and by codingCostScale.
type CodingCostAmounts struct {
	ObservedCost int64
	Cache0Cost   int64
	Cache100Cost int64
}

// ObserveCodingInputPrice derives the input-side unit prices of one settlement
// from the price data the relay already computed. Observable=false means "no
// input-price observation", so the request only contributes to the group's cost
// coverage denominator and never fabricates a price:
//
//   - per-call / per-task pricing (PriceData.UsePrice) bills a fixed amount, so
//     there is no input unit price to observe;
//   - tiered expression billing (BillingModeTieredExpr) leaves ModelRatio at 0
//     and prices the request nonlinearly — the reference page states such
//     pricing may remain unobserved;
//   - a non-positive or non-finite model ratio means no per-token price is known.
//
// Multipliers: the observed price carries the group multiplier (and any
// request-level multiplier) because that is what actually settles. Both
// reference prices deliberately omit them, matching the reference page copy
// "Reference cost uses the same pricing rules before the group multiplier".
//
// Cache reads enter the observed blended price at their own cache ratio; cache
// writes have no separate observable ratio in the price data, so they stay at
// the base price in the observed numerator only. The reference prices move the
// cache-read share only, which is exactly what "Actual cache-write costs stay
// fixed" describes.
func ObserveCodingInputPrice(priceData hosttypes.PriceData, cacheTokens, cacheCreationTokens int64) CodingPriceObservation {
	if priceData.UsePrice {
		return CodingPriceObservation{}
	}
	basePrice := priceData.ModelRatio * basePriceFactor
	if !isFiniteNonNegative(basePrice) || basePrice == 0 {
		return CodingPriceObservation{}
	}
	multiplier := priceData.GroupRatioInfo.GroupRatio * priceData.OtherRatioMultiplier()
	if !isFiniteNonNegative(multiplier) {
		return CodingPriceObservation{}
	}
	cacheRatio := priceData.CacheRatio
	if !isFiniteNonNegative(cacheRatio) {
		return CodingPriceObservation{}
	}

	observed := basePrice * multiplier
	cacheReadPrice := basePrice * cacheRatio * multiplier

	return CodingPriceObservation{
		Observable:   true,
		ObservedCost: observed,
		Cache0Cost:   basePrice,
		Cache100Cost: cacheReadPrice,
	}
}

// CostAmounts converts the unit prices into the stored per-request numerators.
// It reports ok=false when any price is not a finite non-negative number, so a
// broken settlement can never poison a bucket with NaN or a negative amount.
func (o CodingPriceObservation) CostAmounts(inputTokens, cacheTokens, cacheCreationTokens int64) (CodingCostAmounts, bool) {
	if !o.Observable || inputTokens <= 0 {
		return CodingCostAmounts{}, false
	}
	if !isFiniteNonNegative(o.ObservedCost) || !isFiniteNonNegative(o.Cache0Cost) || !isFiniteNonNegative(o.Cache100Cost) {
		return CodingCostAmounts{}, false
	}
	if cacheTokens < 0 || cacheCreationTokens < 0 {
		return CodingCostAmounts{}, false
	}
	uncachedTokens := inputTokens - cacheTokens - cacheCreationTokens
	if uncachedTokens < 0 {
		// Upstream cache counters can exceed the prompt prefix; never let the
		// overlap turn into a negative contribution.
		cacheTokens = inputTokens
		cacheCreationTokens = 0
		uncachedTokens = 0
	}

	observed := o.ObservedCost * float64(inputTokens)
	// The reference prices move the cache-read share only: cache writes stay at
	// the observed base price, so they are excluded from the reference cover.
	cacheReadCover := float64(uncachedTokens + cacheTokens)
	cache0 := o.Cache0Cost * cacheReadCover
	cache100 := o.Cache100Cost * cacheReadCover
	return CodingCostAmounts{
		ObservedCost: int64(observed * microUSDPerUnit),
		Cache0Cost:   int64(cache0 * microUSDPerUnit),
		Cache100Cost: int64(cache100 * microUSDPerUnit),
	}, true
}

// buildCodingModelSamples converts the per-model Coding counters of one group
// window into the raw samples the performance page consumes.
//
// Per PRICING.md the frontend only accepts a coding_input_weight in (0, 1], so
// the raw Coding input token counts are normalized by the largest per-model
// count in the same group window. Normalizing by a common factor keeps every
// weight ratio (and therefore coverage) identical to a per-request weighting
// while guaranteeing the weight bound. A model without Coding input tokens is
// skipped, because the frontend discards a zero weight.
func buildCodingModelSamples(groupModels map[string]counters) map[string]GroupModelSummary {
	samples := make(map[string]GroupModelSummary, len(groupModels))
	scale := float64(0)
	for _, value := range groupModels {
		if float64(value.codingInputTokens) > scale {
			scale = float64(value.codingInputTokens)
		}
	}
	if scale <= 0 {
		return samples
	}
	// The observed input price is the group-window average of the per-model
	// observed prices weighted by observed input tokens, so it aggregates the
	// same counter set the weights are built from (this group only).
	var groupObservedCost, groupCostWeight int64
	for _, value := range groupModels {
		groupObservedCost += value.codingObservedCost
		groupCostWeight += value.codingCostInputTokens
	}
	totalObservedCost := float64(groupObservedCost) / microUSDPerUnit
	totalCostWeight := float64(groupCostWeight)

	for modelName, value := range groupModels {
		if value.codingInputTokens <= 0 {
			continue
		}
		sample := GroupModelSummary{
			ModelName: modelName,
			// The frontend gate for a price sample is "did this window observe
			// Coding input at all". The flat group-level coding_cache_observed
			// stays on its original narrower definition (cache writes recorded),
			// while the sample gate also accepts read-only cache traffic —
			// otherwise a coding model that only ever reads the cache would be
			// invisible to the pricing table.
			CodingCacheObserved: hasCacheObservation(value),
			CodingCacheHitRate:  math.Round(codingCacheHitRate(value)*100) / 100,
		}
		inputWeight := float64(value.codingInputTokens) / scale
		costWeight := float64(value.codingCostInputTokens) / scale
		if costWeight > inputWeight {
			costWeight = inputWeight
		}
		sample.CodingInputWeight = floatPtr(inputWeight)
		if costWeight > 0 {
			sample.CodingCostInputWeight = floatPtr(costWeight)
		}
		if value.codingCostInputTokens > 0 && totalCostWeight > 0 {
			sample.CodingCostObserved = true
			observedPrice := totalObservedCost / totalCostWeight
			sample.CodingObservedInputPrice = floatPtr(observedPrice)
			// The two reference prices are linear in the cache mix, so their
			// numerators combine into the exact reference price.
			costInputTokens := float64(value.codingCostInputTokens)
			sample.CodingCache0InputPrice = floatPtr(float64(value.codingCache0Cost) / costInputTokens / microUSDPerUnit)
			sample.CodingCache100InputPrice = floatPtr(float64(value.codingCache100Cost) / costInputTokens / microUSDPerUnit)
		}
		samples[modelName] = sample
	}
	return samples
}

// hasCacheObservation reports whether any cache-side token count was recorded,
// so a read-only Coding workload still yields a price sample.
func hasCacheObservation(value counters) bool {
	return value.promptTokens > 0 || value.cacheTokens > 0 || value.cacheCreationTokens > 0
}

func isFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func floatPtr(value float64) *float64 {
	return &value
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return floatPtr(*value)
}
