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
import type { LogOtherData } from '../types'
import { parseLogOther } from './format'

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
 * Token-weighted cache hit rate (0-100) over the log rows shown on the current
 * page, or null when none of those rows reports input tokens.
 *
 * Cache reads / writes and prompt tokens are reported with different semantics
 * per upstream: Claude's input tokens exclude cache reads and writes, while
 * OpenAI and Gemini already include cached tokens in the prompt total. The
 * denominator therefore rebuilds the full input volume row by row so the rate
 * stays comparable across providers.
 */
export function getPageCacheRate(logs: readonly UsageLog[]): number | null {
  let cachedTokens = 0
  let inputTokens = 0

  for (const log of logs) {
    const other: LogOtherData | null = parseLogOther(log.other)
    const cacheRead = normalizeTokenCount(other?.cache_tokens)
    const cacheWrite = Math.max(
      normalizeTokenCount(other?.cache_creation_tokens),
      normalizeTokenCount(other?.cache_creation_tokens_5m) +
        normalizeTokenCount(other?.cache_creation_tokens_1h)
    )
    const promptTokens = normalizeTokenCount(log.prompt_tokens)
    const totalInput = other?.claude
      ? promptTokens + cacheRead + cacheWrite
      : promptTokens
    if (totalInput <= 0) continue

    cachedTokens += cacheRead
    inputTokens += totalInput
  }

  if (inputTokens <= 0) return null

  const rate = (cachedTokens / inputTokens) * 100
  return Math.min(Math.max(rate, 0), 100)
}
