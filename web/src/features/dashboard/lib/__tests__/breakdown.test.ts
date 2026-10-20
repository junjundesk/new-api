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

import type { BreakdownFilters, UserQuotaBreakdownRow } from '../../types'
import {
  BREAKDOWN_EMPTY_FILTER_VALUE,
  breakdownShare,
  buildBreakdownFilterOptions,
  filterBreakdownRows,
  groupBreakdownRowsByUser,
  resolveBreakdownTimeRange,
  summarizeBreakdownRows,
} from '../breakdown'

const NOW = new Date(2026, 2, 15, 13, 45, 30)
const NOW_SECONDS = Math.floor(NOW.getTime() / 1000)
const TODAY_START = Math.floor(new Date(2026, 2, 15, 0, 0, 0).getTime() / 1000)
const YESTERDAY_START = Math.floor(
  new Date(2026, 2, 14, 0, 0, 0).getTime() / 1000
)
const YESTERDAY_END = Math.floor(
  new Date(2026, 2, 14, 23, 59, 59).getTime() / 1000
)
const DAY_SECONDS = 24 * 60 * 60

function row(
  overrides: Partial<UserQuotaBreakdownRow> = {}
): UserQuotaBreakdownRow {
  return {
    user_id: 1,
    username: 'alice',
    use_group: 'vip',
    model_name: 'gpt-4.1',
    count: 1,
    quota: 100,
    token_used: 40,
    ...overrides,
  }
}

function filters(overrides: Partial<BreakdownFilters> = {}): BreakdownFilters {
  return {
    preset: 'today',
    username: '',
    use_group: '',
    model_name: '',
    query: '',
    ...overrides,
  }
}

describe('resolveBreakdownTimeRange', () => {
  test('today spans from local midnight up to the current moment', () => {
    expect(resolveBreakdownTimeRange('today', NOW)).toEqual({
      start_timestamp: TODAY_START,
      end_timestamp: NOW_SECONDS,
    })
  })

  test('yesterday spans the whole previous local day', () => {
    expect(resolveBreakdownTimeRange('yesterday', NOW)).toEqual({
      start_timestamp: YESTERDAY_START,
      end_timestamp: YESTERDAY_END,
    })
  })

  test('last7 and last30 are rolling windows ending now', () => {
    expect(resolveBreakdownTimeRange('last7', NOW)).toEqual({
      start_timestamp: NOW_SECONDS - 7 * DAY_SECONDS,
      end_timestamp: NOW_SECONDS,
    })
    expect(resolveBreakdownTimeRange('last30', NOW)).toEqual({
      start_timestamp: NOW_SECONDS - 30 * DAY_SECONDS,
      end_timestamp: NOW_SECONDS,
    })
  })

  test('all time leaves both bounds open for the server', () => {
    expect(resolveBreakdownTimeRange('all', NOW)).toEqual({
      start_timestamp: 0,
      end_timestamp: 0,
    })
  })
})

describe('summarizeBreakdownRows', () => {
  test('adds quota, tokens and requests and counts distinct users', () => {
    const summary = summarizeBreakdownRows([
      row({
        user_id: 1,
        username: 'alice',
        quota: 100,
        token_used: 40,
        count: 2,
      }),
      row({
        user_id: 1,
        username: 'alice',
        quota: 50,
        token_used: 20,
        count: 1,
      }),
      row({ user_id: 2, username: 'bob', quota: 25, token_used: 10, count: 3 }),
    ])

    expect(summary).toEqual({
      quota: 175,
      tokens: 70,
      requests: 6,
      users: 2,
    })
  })

  test('returns zeroes for an empty result set', () => {
    expect(summarizeBreakdownRows([])).toEqual({
      quota: 0,
      tokens: 0,
      requests: 0,
      users: 0,
    })
  })
})

describe('buildBreakdownFilterOptions', () => {
  test('deduplicates dimensions and orders each list by cost', () => {
    const options = buildBreakdownFilterOptions([
      row({ username: 'alice', quota: 100 }),
      row({ username: 'alice', quota: 100 }),
      row({
        username: 'bob',
        quota: 400,
        use_group: 'default',
        model_name: 'gpt-5',
      }),
    ])

    expect(options.users).toEqual([
      { value: 'bob', label: 'bob', quota: 400 },
      { value: 'alice', label: 'alice', quota: 200 },
    ])
    expect(options.groups.map((group) => group.value)).toEqual([
      'default',
      'vip',
    ])
    expect(options.models.map((model) => model.value)).toEqual([
      'gpt-5',
      'gpt-4.1',
    ])
  })

  test('legacy rows without a group or model stay selectable', () => {
    const options = buildBreakdownFilterOptions([
      row({ use_group: '', model_name: '' }),
    ])

    expect(options.groups).toEqual([
      { value: BREAKDOWN_EMPTY_FILTER_VALUE, label: '—', quota: 100 },
    ])
    expect(options.models).toEqual([
      { value: BREAKDOWN_EMPTY_FILTER_VALUE, label: '—', quota: 100 },
    ])
  })
})

