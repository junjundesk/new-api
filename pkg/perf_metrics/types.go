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

	// Coding 输入成本观测：仅当请求被判定为 Coding 客户端、且结算价可按 token
	// 计价时才填充（见 service 采集点与 CodingClient）。
	//
	// CodingInputTokens 是本次请求输入侧可观测 token 总量
	// （未命中缓存输入 + 缓存读取 + 缓存写入），用作性能页
	// coding_input_weight 的原始量；CodingCostInputTokens 是其中「有结算价观测」
	// 的那部分，用作 coding_cost_input_weight。两者量纲一致，因此
	// CodingCostInputTokens <= CodingInputTokens 恒成立。
	CodingInputTokens     int64
	CodingCostInputTokens int64

	// 三个单价分子：token 数 × 单价(USD/1M) × 1e6 的整数和，聚合后除以
	// CodingCostInputTokens 即得该样本的 USD/1M 单价。前端只用这三个原始量做
	// 插值，倍率与参考价全部由浏览器计算。
	//
	//   CodingObservedCostPrice  —— 结算价（含分组倍率与请求级倍率）
	//   CodingCache0CostPrice    —— 0% 缓存参考价（不含分组倍率）
	//   CodingCache100CostPrice  —— 100% 缓存参考价（纯缓存读，不含分组倍率）
	CodingObservedCostPrice int64
	CodingCache0CostPrice   int64
	CodingCache100CostPrice int64
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

	// 性能页价格弹层的原始样本。这里只下发观测量：倍率、参考价、覆盖率都是
	// 浏览器端用这些原始量算出来的。
	//
	// 数值规则（前端判定口径，必须严格保持）：
	//   - CodingInputWeight            ∈ (0, 1]，是同一分组内 Coding 输入 token
	//     总数的归一化权重，不是 token 数；
	//   - CodingCostInputWeight        ∈ (0, CodingInputWeight]，成本侧权重；
	//   - CodingCacheHitRate           百分数 0..100；
	//   - CodingCostObserved           仅当输入侧单价可观测时为 true；
	//   - CodingObservedInputPrice 等  USD / 1M input tokens，不可观测时为 null。
	CodingInputWeight        *float64 `json:"coding_input_weight"`
	CodingCostInputWeight    *float64 `json:"coding_cost_input_weight"`
	CodingCostObserved       bool     `json:"coding_cost_observed"`
	CodingObservedInputPrice *float64 `json:"coding_observed_input_price"`
	CodingCache0InputPrice   *float64 `json:"coding_cache0_input_price"`
	CodingCache100InputPrice *float64 `json:"coding_cache100_input_price"`
}

// GroupMetricSummary is the nested per-group aggregate. It carries exactly the
// same observables as the flat GroupSummary fields; the reference performance
// page reads them from summary.* while existing consumers keep reading the flat
// fields. Every value is a raw observation, never a derived price or ratio.
type GroupMetricSummary struct {
	RequestCount        int64   `json:"request_count"`
	SuccessCount        int64   `json:"success_count"`
	SuccessRate         float64 `json:"success_rate"`
	AvgTtftMs           int64   `json:"avg_ttft_ms"`
	AvgLatencyMs        int64   `json:"avg_latency_ms"`
	AvgTps              float64 `json:"avg_tps"`
	CacheObserved       bool    `json:"cache_observed"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
	CodingCacheObserved bool    `json:"coding_cache_observed"`
	CodingCacheHitRate  float64 `json:"coding_cache_hit_rate"`
}

type GroupSummary struct {
	Group         string  `json:"group"`
	Description   string  `json:"description"`
	Ratio         float64 `json:"ratio"`
	RequestCount  int64   `json:"request_count"`
	SuccessCount  int64   `json:"success_count"`
	AvgTtftMs     int64   `json:"avg_ttft_ms"`
	AvgLatencyMs  int64   `json:"avg_latency_ms"`
	SuccessRate   float64 `json:"success_rate"`
	AvgTps        float64 `json:"avg_tps"`
	CacheObserved bool    `json:"cache_observed"`
	CacheHitRate  float64 `json:"cache_hit_rate"`
	// Coding-cache metrics aggregate the cache-creation share over the
	// whole group window (see GroupModelSummary).
	CodingCacheObserved bool                `json:"coding_cache_observed"`
	CodingCacheHitRate  float64             `json:"coding_cache_hit_rate"`
	Summary             *GroupMetricSummary `json:"summary"`
	Series              []GroupSeriesPoint  `json:"series"`
	Models              []GroupModelSummary `json:"models"`

	// modelCounters is the per-model aggregate the sample normalization needs;
	// it never leaves the server.
	modelCounters map[string]counters `json:"-"`
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
	// Coding 输入成本观测的原始量（见 Sample）。
	codingInputTokens     int64
	codingCostInputTokens int64
	codingObservedCost    int64
	codingCache0Cost      int64
	codingCache100Cost    int64
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

	codingInputTokens     atomic.Int64
	codingCostInputTokens atomic.Int64
	codingObservedCost    atomic.Int64
	codingCache0Cost      atomic.Int64
	codingCache100Cost    atomic.Int64
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
	if sample.CodingInputTokens > 0 {
		b.codingInputTokens.Add(sample.CodingInputTokens)
	}
	if sample.CodingCostInputTokens > 0 {
		b.codingCostInputTokens.Add(sample.CodingCostInputTokens)
		b.codingObservedCost.Add(sample.CodingObservedCostPrice)
		b.codingCache0Cost.Add(sample.CodingCache0CostPrice)
		b.codingCache100Cost.Add(sample.CodingCache100CostPrice)
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

		codingInputTokens:     b.codingInputTokens.Load(),
		codingCostInputTokens: b.codingCostInputTokens.Load(),
		codingObservedCost:    b.codingObservedCost.Load(),
		codingCache0Cost:      b.codingCache0Cost.Load(),
		codingCache100Cost:    b.codingCache100Cost.Load(),
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

		codingInputTokens:     b.codingInputTokens.Swap(0),
		codingCostInputTokens: b.codingCostInputTokens.Swap(0),
		codingObservedCost:    b.codingObservedCost.Swap(0),
		codingCache0Cost:      b.codingCache0Cost.Swap(0),
		codingCache100Cost:    b.codingCache100Cost.Swap(0),
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
	if c.codingInputTokens != 0 {
		b.codingInputTokens.Add(c.codingInputTokens)
	}
	if c.codingCostInputTokens != 0 {
		b.codingCostInputTokens.Add(c.codingCostInputTokens)
	}
	if c.codingObservedCost != 0 {
		b.codingObservedCost.Add(c.codingObservedCost)
	}
	if c.codingCache0Cost != 0 {
		b.codingCache0Cost.Add(c.codingCache0Cost)
	}
	if c.codingCache100Cost != 0 {
		b.codingCache100Cost.Add(c.codingCache100Cost)
	}
}
