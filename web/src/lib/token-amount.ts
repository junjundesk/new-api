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

// ============================================================================
// Token amount formatting / parsing
// ============================================================================
// Token package totals are plain token counts. The UI shows them with the
// largest unit that still yields a value >= 1, so 10,000,000 renders as
// "1000万" in Chinese instead of "0.1亿". English uses the K/M/B units.
// The numeric part is truncated (never rounded) so a value just below the next
// unit can never be displayed as that unit.

export const TOKEN_AMOUNT_MAX = 1_000_000_000_000_000

type TokenUnit = {
  factor: number
  suffix: string
}

const ZH_UNITS: TokenUnit[] = [
  { factor: 100_000_000, suffix: '亿' },
  { factor: 10_000, suffix: '万' },
  { factor: 1_000, suffix: '千' },
]

const EN_UNITS: TokenUnit[] = [
  { factor: 1_000_000_000, suffix: 'B' },
  { factor: 1_000_000, suffix: 'M' },
  { factor: 1_000, suffix: 'K' },
]

function isChineseLanguage(language?: string): boolean {
  return (language ?? '').toLowerCase().startsWith('zh')
}

function formatScaledNumber(value: number): string {
  const truncated = Math.trunc(value * 100) / 100
  if (Number.isInteger(truncated)) return String(truncated)
  return truncated.toFixed(2).replace(/\.?0+$/, '')
}

/**
 * Format a token count with the site's display units.
 *
 * zh: 千/万/亿, en: K/M/B. The chosen unit is the largest one whose factor is
 * <= value, so 1000万 never becomes 0.1亿.
 */
export function formatTokenAmount(
  value: number | null | undefined,
  language?: string
): string {
  if (value == null || !Number.isFinite(value) || value < 0) return '-'
  if (value === 0) return '0'
  const units = isChineseLanguage(language) ? ZH_UNITS : EN_UNITS
  for (const unit of units) {
    if (value >= unit.factor) {
      return `${formatScaledNumber(value / unit.factor)}${unit.suffix}`
    }
  }
  return String(Math.trunc(value))
}

const TOKEN_INPUT_PATTERN = /^(\d+(?:\.\d+)?)\s*([kKmMbB]|[千万亿])?$/
const TOKEN_INPUT_MULTIPLIERS: Record<string, number> = {
  k: 1_000,
  m: 1_000_000,
  b: 1_000_000_000,
  千: 1_000,
  万: 10_000,
  亿: 100_000_000,
}

export type ParseTokenAmountResult =
  | { ok: true; value: number }
  | { ok: false }

/**
 * Parse an editable token total. Accepts plain digits and the K/M/B or
 * 千/万/亿 suffixes (case-insensitive, optional whitespace before the suffix).
 * Empty, malformed, non-positive and out-of-range input is rejected; the form
 * decides whether 0 or empty means "unlimited".
 */
export function parseTokenAmountInput(input: string): ParseTokenAmountResult {
  const trimmed = input.trim()
  if (trimmed === '') return { ok: false }
  const match = TOKEN_INPUT_PATTERN.exec(trimmed)
  if (!match) return { ok: false }

  const amount = Number(match[1])
  if (!Number.isFinite(amount)) return { ok: false }

  const rawSuffix = match[2]
  const suffix = rawSuffix ? rawSuffix.toLowerCase() : ''
  const multiplier = suffix === '' ? 1 : TOKEN_INPUT_MULTIPLIERS[suffix]
  if (!multiplier) return { ok: false }

  const value = Math.round(amount * multiplier)
  if (!Number.isSafeInteger(value) || value < 0 || value > TOKEN_AMOUNT_MAX) {
    return { ok: false }
  }
  return { ok: true, value }
}
