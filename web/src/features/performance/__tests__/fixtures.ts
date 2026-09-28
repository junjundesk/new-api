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
import type {
  PerformanceGroupModelSummary,
  PerformanceGroupSummary,
} from '@/features/performance-metrics/types'

export function makeModel(
  overrides: Partial<PerformanceGroupModelSummary> & { model_name: string }
): PerformanceGroupModelSummary {
  return {
    request_count: 0,
    success_rate: 100,
    avg_ttft_ms: 0,
    avg_latency_ms: 0,
    avg_tps: 0,
    cache_observed: true,
    cache_hit_rate: 0,
    ...overrides,
  }
}

export function makeGroup(
  overrides: Partial<PerformanceGroupSummary> & { group: string }
): PerformanceGroupSummary {
  return {
    description: '',
    ratio: 1,
    request_count: 0,
    success_count: 0,
    avg_ttft_ms: 0,
    avg_latency_ms: 0,
    success_rate: 100,
    avg_tps: 0,
    cache_observed: true,
    cache_hit_rate: 0,
    series: [],
    models: [],
    ...overrides,
  }
}
