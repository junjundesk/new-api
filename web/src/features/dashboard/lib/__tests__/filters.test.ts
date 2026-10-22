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

import { buildQueryParams, cleanFilters } from '../filters'

const TIME_RANGE = { start_timestamp: 1000, end_timestamp: 2000 }

describe('buildQueryParams', () => {
  test('serializes excluded users into one comma-separated parameter', () => {
    const params = buildQueryParams(TIME_RANGE, {
      time_granularity: 'day',
      exclude_users: [
        { id: 10, username: 'admin' },
        { id: 3, username: 'ops' },
      ],
    })

    expect(params.exclude_user_ids).toBe('10,3')
  })

  test('omits the exclusion parameter when nobody is excluded', () => {
    const params = buildQueryParams(TIME_RANGE, {
      time_granularity: 'day',
      exclude_users: [],
    })

    expect('exclude_user_ids' in params).toBe(false)
  })

  test('drops invalid ids instead of sending them to the server', () => {
    const params = buildQueryParams(TIME_RANGE, {
      time_granularity: 'day',
      exclude_users: [
        { id: 0, username: 'zero' },
        { id: -1, username: 'negative' },
        { id: 7, username: 'valid' },
      ],
    })

    expect(params.exclude_user_ids).toBe('7')
  })
})

describe('cleanFilters', () => {
  test('removes an empty exclusion list from the applied filters', () => {
    expect(
      cleanFilters({
        username: 'alice',
        exclude_users: [],
      })
    ).toEqual({ username: 'alice' })
  })

  test('keeps a non-empty exclusion list', () => {
    const excludeUsers = [{ id: 10, username: 'admin' }]

    expect(cleanFilters({ exclude_users: excludeUsers })).toEqual({
      exclude_users: excludeUsers,
    })
  })
})
