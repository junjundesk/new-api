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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := counters{cacheTokens: tc.cache, promptTokens: tc.prompt}
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
		Model:        "gpt-4o",
		Group:        "default",
		Success:      true,
		OutputTokens: 100,
		GenerationMs: 1000,
		CacheTokens:  80,
		PromptTokens: 20,
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
	require.Equal(t, int64(35), snap.promptTokens)
	require.InDelta(t, 70.83, math.Round(cacheHitRate(snap)*100)/100, 0.01)
}
