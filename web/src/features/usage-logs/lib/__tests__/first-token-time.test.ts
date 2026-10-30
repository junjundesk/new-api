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
import { describe, expect, test } from 'vitest'

import { formatFirstTokenTime } from '../format'

describe('first token latency formatting', () => {
  test.each([
    [0.034, '34ms'],
    [0.5, '500ms'],
    [0.999, '999ms'],
    [0.9996, '1.0s'],
    [1, '1.0s'],
    [1.24, '1.2s'],
    [65.4, '1m 5s'],
  ])('formats %s seconds as %s', (seconds, expected) => {
    expect(formatFirstTokenTime(seconds)).toBe(expected)
  })
})
