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
import { MAX_REFERENCE_CACHE_RATE } from '@/features/performance-metrics/lib/pricing'

export const DEFAULT_REFERENCE_CACHE_RATE_INPUT = '90'

/** Largest accepted percentage; 100% is rejected upstream (see pricing.ts). */
export const MAX_REFERENCE_CACHE_RATE_PERCENT = 99.9

/**
 * Parses the reference cache rate input as a 0..0.999 fraction. An empty,
 * negative, non-numeric or out-of-range value is null (never a 90% fallback),
 * which is exactly what the pricing function needs to drop the reference side.
 */
export function parseReferenceCacheRate(input: string): number | null {
  if (input.trim() === '') return null
  const percent = Number(input)
  if (!Number.isFinite(percent) || percent < 0) return null
  if (percent > MAX_REFERENCE_CACHE_RATE_PERCENT) return null
  // 99.9 / 100 is 0.9990000000000001 in binary floating point, so the fraction
  // is capped at the documented maximum the pricing function accepts.
  return Math.min(percent / 100, MAX_REFERENCE_CACHE_RATE)
}
