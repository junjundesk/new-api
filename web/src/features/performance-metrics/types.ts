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
export type PerformanceSeriesPoint = {
  ts: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
}

export type PerformanceGroup = {
  group: string
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  series: PerformanceSeriesPoint[]
}

export type PerformanceMetricsData = {
  success: boolean
  message?: string
  data: {
    model_name: string
    series_schema?: string
    groups: PerformanceGroup[]
  }
}

export type PerfModelSummary = {
  model_name: string
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
  request_count?: number
}

export type PerfSummaryAllData = {
  success: boolean
  message?: string
  data: {
    models: PerfModelSummary[]
  }
}

export type PerformanceGroupSeriesPoint = {
  ts: number
  request_count: number
  avg_latency_ms: number
  success_rate: number
}

export type PerformanceGroupModelSummary = {
  model_name: string
  request_count: number
  success_rate: number
  avg_ttft_ms: number
  avg_latency_ms: number
  avg_tps: number
  cache_observed: boolean
  cache_hit_rate: number
  coding_cache_observed?: boolean
  coding_cache_hit_rate?: number
  /** Normalized share of the group Coding input tokens, (0, 1]. */
  coding_input_weight?: number
  /** Must be exactly true before the sample counts as a cost observation. */
  coding_cost_observed?: boolean
  /** Cost-observed share of the group input, (0, coding_input_weight]. */
  coding_cost_input_weight?: number
  /** Settled input price, USD per 1M input tokens (group multiplier applied). */
  coding_observed_input_price?: number
  /** Input price with 0% cache reads, USD per 1M input tokens. */
  coding_cache0_input_price?: number
  /** Input price with 100% cache reads, USD per 1M input tokens. */
  coding_cache100_input_price?: number
}

export type PerformanceGroupSummary = {
  group: string
  /** Display name; falls back to group when the backend omits it. */
  name?: string
  description: string
  ratio: number
  request_count: number
  success_count: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  cache_observed: boolean
  cache_hit_rate: number
  coding_cache_observed?: boolean
  coding_cache_hit_rate?: number
  series: PerformanceGroupSeriesPoint[]
  models: PerformanceGroupModelSummary[]
}

export type PerfGroupsData = {
  success: boolean
  message?: string
  data: {
    groups: PerformanceGroupSummary[]
    cache_observed: boolean
    cache_hit_rate: number
    start_ts: number
    end_ts: number
    bucket_seconds: number
  }
}
