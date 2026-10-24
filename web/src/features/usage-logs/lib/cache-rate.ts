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

import type { UsageLog } from '../data/schema'
import { parseLogOther } from './format'

/**
 * Upper bound for a log's complete input volume. Upstream usage beyond this is
 * not trustworthy, so the per-log cache badge stays hidden instead of showing a
 * meaningless rate.
 */
const MAX_CACHE_INPUT_TOKENS = 1_000_000_000

export interface CacheTokenTotals {
  /** Prompt tokens served from cache (cache reads). */
  cachedTokens: number
  /** Complete input volume: uncached prompt plus cache reads and cache writes. */
  inputTokens: number
}

export interface LogCacheUsage extends CacheTokenTotals {
  /** Cache hit rate in percent (0-100). */
  rate: number
}

export type CacheHitRateLevel =
  | 'success-subtle'
  | 'warning-subtle'
  | 'danger-subtle'

/**
 * Normalizes a log token count. Log fields are optional and upstream-controlled,
 * so strings, negative values, and non-finite numbers never enter a rate.
 */
function normalizeTokenCount(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? value
    : 0
}

/**
 * Rebuilds the cache reads and the complete input volume of a single log row.
 *
 * Cache reads / writes and prompt tokens are reported with different semantics
 * per upstream: Claude's input tokens exclude cache reads and writes, while
 * OpenAI and Gemini already include cached tokens in the prompt total. The
 * denominator therefore rebuilds the full input volume so the rate stays
 * comparable across providers.
 */
export function getCacheTokenTotals(log: UsageLog): CacheTokenTotals | null {
  const other = parseLogOther(log.other)
  const cacheRead = normalizeTokenCount(other?.cache_tokens)
  const cacheWrite = Math.max(
    normalizeTokenCount(other?.cache_creation_tokens),
    normalizeTokenCount(other?.cache_creation_tokens_5m) +
      normalizeTokenCount(other?.cache_creation_tokens_1h)
  )
  const promptTokens = normalizeTokenCount(log.prompt_tokens)
  const inputTokens = other?.claude
    ? promptTokens + cacheRead + cacheWrite
    : promptTokens
  if (inputTokens <= 0) return null

  return { cachedTokens: cacheRead, inputTokens }
}

/**
 * Token-weighted cache hit rate (0-100) over the log rows shown on the current
 * page, or null when none of those rows reports input tokens.
 */
export function getPageCacheRate(logs: readonly UsageLog[]): number | null {
  let cachedTokens = 0
  let inputTokens = 0

  for (const log of logs) {
    const totals = getCacheTokenTotals(log)
    if (!totals) continue

    cachedTokens += totals.cachedTokens
    inputTokens += totals.inputTokens
  }

  if (inputTokens <= 0) return null

  const rate = (cachedTokens / inputTokens) * 100
  return Math.min(Math.max(rate, 0), 100)
}

/**
 * Cache hit rate of a single log row, or null when its usage cannot back a rate.
 * Counts must be safe integers bounded by the input volume, otherwise the row
 * carries corrupted or unrepresentable usage and no badge is shown.
 */
export function getLogCacheUsage(log: UsageLog): LogCacheUsage | null {
  const totals = getCacheTokenTotals(log)
  if (!totals) return null

  const { cachedTokens, inputTokens } = totals
  if (
    !Number.isSafeInteger(cachedTokens) ||
    !Number.isSafeInteger(inputTokens) ||
    inputTokens > MAX_CACHE_INPUT_TOKENS ||
    cachedTokens > inputTokens
  ) {
    return null
  }

  return { cachedTokens, inputTokens, rate: (cachedTokens / inputTokens) * 100 }
}

/** Maps a rate to the badge tone: green above 90%, yellow from 50%, red below. */
export function getCacheHitRateLevel(rate: number): CacheHitRateLevel {
  if (rate > 90) return 'success-subtle'
  if (rate >= 50) return 'warning-subtle'
  return 'danger-subtle'
}

/**
 * Formats a rate with one decimal. Rates that round down to 0.0% read as
 * "<0.1%" and rates that floor to 90.0% while still being above 90% read as
 * ">90.0%" so the sign never contradicts the badge tone.
 */
export function formatCacheHitRate(
  usage: LogCacheUsage,
  locale?: string
): string {
  const perMille = Math.floor((1000 * usage.cachedTokens) / usage.inputTokens)
  const percent = (Math.max(perMille, 1) / 10).toLocaleString(locale, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })

  if (perMille === 0) return `<${percent}%`
  if (perMille === 900 && usage.rate > 90) return `>${percent}%`
  return `${percent}%`
}
