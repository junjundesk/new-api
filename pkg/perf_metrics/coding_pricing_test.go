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
	"testing"

	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveCodingInputPriceUnits(t *testing.T) {
	priceData := hosttypes.PriceData{
		ModelRatio:     5,
		CacheRatio:     0.1,
		GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1.5},
	}

	observation := ObserveCodingInputPrice(priceData, 0, 0)
	require.True(t, observation.Observable)
	// ratio 5 → $10/1M; the group multiplier settles, so the observed price is
	// $10 × 1.5; both reference prices are quoted without it.
	assert.InDelta(t, 10, observation.Cache0Cost, 1e-12)
	assert.InDelta(t, 15, observation.ObservedCost, 1e-12)
	assert.InDelta(t, 1.5, observation.Cache100Cost, 1e-12)
}

func TestObserveCodingInputPriceIncludesRequestRatios(t *testing.T) {
	priceData := hosttypes.PriceData{
		ModelRatio:     5,
		CacheRatio:     0.1,
		GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1.5},
	}
	priceData.AddOtherRatio("n", 3)

	observation := ObserveCodingInputPrice(priceData, 0, 0)
	require.True(t, observation.Observable)
	// Request-level multipliers apply to what settles, so the observed price
	// carries them: 5 × 2 × 1.5 × 3.
	assert.InDelta(t, 45, observation.ObservedCost, 1e-12)
	// The cache-read price follows the same multipliers.
	assert.InDelta(t, 4.5, observation.Cache100Cost, 1e-12)
	// Reference prices stay before those multipliers.
	assert.InDelta(t, 10, observation.Cache0Cost, 1e-12)
}