describe('filterBreakdownRows', () => {
  const rows = [
    row({
      user_id: 1,
      username: 'alice',
      use_group: 'vip',
      model_name: 'gpt-4.1',
    }),
    row({
      user_id: 2,
      username: 'bob',
      use_group: 'default',
      model_name: 'gpt-5',
    }),
    row({ user_id: 3, username: 'carol', use_group: '', model_name: 'gpt-5' }),
  ]

  test('returns every row when no dimension is selected', () => {
    expect(filterBreakdownRows(rows, filters())).toHaveLength(3)
  })

  test('narrows by user, group and model together', () => {
    expect(
      filterBreakdownRows(rows, filters({ username: 'bob' }))
    ).toHaveLength(1)
    expect(
      filterBreakdownRows(rows, filters({ use_group: 'default' }))
    ).toHaveLength(1)
    expect(
      filterBreakdownRows(
        rows,
        filters({ username: 'bob', model_name: 'gpt-4.1' })
      )
    ).toHaveLength(0)
  })

  test('selects legacy rows through the empty-value placeholder', () => {
    const legacy = filterBreakdownRows(
      rows,
      filters({ use_group: BREAKDOWN_EMPTY_FILTER_VALUE })
    )

    expect(legacy).toHaveLength(1)
    expect(legacy[0].username).toBe('carol')
  })

  test('matches the free-text query case-insensitively across columns', () => {
    expect(filterBreakdownRows(rows, filters({ query: 'GPT-5' }))).toHaveLength(
      2
    )
    expect(filterBreakdownRows(rows, filters({ query: 'alice' }))).toHaveLength(
      1
    )
    expect(
      filterBreakdownRows(rows, filters({ query: 'nobody' }))
    ).toHaveLength(0)
  })
})

describe('groupBreakdownRowsByUser', () => {
  test('sums one user across groups and models and orders details by cost', () => {
    const grouped = groupBreakdownRowsByUser([
      row({
        user_id: 1,
        username: 'alice',
        use_group: 'vip',
        model_name: 'gpt-a',
        count: 2,
        quota: 100,
        token_used: 10,
      }),
      row({
        user_id: 1,
        username: 'alice',
        use_group: 'default',
        model_name: 'gpt-b',
        count: 1,
        quota: 50,
        token_used: 5,
      }),
      row({
        user_id: 2,
        username: 'bob',
        use_group: 'vip',
        model_name: 'gpt-a',
        count: 3,
        quota: 75,
        token_used: 7,
      }),
    ])

    expect(grouped).toHaveLength(2)
    expect(grouped[0]).toMatchObject({
      key: 'id:1',
      user_id: 1,
      username: 'alice',
      quota: 150,
      count: 3,
      token_used: 15,
    })
    expect(grouped[0].details.map((detail) => detail.use_group)).toEqual([
      'vip',
      'default',
    ])
    expect(grouped[1]).toMatchObject({
      key: 'id:2',
      username: 'bob',
      quota: 75,
      count: 3,
      token_used: 7,
    })
  })

  test('groups legacy rows without a user id by username', () => {
    const grouped = groupBreakdownRowsByUser([
      row({
        user_id: 0,
        username: 'legacy',
        use_group: 'vip',
        model_name: 'a',
        quota: 10,
      }),
      row({
        user_id: 0,
        username: 'legacy',
        use_group: 'default',
        model_name: 'b',
        quota: 20,
      }),
      row({
        user_id: 0,
        username: 'other',
        use_group: 'vip',
        model_name: 'a',
        quota: 30,
      }),
    ])

    expect(grouped).toHaveLength(2)
    expect(grouped.find((entry) => entry.username === 'legacy')?.quota).toBe(30)
    expect(
      grouped.find((entry) => entry.username === 'legacy')?.details
    ).toHaveLength(2)
  })

  test('returns an empty array when there are no rows', () => {
    expect(groupBreakdownRowsByUser([])).toEqual([])
  })
})

describe('breakdownShare', () => {
  test('returns a percentage of the total', () => {
    expect(breakdownShare(25, 100)).toBe(25)
    expect(breakdownShare(1, 3)).toBeCloseTo(33.33, 2)
  })

  test('never produces NaN when the total is empty or invalid', () => {
    expect(breakdownShare(10, 0)).toBe(0)
    expect(breakdownShare(10, Number.NaN)).toBe(0)
    expect(breakdownShare(Number.NaN, 100)).toBe(0)
  })
})
