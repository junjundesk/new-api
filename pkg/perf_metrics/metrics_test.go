package perfmetrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheHitRate(t *testing.T) {
	cases := []struct {
		name     string
		cache    int64
		prompt   int64
		expected float64
	}{
		{name: "typical", cache: 90, prompt: 10, expected: 90},
		{name: "no input", cache: 0, prompt: 0, expected: 0},
		{name: "fully cached", cache: 50, prompt: 0, expected: 100},
		{name: "no cache", cache: 0, prompt: 100, expected: 0},
		{name: "half", cache: 25, prompt: 75, expected: 25},
		{name: "cache creation included", cache: 50, prompt: 25, expected: 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := counters{cacheTokens: tc.cache, promptTokens: tc.prompt}
			if tc.name == "cache creation included" {
				value.cacheCreationTokens = 50
			}
			require.InDelta(t, tc.expected, cacheHitRate(value), 1e-6)
		})
	}
}

func TestCacheObserved(t *testing.T) {
	require.True(t, cacheObserved(counters{cacheTokens: 1}))
	require.True(t, cacheObserved(counters{promptTokens: 1}))
	require.False(t, cacheObserved(counters{}))
}

func TestAtomicBucketAccumulatesCacheTokens(t *testing.T) {
	bucket := &atomicBucket{}
	bucket.add(Sample{
		Model:               "gpt-4o",
		Group:               "default",
		Success:             true,
		OutputTokens:        100,
		GenerationMs:        1000,
		CacheTokens:         80,
		CacheCreationTokens: 10,
		PromptTokens:        20,
	})
	bucket.add(Sample{
		Model:        "gpt-4o",
		Group:        "default",
		Success:      false,
		CacheTokens:  5,
		PromptTokens: 15,
	})
	snap := bucket.snapshot()
	require.Equal(t, int64(2), snap.requestCount)
	require.Equal(t, int64(1), snap.successCount)
	require.Equal(t, int64(85), snap.cacheTokens)
	require.Equal(t, int64(10), snap.cacheCreationTokens)
	require.Equal(t, int64(35), snap.promptTokens)
	require.InDelta(t, 65.38, math.Round(cacheHitRate(snap)*100)/100, 0.01)
}

func TestCodingCacheHitRate(t *testing.T) {
	cases := []struct {
		name         string
		cache        int64
		prompt       int64
		creation     int64
		expectedRate float64
	}{
		{name: "no data", cache: 0, prompt: 0, creation: 0, expectedRate: 0},
		{name: "creation only", cache: 0, prompt: 0, creation: 50, expectedRate: 100},
		{name: "no creation", cache: 50, prompt: 50, creation: 0, expectedRate: 0},
		{name: "mixed", cache: 60, prompt: 20, creation: 20, expectedRate: 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := counters{
				cacheTokens:         tc.cache,
				promptTokens:        tc.prompt,
				cacheCreationTokens: tc.creation,
			}
			require.InDelta(t, tc.expectedRate, codingCacheHitRate(value), 1e-6)
			require.Equal(t, tc.creation > 0, codingCacheObserved(value))
		})
	}
}
