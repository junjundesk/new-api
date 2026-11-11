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

import { currentTierIndex, formatRate, pageCount } from '../format'

describe('formatRate', () => {
  it.each([
    [0, '0%'],
    [100, '1%'],
    [150, '1.5%'],
    [125, '1.25%'],
    [1000, '10%'],
  ])('renders %i bps as %s', (bps, expected) => {
    expect(formatRate(bps)).toBe(expected)
  })
})

describe('pageCount', () => {
  it.each([
    [0, 10, 1],
    [1, 10, 1],
    [10, 10, 1],
    [11, 10, 2],
    [25, 10, 3],
  ])('%i rows at %i per page is %i pages', (total, size, expected) => {
    expect(pageCount(total, size)).toBe(expected)
  })
})

describe('currentTierIndex', () => {
  const tiers = [
    { min_invitees: 1, rate_bps: 100 },
    { min_invitees: 3, rate_bps: 300 },
    { min_invitees: 5, rate_bps: 500 },
  ]

  it.each([
    [0, -1],
    [1, 0],
    [2, 0],
    [3, 1],
    [5, 2],
    [40, 2],
  ])('%i valid invitees is tier index %i', (valid, expected) => {
    expect(currentTierIndex(tiers, valid)).toBe(expected)
  })

  it('is -1 when there are no tiers', () => {
    expect(currentTierIndex([], 9)).toBe(-1)
  })
})
