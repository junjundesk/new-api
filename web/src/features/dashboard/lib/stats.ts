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
import type { QuotaDataItem } from '@/features/dashboard/types'

/**
 * Safe division: handles NaN and Infinity cases
 */
export function safeDivide(
  value: number,
  divisor: number,
  precision: number = 3
): number {
  const result = value / divisor
  if (Number.isNaN(result) || !Number.isFinite(result)) return 0
  const factor = Math.pow(10, precision)
  return Math.round(result * factor) / factor
}

/**
 * Calculate aggregated statistics from quota data
 */
export function calculateDashboardStats(data: QuotaDataItem[]) {
  return data.reduce(
    (acc, item) => ({
      totalQuota: acc.totalQuota + (Number(item.quota) || 0),
      totalCount: acc.totalCount + (Number(item.count) || 0),
      totalTokens: acc.totalTokens + (Number(item.token_used) || 0),
    }),
    { totalQuota: 0, totalCount: 0, totalTokens: 0 }
  )
}

/** Rejects input volumes large enough to be corrupted usage, not real traffic. */
const MAX_CACHE_INPUT_TOKENS = 1_000_000_000

interface CacheRateRow {
  cache_tokens?: number
  cache_input_tokens?: number
}

/**
 * Token-weighted cache hit rate (0-100) across the given rows, or null when no
 * row reports a usable input volume. Both sides are normalized by the server
 * (Anthropic prompt tokens exclude cache reads and writes, OpenAI and Gemini
 * include them), so the rate is a plain division shared by the page stat cards
 * and the consumption breakdown.
 */
export function computeCacheRate(rows: readonly CacheRateRow[]): number | null {
  let cachedTokens = 0
  let inputTokens = 0

  for (const row of rows) {
    const input = row.cache_input_tokens ?? 0
    const cached = row.cache_tokens ?? 0
    if (input <= 0 || input > MAX_CACHE_INPUT_TOKENS) continue
    if (cached < 0 || cached > input) continue
    inputTokens += input
    cachedTokens += cached
  }

  if (inputTokens <= 0) return null
  return Math.min(Math.max((cachedTokens / inputTokens) * 100, 0), 100)
}
