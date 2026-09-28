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
import type { PerformanceGroupSeriesPoint } from '@/features/performance-metrics/types'

/** Upper bound on rendered bars; each bar aggregates ceil(buckets / 48) of them. */
export const AVAILABILITY_MAX_SLOTS = 48

export const DEFAULT_BUCKET_SECONDS = 3600

export type AvailabilityWindow = {
  bucketSeconds: number
  startTs: number
  endTs: number
}

export type AvailabilitySlot = {
  ts: number
  spanSeconds: number
  requestCount: number
  successRate: number | null
  avgLatencyMs: number
}

/**
 * Groups raw series buckets into the rendered bars: weighted success rate and
 * latency, null success rate for buckets without traffic, and a window aligned
 * to the bucket grid (empty when the window is inverted).
 */
export function bucketAvailability(
  series: PerformanceGroupSeriesPoint[],
  window: AvailabilityWindow,
  maxSlots: number = AVAILABILITY_MAX_SLOTS
): AvailabilitySlot[] {
  const bucket =
    Number.isFinite(window.bucketSeconds) && window.bucketSeconds > 0
      ? window.bucketSeconds
      : DEFAULT_BUCKET_SECONDS
  const windowStart = Math.ceil(window.startTs / bucket) * bucket
  const windowEnd = Math.floor(window.endTs / bucket) * bucket
  if (windowEnd < windowStart) return []

  const rawCount = Math.floor((windowEnd - windowStart) / bucket) + 1
  const groupSize = Math.max(1, Math.ceil(rawCount / Math.max(1, maxSlots)))
  const byTs = new Map(series.map((point) => [point.ts, point]))

  const slots: AvailabilitySlot[] = []
  for (
    let start = windowStart;
    start <= windowEnd;
    start += bucket * groupSize
  ) {
    let requestCount = 0
    let successes = 0
    let latencyTotal = 0
    for (let index = 0; index < groupSize; index += 1) {
      const point = byTs.get(start + index * bucket)
      if (!point || !(point.request_count > 0)) continue
      requestCount += point.request_count
      successes += Math.round((point.success_rate / 100) * point.request_count)
      latencyTotal += point.avg_latency_ms * point.request_count
    }
    slots.push({
      ts: start,
      spanSeconds: bucket * groupSize,
      requestCount,
      successRate:
        requestCount > 0
          ? Math.round((successes / requestCount) * 1e4) / 100
          : null,
      avgLatencyMs:
        requestCount > 0 ? Math.round(latencyTotal / requestCount) : 0,
    })
  }
  return slots
}

/** Bar height steps; buckets without traffic keep the 30% stub. */
export function getAvailabilityBarHeightClass(
  successRate: number | null
): string {
  if (successRate === null || !Number.isFinite(successRate)) return 'h-[30%]'
  if (successRate >= 99.9) return 'h-full'
  if (successRate >= 99) return 'h-[88%]'
  if (successRate >= 95) return 'h-[72%]'
  if (successRate >= 90) return 'h-[55%]'
  return 'h-[42%]'
}
