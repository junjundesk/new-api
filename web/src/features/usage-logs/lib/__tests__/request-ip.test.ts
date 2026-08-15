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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getRequestIpForDisplay } from '../format'

describe('request IP display', () => {
  test('uses the complete admin request IP when an admin views a usage log', () => {
    assert.equal(
      getRequestIpForDisplay(
        '192.0.2.10',
        { request_ip: '203.0.113.42' },
        true
      ),
      '203.0.113.42'
    )
  })

  test('does not expose admin-only request IP data to non-admin viewers', () => {
    assert.equal(
      getRequestIpForDisplay('', { request_ip: '203.0.113.42' }, false),
      ''
    )
  })

  test('falls back to the legacy log IP when admin request IP is unavailable', () => {
    assert.equal(
      getRequestIpForDisplay('192.0.2.10', undefined, true),
      '192.0.2.10'
    )
  })
})

