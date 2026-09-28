package model

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PerfMetric stores aggregated relay performance metrics for the model square.
type PerfMetric struct {
	Id                  int    `json:"id" gorm:"primaryKey"`
	ModelName           string `json:"model_name" gorm:"size:128;uniqueIndex:idx_perf_model_group_bucket,priority:1"`
	Group               string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_perf_model_group_bucket,priority:2"`
	BucketTs            int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_perf_model_group_bucket,priority:3;index:idx_perf_bucket_ts"`
	RequestCount        int64  `json:"-" gorm:"default:0"`
	SuccessCount        int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs      int64  `json:"-" gorm:"default:0"`
	TtftSumMs           int64  `json:"-" gorm:"default:0"`
	TtftCount           int64  `json:"-" gorm:"default:0"`
	OutputTokens        int64  `json:"-" gorm:"default:0"`
	GenerationMs        int64  `json:"-" gorm:"default:0"`
	CacheTokens         int64  `json:"-" gorm:"default:0"` // prompt cache 命中 token 累计
	CacheCreationTokens int64  `json:"-" gorm:"default:0"` // prompt cache 创建 token 累计
	PromptTokens        int64  `json:"-" gorm:"default:0"` // 输入 token（未命中缓存部分）累计

	// Coding 输入成本观测（语义见 pkg/perf_metrics/coding_pricing.go）。
	// CodingInputTokens 是 Coding 客户端请求的输入 token 总量
	// （未命中缓存输入 + 缓存读取 + 缓存写入），用于还原 coding_input_weight；
	// CodingCostInputTokens 是其中输入单价可观测的那部分，用于还原
	// coding_cost_input_weight 与覆盖率。
	//
	// 三个 Cost 列是「token 数 × USD/1M 单价 × 1e6」的整数和：聚合后除以
	// CodingCostInputTokens 即得 USD/1M 单价，因此可以直接求和下发。
	CodingInputTokens     int64 `json:"-" gorm:"default:0"`
	CodingCostInputTokens int64 `json:"-" gorm:"default:0"`
	// 结算价（含分组倍率与请求级倍率）的分子。
	CodingObservedCost int64 `json:"-" gorm:"default:0"`
	// 0% / 100% 缓存参考价（不含分组倍率）的分子。
	CodingCache0Cost   int64 `json:"-" gorm:"default:0"`
	CodingCache100Cost int64 `json:"-" gorm:"default:0"`
}

func (PerfMetric) TableName() string {
	return "perf_metrics"
}

func UpsertPerfMetric(metric *PerfMetric) error {
	if metric == nil || metric.RequestCount == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "model_name"},
			{Name: "group"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count":         gorm.Expr("perf_metrics.request_count + ?", metric.RequestCount),
			"success_count":         gorm.Expr("perf_metrics.success_count + ?", metric.SuccessCount),
			"total_latency_ms":      gorm.Expr("perf_metrics.total_latency_ms + ?", metric.TotalLatencyMs),
			"ttft_sum_ms":           gorm.Expr("perf_metrics.ttft_sum_ms + ?", metric.TtftSumMs),
			"ttft_count":            gorm.Expr("perf_metrics.ttft_count + ?", metric.TtftCount),
			"output_tokens":         gorm.Expr("perf_metrics.output_tokens + ?", metric.OutputTokens),
			"generation_ms":         gorm.Expr("perf_metrics.generation_ms + ?", metric.GenerationMs),
			"cache_tokens":          gorm.Expr("perf_metrics.cache_tokens + ?", metric.CacheTokens),
			"cache_creation_tokens": gorm.Expr("perf_metrics.cache_creation_tokens + ?", metric.CacheCreationTokens),
			"prompt_tokens":         gorm.Expr("perf_metrics.prompt_tokens + ?", metric.PromptTokens),

			"coding_input_tokens":      gorm.Expr("perf_metrics.coding_input_tokens + ?", metric.CodingInputTokens),
			"coding_cost_input_tokens": gorm.Expr("perf_metrics.coding_cost_input_tokens + ?", metric.CodingCostInputTokens),
			"coding_observed_cost":     gorm.Expr("perf_metrics.coding_observed_cost + ?", metric.CodingObservedCost),
			"coding_cache0_cost":       gorm.Expr("perf_metrics.coding_cache0_cost + ?", metric.CodingCache0Cost),
			"coding_cache100_cost":     gorm.Expr("perf_metrics.coding_cache100_cost + ?", metric.CodingCache100Cost),
		}),
	}).Create(metric).Error
}

func GetPerfMetrics(modelName string, group string, startTs int64, endTs int64) ([]PerfMetric, error) {
	var metrics []PerfMetric
	query := DB.Model(&PerfMetric{}).
		Where("model_name = ? AND bucket_ts >= ? AND bucket_ts <= ?", modelName, startTs, endTs)
	if group != "" {
		query = query.Where(commonGroupCol+" = ?", group)
	}
	err := query.Order("bucket_ts ASC").Find(&metrics).Error
	return metrics, err
}

