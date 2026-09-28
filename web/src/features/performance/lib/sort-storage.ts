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
import { GROUP_SORT_OPTIONS, type GroupSortKey } from './sort'

export const SORT_STORAGE_KEY = 'performance-groups-sort'
export const CUSTOM_ORDER_STORAGE_KEY = 'performance-groups-custom-order'
export const DEFAULT_GROUP_SORT: GroupSortKey = 'requests'

/** Persisted sort key, falling back to request volume for unknown values. */
export function readStoredGroupSort(): GroupSortKey {
  if (typeof window === 'undefined') return DEFAULT_GROUP_SORT
  try {
    const raw = window.localStorage.getItem(SORT_STORAGE_KEY)
    const known = GROUP_SORT_OPTIONS.find((option) => option.value === raw)
    return known ? known.value : DEFAULT_GROUP_SORT
  } catch {
    return DEFAULT_GROUP_SORT
  }
}

export function writeStoredGroupSort(value: GroupSortKey): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(SORT_STORAGE_KEY, value)
  } catch {
    return
  }
}

/** Persisted custom order: only non-empty unique strings survive. */
export function readStoredCustomOrder(): string[] {
  if (typeof window === 'undefined') return []
  try {
    const parsed: unknown = JSON.parse(
      window.localStorage.getItem(CUSTOM_ORDER_STORAGE_KEY) ?? '[]'
    )
    if (!Array.isArray(parsed)) return []
    const seen = new Set<string>()
    const order: string[] = []
    for (const value of parsed) {
      if (typeof value !== 'string' || value === '' || seen.has(value)) continue
      seen.add(value)
      order.push(value)
    }
    return order
  } catch {
    return []
  }
}

export function writeStoredCustomOrder(order: string[]): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(CUSTOM_ORDER_STORAGE_KEY, JSON.stringify(order))
  } catch {
    return
  }
}
