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
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The performance page decides "observed" with strict comparisons, so the wire
// contract has to be exact: an unobservable price must be null (never 0), and
// coding_cost_observed must be a real boolean.
func TestGroupSummaryJSONContract(t *testing.T) {
	weight := 0.6
	costWeight := 0.45
	observed := 1.5
	cache0 := 1.25
	cache100 := 0.125

	summary := GroupSummary{
		Group:       "default",
		Description: "Default",
		Ratio:       1.5,
		Summary: &GroupMetricSummary{
			RequestCount:        7,
			SuccessCount:        6,
			SuccessRate:         85.71,
			AvgTtftMs:           120,
			AvgLatencyMs:        900,
			AvgTps:              42.5,
			CacheObserved:       true,
			CacheHitRate:        80,
			CodingCacheObserved: true,
			CodingCacheHitRate:  12.5,
		},
		Series: []GroupSeriesPoint{{Ts: 1, RequestCount: 7}},
		Models: []GroupModelSummary{
			{
				ModelName:                "gpt-5-codex",
				CodingCacheObserved:      true,
				CodingCacheHitRate:       80,
				CodingInputWeight:        &weight,
				CodingCostInputWeight:    &costWeight,
				CodingCostObserved:       true,
				CodingObservedInputPrice: &observed,
				CodingCache0InputPrice:   &cache0,
				CodingCache100InputPrice: &cache100,
			},
			{
				ModelName:           "gpt-4o",
				CodingCacheObserved: true,
				CodingCacheHitRate:  0,
				CodingInputWeight:   &weight,
			},
		},
	}

	encoded, err := common.Marshal(summary)
	require.NoError(t, err)
	payload := string(encoded)

	// Unobservable samples must publish nulls rather than zero-valued prices.
	assert.Contains(t, payload, `"model_name":"gpt-4o"`)
	assert.Contains(t, payload, `"coding_cost_observed":false`)
	assert.Contains(t, payload, `"coding_cost_input_weight":null`)
	assert.Contains(t, payload, `"coding_observed_input_price":null`)
	assert.Contains(t, payload, `"coding_cache0_input_price":null`)
	assert.Contains(t, payload, `"coding_cache100_input_price":null`)

	// Observed samples carry the raw numbers the browser interpolates.
	assert.Contains(t, payload, `"coding_cost_observed":true`)
	assert.Contains(t, payload, `"coding_observed_input_price":1.5`)
	assert.Contains(t, payload, `"coding_cache100_input_price":0.125`)

	// The nested summary duplicates the flat observables for the reference page.
	assert.Contains(t, payload, `"summary":{`)
	assert.Contains(t, payload, `"coding_cache_hit_rate":12.5`)
	for _, field := range []string{
		"request_count", "success_count", "success_rate", "avg_ttft_ms",
		"avg_latency_ms", "avg_tps", "cache_observed", "cache_hit_rate",
		"coding_cache_observed", "coding_cache_hit_rate",
	} {
		assert.Contains(t, payload, `"`+field+`":`, "nested summary field %s", field)
	}
}
