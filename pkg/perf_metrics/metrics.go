package perfmetrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

var hotBuckets sync.Map

const (
	seriesSchema        = "dbcd0a3c01b55203"
	performanceMaxRows  = 100
	performanceCacheTTL = time.Minute
)

var groupsCache = struct {
	sync.Mutex
	entries map[string]groupsCacheEntry
}{entries: make(map[string]groupsCacheEntry)}

type groupsCacheEntry struct {
	result    GroupsResult
	expiresAt time.Time
}

func Init() {
	go flushLoop()
}

func RecordRelaySample(info *relaycommon.RelayInfo, success bool, outputTokens, cacheTokens, cacheCreationTokens, promptTokens int64) {
	if info == nil {
		return
	}
	now := time.Now()
	hasTtft := info.IsStream && info.HasSendResponse()
	ttftMs := int64(0)
	if hasTtft {
		ttftMs = info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
	}
	latencyMs := now.Sub(info.StartTime).Milliseconds()
	generationMs := latencyMs
	if hasTtft {
		generationMs = now.Sub(info.FirstResponseTime).Milliseconds()
	}
	if generationMs <= 0 {
		generationMs = latencyMs
	}
	Record(Sample{
		Model:               info.OriginModelName,
		Group:               info.UsingGroup,
		LatencyMs:           latencyMs,
		TtftMs:              ttftMs,
		HasTtft:             hasTtft,
		Success:             success,
		OutputTokens:        outputTokens,
		GenerationMs:        generationMs,
		CacheTokens:         cacheTokens,
		CacheCreationTokens: cacheCreationTokens,
		PromptTokens:        promptTokens,
	})
}

func Record(sample Sample) {
	setting := perf_metrics_setting.GetSetting()
	if !setting.Enabled || sample.Model == "" {
		return
	}
	if sample.Group == "" {
		sample.Group = "default"
	}
	if sample.LatencyMs < 0 {
		sample.LatencyMs = 0
	}

	key := bucketKey{
		model:    sample.Model,
		group:    sample.Group,
		bucketTs: bucketStart(time.Now().Unix()),
	}
	actual, _ := hotBuckets.LoadOrStore(key, &atomicBucket{})
	actual.(*atomicBucket).add(sample)
	recordRedis(key, sample)
}

func Query(params QueryParams) (QueryResult, error) {
	if params.Hours <= 0 {
		params.Hours = 24
	}
	if params.Hours > 24*30 {
		params.Hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(params.Hours)*3600

	merged := map[bucketKey]counters{}
	rows, err := model.GetPerfMetrics(params.Model, params.Group, startTs, endTs)
	if err != nil {
		return QueryResult{}, err
	}
	for _, row := range rows {
		mergeCounters(merged, bucketKey{
			model:    row.ModelName,
			group:    row.Group,
			bucketTs: row.BucketTs,
		}, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
		})
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.model != params.Model || k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if params.Group != "" && k.group != params.Group {
			return true
		}
		mergeCounters(merged, k, value.(*atomicBucket).snapshot())
		return true
	})

	return buildQueryResult(params.Model, merged), nil
}

func QuerySummaryAll(hours int, groups []string) (SummaryAllResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600
	allowedGroups := allowedGroupSet(groups)

	rows, err := model.GetPerfMetricsSummaryBucketsAll(startTs, endTs, groups)
	if err != nil {
		return SummaryAllResult{}, err
	}

	totals := map[string]counters{}
	modelBuckets := map[string]map[int64]counters{}
	for _, row := range rows {
		value := counters{
			requestCount:        row.RequestCount,
			successCount:        row.SuccessCount,
			totalLatencyMs:      row.TotalLatencyMs,
			outputTokens:        row.OutputTokens,
			generationMs:        row.GenerationMs,
			cacheTokens:         row.CacheTokens,
			cacheCreationTokens: row.CacheCreationTokens,
			promptTokens:        row.PromptTokens,
		}
		mergeModelTotals(totals, row.ModelName, value)
		mergeModelBucket(modelBuckets, row.ModelName, row.BucketTs, value)
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		snap := value.(*atomicBucket).snapshot()
		if snap.requestCount == 0 {
			return true
		}
		mergeModelTotals(totals, k.model, snap)
		mergeModelBucket(modelBuckets, k.model, k.bucketTs, snap)
		return true
	})

	models := make([]ModelSummary, 0, len(totals))
	for name, total := range totals {
		if total.requestCount == 0 {
			continue
		}
		avgLatency := total.totalLatencyMs / total.requestCount
		successRate := float64(total.successCount) / float64(total.requestCount) * 100
		avgTps := 0.0
		if total.generationMs > 0 {
			avgTps = float64(total.outputTokens) / (float64(total.generationMs) / 1000.0)
		}
		models = append(models, ModelSummary{
			ModelName:          name,
			AvgLatencyMs:       avgLatency,
			SuccessRate:        math.Round(successRate*100) / 100,
			AvgTps:             math.Round(avgTps*100) / 100,
			RecentSuccessRates: recentSuccessRates(modelBuckets[name], 3),
			RequestCount:       total.requestCount,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].RequestCount > models[j].RequestCount
	})

	return SummaryAllResult{Models: models}, nil
}

