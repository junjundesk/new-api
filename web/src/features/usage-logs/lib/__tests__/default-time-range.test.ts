/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getDefaultTimeRange } from '../utils'

describe('usage log default time range', () => {
  test('covers the whole current local day', () => {
    const { start, end } = getDefaultTimeRange()

    assert.equal(start.getFullYear(), end.getFullYear())
    assert.equal(start.getMonth(), end.getMonth())
    assert.equal(start.getDate(), end.getDate())
    assert.equal(start.getHours(), 0)
    assert.equal(start.getMinutes(), 0)
    assert.equal(start.getSeconds(), 0)
    assert.equal(start.getMilliseconds(), 0)
    assert.equal(end.getHours(), 23)
    assert.equal(end.getMinutes(), 59)
    assert.equal(end.getSeconds(), 59)
    assert.equal(end.getMilliseconds(), 999)
  })
})
