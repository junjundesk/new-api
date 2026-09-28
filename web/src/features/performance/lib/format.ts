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
import dayjs from 'dayjs'
import type { CSSProperties } from 'react'

/** Placeholder rendered whenever a value is missing (U+2014). */
export const EMPTY_VALUE = '\u2014'

/** Multipliers are always rendered with three decimals and a U+00D7 suffix. */
export function formatMultiplier(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return EMPTY_VALUE
  }
  return `${value.toFixed(3)}\u00d7`
}

/** Group ratio in the header is printed exactly as received. */
export function formatGroupRatio(ratio: number): string {
  return `\u00d7${ratio}`
}

export function formatCount(value: number): string {
  return Number.isFinite(value) ? value.toLocaleString() : EMPTY_VALUE
}

/** Slot tooltip range, e.g. 07-14 09:00 – 10:00 for hourly buckets. */
export function formatSlotRange(ts: number, spanSeconds: number): string {
  const start = dayjs.unix(ts)
  const end = dayjs.unix(ts + spanSeconds)
  if (spanSeconds <= 3600) {
    return `${start.format('MM-DD HH:mm')} \u2013 ${end.format('HH:mm')}`
  }
  return `${start.format('MM-DD HH:mm')} \u2013 ${end.format('MM-DD HH:mm')}`
}

/**
 * Inline style carrying the --perf-delay custom property consumed by the
 * perf-rise-in / perf-bar-grow entrance animations.
 */
export function perfDelayStyle(delayMs: number): CSSProperties {
  return { '--perf-delay': `${delayMs}ms` } as CSSProperties
}