func TestObserveCodingInputPriceSkipsFixedAndNonTokenBilling(t *testing.T) {
	tests := []struct {
		name      string
		priceData hosttypes.PriceData
	}{
		{
			name:      "per-call pricing has no input unit price",
			priceData: hosttypes.PriceData{UsePrice: true, ModelPrice: 3, ModelRatio: 5, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
		},
		{
			name:      "tiered expression leaves the model ratio at zero",
			priceData: hosttypes.PriceData{GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
		},
		{
			name:      "negative model ratio",
			priceData: hosttypes.PriceData{ModelRatio: -1, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
		},
		{
			name:      "negative group ratio",
			priceData: hosttypes.PriceData{ModelRatio: 5, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: -1}},
		},
		{
			name:      "negative cache ratio",
			priceData: hosttypes.PriceData{ModelRatio: 5, CacheRatio: -0.5, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observation := ObserveCodingInputPrice(tt.priceData, 0, 0)
			assert.False(t, observation.Observable)
			amounts, ok := observation.CostAmounts(100, 0, 0)
			assert.False(t, ok)
			assert.Equal(t, CodingCostAmounts{}, amounts)
		})
	}
}

func TestObserveCodingInputPriceZeroRatioIsNotObservable(t *testing.T) {
	// A zero model ratio cannot settle a per-token input price: the charge comes
	// from the minimum-quota floor rather than from tokens (service/text_quota.go
	// raises a zero result to 1 quota when the ratio is non-zero, and a zero ratio
	// charges nothing). Publishing $0.00 here would claim an observation the
	// settlement never made, so the sample is left to the coverage denominator.
	priceData := hosttypes.PriceData{ModelRatio: 0, CacheRatio: 0.1, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}}
	observation := ObserveCodingInputPrice(priceData, 0, 0)
	assert.False(t, observation.Observable)
}

func TestCodingPriceObservationCostAmounts(t *testing.T) {
	observation := CodingPriceObservation{
		Observable:   true,
		ObservedCost: 15,
		Cache0Cost:   10,
		Cache100Cost: 1.5,
	}

	amounts, ok := observation.CostAmounts(200, 60, 20)
	require.True(t, ok)
	// observed: 200 tokens × $15/1M
	assert.EqualValues(t, 200*15*microUSDPerUnit, amounts.ObservedCost)
	// references move the cache-read share only: cache writes are already at the
	// base price in the observed numerator, so references cover 200-20 tokens.
	assert.EqualValues(t, 180*10*microUSDPerUnit, amounts.Cache0Cost)
	assert.EqualValues(t, 180*1.5*microUSDPerUnit, amounts.Cache100Cost)
}

func TestCodingPriceObservationCostAmountsRejectsImpossibleInput(t *testing.T) {
	observation := CodingPriceObservation{Observable: true, ObservedCost: 15, Cache0Cost: 10, Cache100Cost: 1.5}

	t.Run("non-positive token count", func(t *testing.T) {
		_, ok := observation.CostAmounts(0, 0, 0)
		assert.False(t, ok)
	})

	t.Run("non-finite price", func(t *testing.T) {
		broken := observation
		broken.ObservedCost = math.Inf(1)
		_, ok := broken.CostAmounts(100, 0, 0)
		assert.False(t, ok)
	})

	t.Run("cache counters exceed the prompt prefix", func(t *testing.T) {
		// OpenAI cache-write usage can report unadjusted prefix counts; the
		// overlap must be clamped instead of producing a negative reference.
		amounts, ok := observation.CostAmounts(100, 80, 40)
		require.True(t, ok)
		assert.EqualValues(t, 100*15*microUSDPerUnit, amounts.ObservedCost)
		assert.EqualValues(t, 100*10*microUSDPerUnit, amounts.Cache0Cost)
		assert.EqualValues(t, 100*1.5*microUSDPerUnit, amounts.Cache100Cost)
	})
}

func TestBuildCodingModelSamplesNormalizesWeights(t *testing.T) {
	groupModels := map[string]counters{
		"gpt-5-codex": {
			requestCount:          10,
			codingInputTokens:     600_000,
			codingCostInputTokens: 600_000,
			codingObservedCost:    600_000 * 15 * microUSDPerUnit,
			codingCache0Cost:      600_000 * 10 * microUSDPerUnit,
			codingCache100Cost:    600_000 * 1.5 * microUSDPerUnit,
			promptTokens:          60_000,
			cacheTokens:           480_000,
			cacheCreationTokens:   60_000,
		},
		"claude-sonnet-4.5": {
			requestCount: 5,
			// Cost observed for only half of the input: this is the coverage
			// denominator/numerator split.
			codingInputTokens:     300_000,
			codingCostInputTokens: 150_000,
			codingObservedCost:    150_000 * 20 * microUSDPerUnit,
			codingCache0Cost:      150_000 * 18 * microUSDPerUnit,
			codingCache100Cost:    150_000 * 1.8 * microUSDPerUnit,
			promptTokens:          30_000,
			cacheTokens:           240_000,
			cacheCreationTokens:   30_000,
		},
		// No Coding traffic at all: the model must be skipped instead of being
		// published with a zero weight.
		"gpt-4o": {requestCount: 3, promptTokens: 1_000},
	}
	samples := buildCodingModelSamples(groupModels)
	require.Len(t, samples, 2)
	_, hasNoCodingTraffic := samples["gpt-4o"]
	assert.False(t, hasNoCodingTraffic)

	codex := samples["gpt-5-codex"]
	require.NotNil(t, codex.CodingInputWeight)
	assert.InDelta(t, 1, *codex.CodingInputWeight, 1e-12)
	require.NotNil(t, codex.CodingCostInputWeight)
	assert.InDelta(t, 1, *codex.CodingCostInputWeight, 1e-12)
	assert.True(t, codex.CodingCostObserved)
	require.NotNil(t, codex.CodingObservedInputPrice)
	// The settled input price is a group-window average, so every model in the
	// group reports the same value; codex's own per-model prices show up in the
	// two reference fields below.
	assert.InDelta(t, 16, *codex.CodingObservedInputPrice, 1e-9)
	require.NotNil(t, codex.CodingCache0InputPrice)
	assert.InDelta(t, 10, *codex.CodingCache0InputPrice, 1e-9)
	require.NotNil(t, codex.CodingCache100InputPrice)
	assert.InDelta(t, 1.5, *codex.CodingCache100InputPrice, 1e-9)
	assert.True(t, codex.CodingCacheObserved)

	sonnet := samples["claude-sonnet-4.5"]
	require.NotNil(t, sonnet.CodingInputWeight)
	assert.InDelta(t, 0.5, *sonnet.CodingInputWeight, 1e-12)
	require.NotNil(t, sonnet.CodingCostInputWeight)
	assert.InDelta(t, 0.25, *sonnet.CodingCostInputWeight, 1e-12)
	// The frontend skips a sample whose cost weight exceeds the input weight by
	// more than 1e-12; the aggregator must never publish that.
	assert.LessOrEqual(t, *sonnet.CodingCostInputWeight, *sonnet.CodingInputWeight+1e-12)
	assert.True(t, sonnet.CodingCostObserved)
	// Same group-window average as above: (600_000×15 + 150_000×20) / 750_000.
	assert.InDelta(t, 16, *sonnet.CodingObservedInputPrice, 1e-9)
	// The reference prices stay per model: claude-sonnet-4.5 settles at $18/1M
	// with 0% cache and $1.8/1M with 100% cache.
	require.NotNil(t, sonnet.CodingCache0InputPrice)
	assert.InDelta(t, 18, *sonnet.CodingCache0InputPrice, 1e-9)
	require.NotNil(t, sonnet.CodingCache100InputPrice)
	assert.InDelta(t, 1.8, *sonnet.CodingCache100InputPrice, 1e-9)
}

func TestBuildCodingModelSamplesReferencePricesStayExact(t *testing.T) {
	// Two requests of the same model with different cache mixes. The normalizing
	// factor is arbitrary as long as it is shared, so the published prices must
	// match the per-request unit prices exactly.
	groupModels := map[string]counters{
		"a": {
			codingInputTokens:     200,
			codingCostInputTokens: 200,
			codingObservedCost:    200 * 15 * microUSDPerUnit,
			codingCache0Cost:      200 * 10 * microUSDPerUnit,
			codingCache100Cost:    200 * 2 * microUSDPerUnit,
		},
		"b": {
			codingInputTokens:     100,
			codingCostInputTokens: 100,
			codingObservedCost:    100 * 15 * microUSDPerUnit,
			codingCache0Cost:      100 * 10 * microUSDPerUnit,
			codingCache100Cost:    100 * 2 * microUSDPerUnit,
		},
	}
	samples := buildCodingModelSamples(groupModels)
	for name, sample := range samples {
		require.NotNil(t, sample.CodingInputWeight, name)
		require.NotNil(t, sample.CodingCostInputWeight, name)
		require.NotNil(t, sample.CodingObservedInputPrice, name)
		require.NotNil(t, sample.CodingCache0InputPrice, name)
		require.NotNil(t, sample.CodingCache100InputPrice, name)
		assert.InDelta(t, 15, *sample.CodingObservedInputPrice, 1e-9, name)
		assert.InDelta(t, 10, *sample.CodingCache0InputPrice, 1e-9, name)
		assert.InDelta(t, 2, *sample.CodingCache100InputPrice, 1e-9, name)
	}
}

func TestBuildCodingModelSamplesSkipsUnobservedCost(t *testing.T) {
	groupModels := map[string]counters{
		"a": {
			codingInputTokens:     500,
			codingCostInputTokens: 0,
			promptTokens:          500,
		},
		"b": {codingInputTokens: 100, promptTokens: 100},
	}

	samples := buildCodingModelSamples(groupModels)
	a := samples["a"]
	require.NotNil(t, a.CodingInputWeight)
	assert.InDelta(t, 1, *a.CodingInputWeight, 1e-12)
	// No cost observation: the weight and the three prices must stay absent so
	// the frontend sees undefined/null instead of a fabricated 0.
	assert.False(t, a.CodingCostObserved)
	assert.Nil(t, a.CodingCostInputWeight)
	assert.Nil(t, a.CodingObservedInputPrice)
	assert.Nil(t, a.CodingCache0InputPrice)
	assert.Nil(t, a.CodingCache100InputPrice)
}

func TestBuildCodingModelSamplesAcceptsReadOnlyCacheTraffic(t *testing.T) {
	// A Coding client that only ever reads the cache has no cache-creation
	// tokens. The sample gate must still publish it, otherwise the model would
	// disappear from the pricing table entirely.
	groupModels := map[string]counters{
		"claude-code": {
			codingInputTokens:     1_000,
			codingCostInputTokens: 1_000,
			codingObservedCost:    1_000 * 2 * microUSDPerUnit,
			codingCache0Cost:      1_000 * 1 * microUSDPerUnit,
			codingCache100Cost:    1_000 * 0.1 * microUSDPerUnit,
			promptTokens:          200,
			cacheTokens:           800,
		},
	}

	samples := buildCodingModelSamples(groupModels)
	sample, ok := samples["claude-code"]
	require.True(t, ok)
	assert.True(t, sample.CodingCacheObserved)
	// The flat Coding-cache metric keeps its existing definition — the
	// cache-write share of cacheable prompt tokens — which is zero when nothing
	// was written; the sample must still be published with that 0.
	assert.InDelta(t, 0, sample.CodingCacheHitRate, 1e-12)
	assert.True(t, sample.CodingCostObserved)
}

func TestBuildCodingModelSamplesWithoutCodingTraffic(t *testing.T) {
	groupModels := map[string]counters{"a": {requestCount: 5, promptTokens: 100}}
	samples := buildCodingModelSamples(groupModels)
	assert.Empty(t, samples)
}
