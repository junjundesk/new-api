package perfmetrics

import "sync/atomic"

type Store interface {
	Record(sample Sample)
	Query(params QueryParams) (QueryResult, error)
}

type Sample struct {
	Model               string
	Group               string
	LatencyMs           int64
	TtftMs              int64
	HasTtft             bool
	Success             bool
	OutputTokens        int64
	GenerationMs        int64
	CacheTokens         int64 // prompt cache 命中 token
	CacheCreationTokens int64 // prompt cache 创建 token
	PromptTokens        int64 // 输入 token（未命中缓存部分）
}

type QueryParams struct {
	Model string
	Group string
	Hours int
}

type BucketPoint struct {
	Ts           int64   `json:"ts"`
	RequestCount int64   `json:"request_count"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type GroupResult struct {
	Group        string        `json:"group"`
	AvgTtftMs    int64         `json:"avg_ttft_ms"`
	AvgLatencyMs int64         `json:"avg_latency_ms"`
	SuccessRate  float64       `json:"success_rate"`
	AvgTps       float64       `json:"avg_tps"`
	Series       []BucketPoint `json:"series"`
}

type QueryResult struct {
	ModelName    string        `json:"model_name"`
	SeriesSchema string        `json:"series_schema"`
	Groups       []GroupResult `json:"groups"`
}

type ModelSummary struct {
	ModelName          string    `json:"model_name"`
	AvgLatencyMs       int64     `json:"avg_latency_ms"`
	SuccessRate        float64   `json:"success_rate"`
	AvgTps             float64   `json:"avg_tps"`
	RecentSuccessRates []float64 `json:"recent_success_rates,omitempty"`
	RequestCount       int64     `json:"-"`
}

type SummaryAllResult struct {
	Models []ModelSummary `json:"models"`
}

type GroupMetadata struct {
	Description string
	Ratio       float64
}

type GroupSeriesPoint struct {
	Ts           int64   `json:"ts"`
	RequestCount int64   `json:"request_count"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
}

