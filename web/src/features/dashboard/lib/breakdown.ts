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
import type {
  BreakdownFilters,
  BreakdownTimePreset,
  UserQuotaBreakdownGroupedRow,
  UserQuotaBreakdownRow,
} from '@/features/dashboard/types'
import { dateToUnixTimestamp, getEndOfDay, getStartOfDay } from '@/lib/time'

const DAY_SECONDS = 24 * 60 * 60

// Legacy quota_data rows can carry an empty group or model name. The empty
// string already means "no filter", so those rows get a dedicated filter value
// that no real group or model can take.
export const BREAKDOWN_EMPTY_FILTER_VALUE = '\u0000'

export interface BreakdownFilterOption {
  value: string
  label: string
  quota: number
}

export interface BreakdownFilterOptions {
  users: BreakdownFilterOption[]
  groups: BreakdownFilterOption[]
  models: BreakdownFilterOption[]
}

export interface BreakdownSummary {
  quota: number
  tokens: number
  requests: number
  users: number
}

/**
 * Resolve a preset into the timestamp range the API expects.
 * "today" and "yesterday" are local calendar days; "last7"/"last30" are rolling
 * windows matching the chart presets; "all" sends zero bounds, which the server
 * reads as "unbounded".
 */
export function resolveBreakdownTimeRange(
  preset: BreakdownTimePreset,
  now: Date = new Date()
): { start_timestamp: number; end_timestamp: number } {
  const nowSeconds = dateToUnixTimestamp(now)

  if (preset === 'today') {
    return {
      start_timestamp: dateToUnixTimestamp(getStartOfDay(now)),
      end_timestamp: nowSeconds,
    }
  }

  if (preset === 'yesterday') {
    const yesterday = new Date(now)
    yesterday.setDate(yesterday.getDate() - 1)
    return {
      start_timestamp: dateToUnixTimestamp(getStartOfDay(yesterday)),
      end_timestamp: dateToUnixTimestamp(getEndOfDay(yesterday)),
    }
  }

  if (preset === 'last7') {
    return {
      start_timestamp: nowSeconds - 7 * DAY_SECONDS,
      end_timestamp: nowSeconds,
    }
  }

  if (preset === 'last30') {
    return {
      start_timestamp: nowSeconds - 30 * DAY_SECONDS,
      end_timestamp: nowSeconds,
    }
  }

  return { start_timestamp: 0, end_timestamp: 0 }
}

function breakdownFilterValue(raw: string): string {
  return raw === '' ? BREAKDOWN_EMPTY_FILTER_VALUE : raw
}

export function summarizeBreakdownRows(
  rows: UserQuotaBreakdownRow[]
): BreakdownSummary {
  let quota = 0
  let tokens = 0
  let requests = 0
  const activeUsers = new Set<string>()

  for (const row of rows) {
    quota += row.quota || 0
    tokens += row.token_used || 0
    requests += row.count || 0
    activeUsers.add(
      row.user_id > 0 ? `#${row.user_id}` : `name:${row.username}`
    )
  }

  return { quota, tokens, requests, users: activeUsers.size }
}

/**
 * Build the user/group/model filter choices from the rows already loaded, so
 * narrowing never needs another request. Choices are ordered by cost.
 */
export function buildBreakdownFilterOptions(
  rows: UserQuotaBreakdownRow[]
): BreakdownFilterOptions {
  const users = new Map<string, BreakdownFilterOption>()
  const groups = new Map<string, BreakdownFilterOption>()
  const models = new Map<string, BreakdownFilterOption>()

  const collect = (
    target: Map<string, BreakdownFilterOption>,
    label: string,
    quota: number
  ) => {
    const value = breakdownFilterValue(label)
    const existing = target.get(value)
    if (existing) {
      existing.quota += quota
      return
    }
    target.set(value, {
      value,
      label: label === '' ? '—' : label,
      quota,
    })
  }

  for (const row of rows) {
    collect(users, row.username, row.quota || 0)
    collect(groups, row.use_group, row.quota || 0)
    collect(models, row.model_name, row.quota || 0)
  }

  const sorted = (target: Map<string, BreakdownFilterOption>) =>
    [...target.values()].sort((a, b) => b.quota - a.quota)

  return {
    users: sorted(users),
    groups: sorted(groups),
    models: sorted(models),
  }
}

export function filterBreakdownRows(
  rows: UserQuotaBreakdownRow[],
  filters: BreakdownFilters
): UserQuotaBreakdownRow[] {
  const query = filters.query.trim().toLowerCase()

  return rows.filter((row) => {
    if (
      filters.username &&
      breakdownFilterValue(row.username) !== filters.username
    ) {
      return false
    }
    if (
      filters.use_group &&
      breakdownFilterValue(row.use_group) !== filters.use_group
    ) {
      return false
    }
    if (
      filters.model_name &&
      breakdownFilterValue(row.model_name) !== filters.model_name
    ) {
      return false
    }
    if (!query) return true

    return `${row.username} ${row.use_group} ${row.model_name} ${row.user_id}`
      .toLowerCase()
      .includes(query)
  })
}

export function isGroupedBreakdownRow(
  row: UserQuotaBreakdownRow
): row is UserQuotaBreakdownGroupedRow {
  return 'details' in row
}

/**
 * Collapse the per-user x group x model rows into one row per user for the
 * admin view. Totals cover exactly the rows passed in, so callers must filter
 * first; the attached details keep the original per-group/per-model numbers.
 */
export function groupBreakdownRowsByUser(
  rows: UserQuotaBreakdownRow[]
): UserQuotaBreakdownGroupedRow[] {
  const grouped = new Map<string, UserQuotaBreakdownGroupedRow>()
  for (const row of rows) {
    const key = row.user_id > 0 ? `id:${row.user_id}` : `name:${row.username}`
    let entry = grouped.get(key)
    if (!entry) {
      entry = {
        key,
        user_id: row.user_id,
        username: row.username,
        use_group: '',
        model_name: '',
        count: 0,
        quota: 0,
        token_used: 0,
        details: [],
      }
      grouped.set(key, entry)
    }
    if (!entry.username && row.username) {
      entry.username = row.username
    }
    entry.count += row.count || 0
    entry.quota += row.quota || 0
    entry.token_used += row.token_used || 0
    entry.details.push(row)
  }

  const result = [...grouped.values()]
  for (const entry of result) {
    entry.details.sort((a, b) => {
      const quotaDiff = (b.quota || 0) - (a.quota || 0)
      if (quotaDiff !== 0) return quotaDiff
      if (a.use_group !== b.use_group) {
        return a.use_group < b.use_group ? -1 : 1
      }
      if (a.model_name !== b.model_name) {
        return a.model_name < b.model_name ? -1 : 1
      }
      return 0
    })
  }
  result.sort((a, b) => {
    const quotaDiff = (b.quota || 0) - (a.quota || 0)
    if (quotaDiff !== 0) return quotaDiff
    if (a.username !== b.username) {
      return a.username < b.username ? -1 : 1
    }
    return a.user_id - b.user_id
  })
  return result
}

/**
 * Share of the filtered total, as a percentage in the 0-100 range so it can be
 * passed straight to `formatPercent`.
 */
export function breakdownShare(rowQuota: number, totalQuota: number): number {
  if (!Number.isFinite(rowQuota) || !Number.isFinite(totalQuota)) return 0
  if (totalQuota <= 0) return 0
  return (rowQuota / totalQuota) * 100
}