// QueryGroups aggregates the latest persisted performance records by usable
// group. Results are cached for one minute per effective group visibility set.
// hours limits the aggregation window to the most recent N hours (0 = default).
func QueryGroups(metadata map[string]GroupMetadata, hours int) (GroupsResult, error) {
	return queryGroups(metadata, true, hours)
}

// QueryGroupsAll returns the same group summaries without restricting the
// persisted rows to metadata keys. It is reserved for administrator views so
// historical groups remain inspectable after being disabled.
func QueryGroupsAll(metadata map[string]GroupMetadata, hours int) (GroupsResult, error) {
	return queryGroups(metadata, false, hours)
}

func queryGroups(metadata map[string]GroupMetadata, filterGroups bool, hours int) (GroupsResult, error) {
	groupFilter := metadataKeys(metadata)
	if !filterGroups {
		groupFilter = nil
	}
	cacheKey := fmt.Sprintf("%t:%v:%d", filterGroups, groupFilter, hours)
	now := time.Now()
	groupsCache.Lock()
	if cached, ok := groupsCache.entries[cacheKey]; ok && now.Before(cached.expiresAt) {
		result := cloneGroupsResult(cached.result)
		groupsCache.Unlock()
		return result, nil
	}

	rows, err := model.GetPerfMetricsGroupSummaryBucketsLatest(performanceMaxRows, groupFilter)
	if err != nil {
		groupsCache.Unlock()
		return GroupsResult{}, err
	}

	allowedGroups := allowedGroupSet(groupFilter)
	windowStart := int64(0)
	if hours > 0 {
		windowStart = time.Now().Unix() - int64(hours)*3600
	}
	merged := map[bucketKey]counters{}
	startTs := int64(0)
	endTs := int64(0)
	for _, row := range rows {
		if windowStart > 0 && row.BucketTs < windowStart {
			continue
		}
		if startTs == 0 || row.BucketTs < startTs {
			startTs = row.BucketTs
		}
		if row.BucketTs > endTs {
			endTs = row.BucketTs
		}
		mergeCounters(merged, bucketKey{
			model:    row.ModelName,
			group:    row.Group,
			bucketTs: row.BucketTs,
		}, counters{
			requestCount:        row.RequestCount,
			successCount:        row.SuccessCount,
			totalLatencyMs:      row.TotalLatencyMs,
			ttftSumMs:           row.TtftSumMs,
			ttftCount:           row.TtftCount,
			outputTokens:        row.OutputTokens,
			generationMs:        row.GenerationMs,
			cacheTokens:         row.CacheTokens,
			cacheCreationTokens: row.CacheCreationTokens,
			promptTokens:        row.PromptTokens,
		})
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if windowStart > 0 && k.bucketTs < windowStart {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		mergeCounters(merged, k, value.(*atomicBucket).snapshot())
		return true
	})

	groupBuckets := map[string]map[int64]counters{}
	groupModels := map[string]map[string]counters{}
	for key, value := range merged {
		if value.requestCount == 0 {
			continue
		}
		if _, ok := groupBuckets[key.group]; !ok {
			groupBuckets[key.group] = map[int64]counters{}
		}
		currentBucket := groupBuckets[key.group][key.bucketTs]
		groupBuckets[key.group][key.bucketTs] = addCounters(currentBucket, value)
		if _, ok := groupModels[key.group]; !ok {
			groupModels[key.group] = map[string]counters{}
		}
		currentModel := groupModels[key.group][key.model]
		groupModels[key.group][key.model] = addCounters(currentModel, value)
	}

	groupNames := make([]string, 0, len(metadata)+len(groupBuckets))
	seen := make(map[string]struct{}, len(metadata)+len(groupBuckets))
	for group := range metadata {
		groupNames = append(groupNames, group)
		seen[group] = struct{}{}
	}
	for group := range groupBuckets {
		if _, ok := seen[group]; ok {
			continue
		}
		groupNames = append(groupNames, group)
		seen[group] = struct{}{}
	}

	groups := make([]GroupSummary, 0, len(groupNames))
	var overall counters
	for _, group := range groupNames {
		total := counters{}
		for _, value := range groupBuckets[group] {
			total = addCounters(total, value)
		}

		series := make([]GroupSeriesPoint, 0, len(groupBuckets[group]))
		timestamps := make([]int64, 0, len(groupBuckets[group]))
		for ts := range groupBuckets[group] {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
		for _, ts := range timestamps {
			value := groupBuckets[group][ts]
			series = append(series, GroupSeriesPoint{
				Ts:           ts,
				RequestCount: value.requestCount,
				AvgLatencyMs: avg(value.totalLatencyMs, value.requestCount),
				SuccessRate:  successRate(value),
			})
		}

		models := make([]GroupModelSummary, 0, len(groupModels[group]))
		for modelName, value := range groupModels[group] {
			models = append(models, GroupModelSummary{
				ModelName:           modelName,
				RequestCount:        value.requestCount,
				SuccessRate:         successRate(value),
				AvgTtftMs:           avg(value.ttftSumMs, value.ttftCount),
				AvgLatencyMs:        avg(value.totalLatencyMs, value.requestCount),
				AvgTps:              avgTps(value),
				CacheObserved:       cacheObserved(value),
				CacheHitRate:        math.Round(cacheHitRate(value)*100) / 100,
				CodingCacheObserved: codingCacheObserved(value),
				CodingCacheHitRate:  math.Round(codingCacheHitRate(value)*100) / 100,
			})
		}
		sort.Slice(models, func(i, j int) bool {
			if models[i].RequestCount != models[j].RequestCount {
				return models[i].RequestCount > models[j].RequestCount
			}
			return models[i].ModelName < models[j].ModelName
		})

		meta := metadata[group]
		description := meta.Description
		if description == "" {
			description = group
		}
		overall = addCounters(overall, total)
		groups = append(groups, GroupSummary{
			Group:               group,
			Description:         description,
			Ratio:               meta.Ratio,
			RequestCount:        total.requestCount,
			SuccessCount:        total.successCount,
			AvgTtftMs:           avg(total.ttftSumMs, total.ttftCount),
			AvgLatencyMs:        avg(total.totalLatencyMs, total.requestCount),
			SuccessRate:         successRate(total),
			AvgTps:              avgTps(total),
			CacheObserved:       cacheObserved(total),
			CacheHitRate:        math.Round(cacheHitRate(total)*100) / 100,
			CodingCacheObserved: codingCacheObserved(total),
			CodingCacheHitRate:  math.Round(codingCacheHitRate(total)*100) / 100,
			Series:              series,
			Models:              models,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].RequestCount != groups[j].RequestCount {
			return groups[i].RequestCount > groups[j].RequestCount
		}
		return groups[i].Group < groups[j].Group
	})

	result := GroupsResult{
		Groups:        groups,
		CacheObserved: cacheObserved(overall),
		CacheHitRate:  math.Round(cacheHitRate(overall)*100) / 100,
		StartTs:       startTs,
		EndTs:         endTs,
		BucketSeconds: int64(perf_metrics_setting.GetBucketSeconds()),
	}
	groupsCache.entries[cacheKey] = groupsCacheEntry{
		result:    cloneGroupsResult(result),
		expiresAt: now.Add(performanceCacheTTL),
	}
	groupsCache.Unlock()
	return result, nil
}

func cloneGroupsResult(result GroupsResult) GroupsResult {
	cloned := result
	cloned.Groups = make([]GroupSummary, len(result.Groups))
	copy(cloned.Groups, result.Groups)
	for i := range cloned.Groups {
		cloned.Groups[i].Series = append([]GroupSeriesPoint(nil), result.Groups[i].Series...)
		cloned.Groups[i].Models = append([]GroupModelSummary(nil), result.Groups[i].Models...)
	}
	return cloned
}

func metadataKeys(metadata map[string]GroupMetadata) []string {
	if metadata == nil {
		return nil
	}
	groups := make([]string, 0, len(metadata))
	for group := range metadata {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func addCounters(current, value counters) counters {
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	current.cacheTokens += value.cacheTokens
	current.cacheCreationTokens += value.cacheCreationTokens
	current.promptTokens += value.promptTokens
	return current
}

func mergeModelTotals(totals map[string]counters, modelName string, value counters) {
	if value.requestCount == 0 {
		return
	}
	current := totals[modelName]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	current.cacheTokens += value.cacheTokens
	current.cacheCreationTokens += value.cacheCreationTokens
	current.promptTokens += value.promptTokens
	totals[modelName] = current
}

func mergeModelBucket(modelBuckets map[string]map[int64]counters, modelName string, bucketTs int64, value counters) {
	if value.requestCount == 0 {
		return
	}
	if _, ok := modelBuckets[modelName]; !ok {
		modelBuckets[modelName] = map[int64]counters{}
	}
	current := modelBuckets[modelName][bucketTs]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	current.cacheTokens += value.cacheTokens
	current.cacheCreationTokens += value.cacheCreationTokens
	current.promptTokens += value.promptTokens
	modelBuckets[modelName][bucketTs] = current
}

func recentSuccessRates(buckets map[int64]counters, limit int) []float64 {
	if len(buckets) == 0 || limit <= 0 {
		return nil
	}
	timestamps := make([]int64, 0, len(buckets))
	for ts := range buckets {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	if len(timestamps) > limit {
		timestamps = timestamps[len(timestamps)-limit:]
	}
	rates := make([]float64, 0, len(timestamps))
	for _, ts := range timestamps {
		rates = append(rates, math.Round(successRate(buckets[ts])*100)/100)
	}
	return rates
}

func allowedGroupSet(groups []string) map[string]struct{} {
	if groups == nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		allowed[group] = struct{}{}
	}
	return allowed
}

func bucketStart(ts int64) int64 {
	bucketSeconds := perf_metrics_setting.GetBucketSeconds()
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	return ts - (ts % bucketSeconds)
}

func mergeCounters(merged map[bucketKey]counters, key bucketKey, value counters) {
	if value.requestCount == 0 {
		return
	}
	current := merged[key]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	current.cacheTokens += value.cacheTokens
	current.cacheCreationTokens += value.cacheCreationTokens
	current.promptTokens += value.promptTokens
	merged[key] = current
}

func buildQueryResult(modelName string, merged map[bucketKey]counters) QueryResult {
	groupBuckets := map[string]map[int64]counters{}
	for key, value := range merged {
		if value.requestCount == 0 {
			continue
		}
		if _, ok := groupBuckets[key.group]; !ok {
			groupBuckets[key.group] = map[int64]counters{}
		}
		groupBuckets[key.group][key.bucketTs] = value
	}

	groups := make([]string, 0, len(groupBuckets))
	for group := range groupBuckets {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	results := make([]GroupResult, 0, len(groups))
	for _, group := range groups {
		buckets := groupBuckets[group]
		timestamps := make([]int64, 0, len(buckets))
		for ts := range buckets {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		total := counters{}
		series := make([]BucketPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			value := buckets[ts]
			total.requestCount += value.requestCount
			total.successCount += value.successCount
			total.totalLatencyMs += value.totalLatencyMs
			total.ttftSumMs += value.ttftSumMs
			total.ttftCount += value.ttftCount
			total.outputTokens += value.outputTokens
			total.generationMs += value.generationMs
			series = append(series, bucketPoint(ts, value))
		}

		results = append(results, GroupResult{
			Group:        group,
			AvgTtftMs:    avg(total.ttftSumMs, total.ttftCount),
			AvgLatencyMs: avg(total.totalLatencyMs, total.requestCount),
			SuccessRate:  successRate(total),
			AvgTps:       avgTps(total),
			Series:       series,
		})
	}

	return QueryResult{
		ModelName:    modelName,
		SeriesSchema: seriesSchema,
		Groups:       results,
	}
}

func bucketPoint(ts int64, value counters) BucketPoint {
	return BucketPoint{
		Ts:           ts,
		RequestCount: value.requestCount,
		AvgTtftMs:    avg(value.ttftSumMs, value.ttftCount),
		AvgLatencyMs: avg(value.totalLatencyMs, value.requestCount),
		SuccessRate:  successRate(value),
		AvgTps:       avgTps(value),
	}
}

func avg(sum int64, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return sum / count
}

func successRate(value counters) float64 {
	if value.requestCount <= 0 {
		return 0
	}
	return float64(value.successCount) / float64(value.requestCount) * 100
}

func avgTps(value counters) float64 {
	if value.outputTokens <= 0 || value.generationMs <= 0 {
		return 0
	}
	return float64(value.outputTokens) / (float64(value.generationMs) / 1000)
}

// cacheObserved reports whether any cache or non-cached prompt token count was
// recorded, so the UI can distinguish "0% cache hit" from "no cache data".
func cacheObserved(value counters) bool {
	return value.promptTokens > 0 || value.cacheTokens > 0 || value.cacheCreationTokens > 0
}

// cacheHitRate returns the fraction of prompt input tokens served from the
// prompt cache, excluding cache creation tokens from the numerator.
func cacheHitRate(value counters) float64 {
	total := value.promptTokens + value.cacheTokens + value.cacheCreationTokens
	if total <= 0 {
		return 0
	}
	return float64(value.cacheTokens) / float64(total) * 100
}

// codingCacheObserved reports whether any cache-creation token count was
// recorded, so the UI can distinguish "0% coding cache" from "no data".
func codingCacheObserved(value counters) bool {
	return value.cacheCreationTokens > 0
}

// codingCacheHitRate returns the fraction of cacheable prompt tokens written
// to the cache (5m/1h creation), the "Coding cache" metric on the reference
// performance page.
func codingCacheHitRate(value counters) float64 {
	total := value.promptTokens + value.cacheTokens + value.cacheCreationTokens
	if total <= 0 || value.cacheCreationTokens <= 0 {
		return 0
	}
	return float64(value.cacheCreationTokens) / float64(total) * 100
}

func recordRedis(key bucketKey, sample Sample) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	redisKey := redisBucketKey(key)
	pipe := common.RDB.TxPipeline()
	pipe.HIncrBy(ctx, redisKey, "req", 1)
	if sample.Success {
		pipe.HIncrBy(ctx, redisKey, "ok", 1)
	}
	if sample.LatencyMs > 0 {
		pipe.HIncrBy(ctx, redisKey, "lat", sample.LatencyMs)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		pipe.HIncrBy(ctx, redisKey, "ttft", sample.TtftMs)
		pipe.HIncrBy(ctx, redisKey, "ttft_n", 1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		pipe.HIncrBy(ctx, redisKey, "out", sample.OutputTokens)
		pipe.HIncrBy(ctx, redisKey, "gen_ms", sample.GenerationMs)
	}
	if sample.CacheTokens > 0 {
		pipe.HIncrBy(ctx, redisKey, "cache", sample.CacheTokens)
	}
	if sample.CacheCreationTokens > 0 {
		pipe.HIncrBy(ctx, redisKey, "cache_creation", sample.CacheCreationTokens)
	}
	if sample.PromptTokens > 0 {
		pipe.HIncrBy(ctx, redisKey, "prompt", sample.PromptTokens)
	}
	pipe.Expire(ctx, redisKey, time.Hour)
	_, _ = pipe.Exec(ctx)
}

func mergeRedisActiveBuckets(merged map[bucketKey]counters, params QueryParams, startTs int64, endTs int64) {
	if !common.RedisEnabled || common.RDB == nil || params.Model == "" || params.Group == "" {
		return
	}
	active := bucketStart(time.Now().Unix())
	if active < startTs || active > endTs {
		return
	}
	key := bucketKey{model: params.Model, group: params.Group, bucketTs: active}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	values, err := common.RDB.HGetAll(ctx, redisBucketKey(key)).Result()
	if err != nil || len(values) == 0 {
		return
	}
	mergeCounters(merged, key, redisCounters(values))
}

func redisBucketKey(key bucketKey) string {
	return fmt.Sprintf("perf:%s:%s:%d", key.model, key.group, key.bucketTs)
}