type PerfMetricSummary struct {
	ModelName           string `json:"model_name"`
	RequestCount        int64  `json:"request_count"`
	SuccessCount        int64  `json:"success_count"`
	TotalLatencyMs      int64  `json:"total_latency_ms"`
	OutputTokens        int64  `json:"output_tokens"`
	GenerationMs        int64  `json:"generation_ms"`
	CacheTokens         int64  `json:"cache_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	PromptTokens        int64  `json:"prompt_tokens"`
}

type PerfMetricSummaryBucket struct {
	ModelName           string `json:"model_name"`
	BucketTs            int64  `json:"bucket_ts"`
	RequestCount        int64  `json:"request_count"`
	SuccessCount        int64  `json:"success_count"`
	TotalLatencyMs      int64  `json:"total_latency_ms"`
	OutputTokens        int64  `json:"output_tokens"`
	GenerationMs        int64  `json:"generation_ms"`
	CacheTokens         int64  `json:"cache_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	PromptTokens        int64  `json:"prompt_tokens"`
}

// PerfMetricGroupSummaryBucket is the database projection used by the
// performance overview page. Keeping the group in the projection lets the
// endpoint aggregate every model in one query instead of issuing one query
// per model from the browser.
type PerfMetricGroupSummaryBucket struct {
	ModelName           string `json:"model_name"`
	Group               string `json:"group"`
	BucketTs            int64  `json:"bucket_ts"`
	RequestCount        int64  `json:"request_count"`
	SuccessCount        int64  `json:"success_count"`
	TotalLatencyMs      int64  `json:"total_latency_ms"`
	TtftSumMs           int64  `json:"ttft_sum_ms"`
	TtftCount           int64  `json:"ttft_count"`
	OutputTokens        int64  `json:"output_tokens"`
	GenerationMs        int64  `json:"generation_ms"`
	CacheTokens         int64  `json:"cache_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	PromptTokens        int64  `json:"prompt_tokens"`

	CodingInputTokens     int64 `json:"coding_input_tokens"`
	CodingCostInputTokens int64 `json:"coding_cost_input_tokens"`
	CodingObservedCost    int64 `json:"coding_observed_cost"`
	CodingCache0Cost      int64 `json:"coding_cache0_cost"`
	CodingCache100Cost    int64 `json:"coding_cache100_cost"`
}

func GetPerfMetricsSummaryAll(startTs int64, endTs int64, groups []string) ([]PerfMetricSummary, error) {
	var summaries []PerfMetricSummary
	query := DB.Model(&PerfMetric{}).
		Select("model_name, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms, SUM(cache_tokens) as cache_tokens, SUM(cache_creation_tokens) as cache_creation_tokens, SUM(prompt_tokens) as prompt_tokens").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Group("model_name").
		Having("SUM(request_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func GetPerfMetricsSummaryBucketsAll(startTs int64, endTs int64, groups []string) ([]PerfMetricSummaryBucket, error) {
	var summaries []PerfMetricSummaryBucket
	query := DB.Model(&PerfMetric{}).
		Select("model_name, bucket_ts, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms, SUM(cache_tokens) as cache_tokens, SUM(cache_creation_tokens) as cache_creation_tokens, SUM(prompt_tokens) as prompt_tokens").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Group("model_name, bucket_ts").
		Having("SUM(request_count) > 0").
		Order("bucket_ts ASC").
		Find(&summaries).Error
	return summaries, err
}

func GetPerfMetricsGroupSummaryBucketsLatest(limit int, groups []string) ([]PerfMetricGroupSummaryBucket, error) {
	var summaries []PerfMetricGroupSummaryBucket
	if limit <= 0 {
		limit = 100
	}
	query := DB.Model(&PerfMetric{}).
		Select("model_name, " + commonGroupCol + ", bucket_ts, request_count, success_count, total_latency_ms, ttft_sum_ms, ttft_count, output_tokens, generation_ms, cache_tokens, cache_creation_tokens, prompt_tokens, coding_input_tokens, coding_cost_input_tokens, coding_observed_cost, coding_cache0_cost, coding_cache100_cost")
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Where("request_count > 0").
		Order("bucket_ts DESC").
		Order("id DESC").
		Limit(limit).
		Find(&summaries).Error
	return summaries, err
}

func GetPerfMetricsGroupSummaryBucketsAll(startTs int64, endTs int64, groups []string) ([]PerfMetricGroupSummaryBucket, error) {
	var summaries []PerfMetricGroupSummaryBucket
	query := DB.Model(&PerfMetric{}).
		Select("model_name, "+commonGroupCol+", bucket_ts, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(ttft_sum_ms) as ttft_sum_ms, SUM(ttft_count) as ttft_count, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms, SUM(cache_tokens) as cache_tokens, SUM(cache_creation_tokens) as cache_creation_tokens, SUM(prompt_tokens) as prompt_tokens, SUM(coding_input_tokens) as coding_input_tokens, SUM(coding_cost_input_tokens) as coding_cost_input_tokens, SUM(coding_observed_cost) as coding_observed_cost, SUM(coding_cache0_cost) as coding_cache0_cost, SUM(coding_cache100_cost) as coding_cache100_cost").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Group("model_name, " + commonGroupCol + ", bucket_ts").
		Having("SUM(request_count) > 0").
		Order("bucket_ts ASC").
		Find(&summaries).Error
	return summaries, err
}

func DeletePerfMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&PerfMetric{}).Error
}

func PerfMetricStartTime(hours int) int64 {
	if hours <= 0 {
		hours = 24
	}
	return time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
}
