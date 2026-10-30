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
import { describe, expect, it } from 'vitest'

import {
  TOKEN_AMOUNT_MAX,
  formatTokenAmount,
  parseTokenAmountInput,
} from '../token-amount'

describe('formatTokenAmount', () => {
  it('uses Chinese 千/万/亿 units and never shows 0.1亿 for 1000万', () => {
    expect(formatTokenAmount(999, 'zhCN')).toBe('999')
    expect(formatTokenAmount(1_000, 'zhCN')).toBe('1千')
    expect(formatTokenAmount(1_500, 'zhCN')).toBe('1.5千')
    expect(formatTokenAmount(9_999, 'zhCN')).toBe('9.99千')
    expect(formatTokenAmount(10_000, 'zhCN')).toBe('1万')
    expect(formatTokenAmount(12_345, 'zhCN')).toBe('1.23万')
    expect(formatTokenAmount(10_000_000, 'zhCN')).toBe('1000万')
    expect(formatTokenAmount(99_999_999, 'zhCN')).toBe('9999.99万')
    expect(formatTokenAmount(100_000_000, 'zhCN')).toBe('1亿')
    expect(formatTokenAmount(123_456_789, 'zhCN')).toBe('1.23亿')
  })

  it('uses K/M/B units for English', () => {
    expect(formatTokenAmount(999, 'en')).toBe('999')
    expect(formatTokenAmount(1_000, 'en')).toBe('1K')
    expect(formatTokenAmount(1_500, 'en')).toBe('1.5K')
    expect(formatTokenAmount(1_000_000, 'en')).toBe('1M')
    expect(formatTokenAmount(1_234_567, 'en')).toBe('1.23M')
    expect(formatTokenAmount(1_000_000_000, 'en')).toBe('1B')
  })

  it('handles invalid values without throwing', () => {
    expect(formatTokenAmount(0, 'zhCN')).toBe('0')
    expect(formatTokenAmount(Number.NaN, 'zhCN')).toBe('-')
    expect(formatTokenAmount(-1, 'en')).toBe('-')
    expect(formatTokenAmount(null, 'en')).toBe('-')
  })
})

describe('parseTokenAmountInput', () => {
  it('accepts plain numbers and K/M/B suffixes case-insensitively', () => {
    expect(parseTokenAmountInput('1000')).toEqual({ ok: true, value: 1_000 })
    expect(parseTokenAmountInput('1.5K')).toEqual({ ok: true, value: 1_500 })
    expect(parseTokenAmountInput('2m')).toEqual({ ok: true, value: 2_000_000 })
    expect(parseTokenAmountInput('3B')).toEqual({ ok: true, value: 3_000_000_000 })
    expect(parseTokenAmountInput('1.5 k')).toEqual({ ok: true, value: 1_500 })
  })

  it('accepts Chinese 千/万/亿 suffixes with optional spaces', () => {
    expect(parseTokenAmountInput('1000万')).toEqual({ ok: true, value: 10_000_000 })
    expect(parseTokenAmountInput('1.2亿')).toEqual({ ok: true, value: 120_000_000 })
    expect(parseTokenAmountInput(' 10 万 ')).toEqual({ ok: true, value: 100_000 })
    expect(parseTokenAmountInput('2千')).toEqual({ ok: true, value: 2_000 })
  })

  it('rounds fractional results to whole tokens', () => {
    expect(parseTokenAmountInput('1.23456万')).toEqual({ ok: true, value: 12_346 })
  })

  it('rejects empty, malformed, negative and out-of-range input', () => {
    for (const input of ['', '   ', 'abc', '1.2.3K', '-5', '1e6', '1000万亿']) {
      expect(parseTokenAmountInput(input).ok).toBe(false)
    }
    expect(
      parseTokenAmountInput(String(TOKEN_AMOUNT_MAX + 1)).ok
    ).toBe(false)
  })
})