type GroupModelSummary struct {
	ModelName     string  `json:"model_name"`
	RequestCount  int64   `json:"request_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgTtftMs     int64   `json:"avg_ttft_ms"`
	AvgLatencyMs  int64   `json:"avg_latency_ms"`
	AvgTps        float64 `json:"avg_tps"`
	CacheObserved bool    `json:"cache_observed"`
	CacheHitRate  float64 `json:"cache_hit_rate"`
	// CodingCacheObserved/CodingCacheHitRate observe the cache-creation
	// (5m/1h write) share of cacheable prompt tokens, matching the "Coding
	// cache" metric on the reference performance page.
	CodingCacheObserved bool    `json:"coding_cache_observed"`
	CodingCacheHitRate  float64 `json:"coding_cache_hit_rate"`
}

type GroupSummary struct {
	Group         string              `json:"group"`
	Description   string              `json:"description"`
	Ratio         float64             `json:"ratio"`
	RequestCount  int64               `json:"request_count"`
	SuccessCount  int64               `json:"success_count"`
	AvgTtftMs     int64               `json:"avg_ttft_ms"`
	AvgLatencyMs  int64               `json:"avg_latency_ms"`
	SuccessRate   float64             `json:"success_rate"`
	AvgTps        float64             `json:"avg_tps"`
	CacheObserved bool                `json:"cache_observed"`
	CacheHitRate  float64             `json:"cache_hit_rate"`
	// Coding-cache metrics aggregate the cache-creation share over the
	// whole group window (see GroupModelSummary).
	CodingCacheObserved bool    `json:"coding_cache_observed"`
	CodingCacheHitRate  float64 `json:"coding_cache_hit_rate"`
	Series        []GroupSeriesPoint  `json:"series"`
	Models        []GroupModelSummary `json:"models"`
}

type GroupsResult struct {
	Groups        []GroupSummary `json:"groups"`
	CacheObserved bool           `json:"cache_observed"`
	CacheHitRate  float64        `json:"cache_hit_rate"`
	StartTs       int64          `json:"start_ts"`
	EndTs         int64          `json:"end_ts"`
	BucketSeconds int64          `json:"bucket_seconds"`
}

type bucketKey struct {
	model    string
	group    string
	bucketTs int64
}

type counters struct {
	requestCount        int64
	successCount        int64
	totalLatencyMs      int64
	ttftSumMs           int64
	ttftCount           int64
	outputTokens        int64
	generationMs        int64
	cacheTokens         int64
	cacheCreationTokens int64
	promptTokens        int64
}

type atomicBucket struct {
	requestCount        atomic.Int64
	successCount        atomic.Int64
	totalLatencyMs      atomic.Int64
	ttftSumMs           atomic.Int64
	ttftCount           atomic.Int64
	outputTokens        atomic.Int64
	generationMs        atomic.Int64
	cacheTokens         atomic.Int64
	cacheCreationTokens atomic.Int64
	promptTokens        atomic.Int64
}

func (b *atomicBucket) add(sample Sample) {
	b.requestCount.Add(1)
	if sample.Success {
		b.successCount.Add(1)
	}
	if sample.LatencyMs > 0 {
		b.totalLatencyMs.Add(sample.LatencyMs)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		b.ttftSumMs.Add(sample.TtftMs)
		b.ttftCount.Add(1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		b.outputTokens.Add(sample.OutputTokens)
		b.generationMs.Add(sample.GenerationMs)
	}
	if sample.CacheTokens > 0 {
		b.cacheTokens.Add(sample.CacheTokens)
	}
	if sample.CacheCreationTokens > 0 {
		b.cacheCreationTokens.Add(sample.CacheCreationTokens)
	}
	if sample.PromptTokens > 0 {
		b.promptTokens.Add(sample.PromptTokens)
	}
}

func (b *atomicBucket) snapshot() counters {
	return counters{
		requestCount:        b.requestCount.Load(),
		successCount:        b.successCount.Load(),
		totalLatencyMs:      b.totalLatencyMs.Load(),
		ttftSumMs:           b.ttftSumMs.Load(),
		ttftCount:           b.ttftCount.Load(),
		outputTokens:        b.outputTokens.Load(),
		generationMs:        b.generationMs.Load(),
		cacheTokens:         b.cacheTokens.Load(),
		cacheCreationTokens: b.cacheCreationTokens.Load(),
		promptTokens:        b.promptTokens.Load(),
	}
}

func (b *atomicBucket) drain() counters {
	return counters{
		requestCount:        b.requestCount.Swap(0),
		successCount:        b.successCount.Swap(0),
		totalLatencyMs:      b.totalLatencyMs.Swap(0),
		ttftSumMs:           b.ttftSumMs.Swap(0),
		ttftCount:           b.ttftCount.Swap(0),
		outputTokens:        b.outputTokens.Swap(0),
		generationMs:        b.generationMs.Swap(0),
		cacheTokens:         b.cacheTokens.Swap(0),
		cacheCreationTokens: b.cacheCreationTokens.Swap(0),
		promptTokens:        b.promptTokens.Swap(0),
	}
}

func (b *atomicBucket) addCounters(c counters) {
	if c.requestCount != 0 {
		b.requestCount.Add(c.requestCount)
	}
	if c.successCount != 0 {
		b.successCount.Add(c.successCount)
	}
	if c.totalLatencyMs != 0 {
		b.totalLatencyMs.Add(c.totalLatencyMs)
	}
	if c.ttftSumMs != 0 {
		b.ttftSumMs.Add(c.ttftSumMs)
	}
	if c.ttftCount != 0 {
		b.ttftCount.Add(c.ttftCount)
	}
	if c.outputTokens != 0 {
		b.outputTokens.Add(c.outputTokens)
	}
	if c.generationMs != 0 {
		b.generationMs.Add(c.generationMs)
	}
	if c.cacheTokens != 0 {
		b.cacheTokens.Add(c.cacheTokens)
	}
	if c.cacheCreationTokens != 0 {
		b.cacheCreationTokens.Add(c.cacheCreationTokens)
	}
	if c.promptTokens != 0 {
		b.promptTokens.Add(c.promptTokens)
	}
}
