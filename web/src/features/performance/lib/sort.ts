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
import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'
import type { PerformanceGroupSummary } from '@/features/performance-metrics/types'

export type GroupSortKey =
  | 'custom'
  | 'requests'
  | 'observed-desc'
  | 'observed-asc'
  | 'effective-desc'
  | 'effective-asc'
  | 'name'
  | 'coding-cache'
  | 'success'
  | 'latency'
  | 'throughput'

/** Select options in source order; the direction is encoded in the value. */
export const GROUP_SORT_OPTIONS: {
  value: GroupSortKey
  labelKey: string
}[] = [
  { value: 'custom', labelKey: 'Custom group order' },
  { value: 'requests', labelKey: 'Request volume' },
  { value: 'observed-desc', labelKey: 'Observed multiplier: high to low' },
  { value: 'observed-asc', labelKey: 'Observed multiplier: low to high' },
  { value: 'effective-desc', labelKey: 'Effective multiplier: high to low' },
  { value: 'effective-asc', labelKey: 'Effective multiplier: low to high' },
  { value: 'name', labelKey: 'Group name' },
  { value: 'coding-cache', labelKey: 'Coding cache: high to low' },
  { value: 'success', labelKey: 'Success rate: high to low' },
  { value: 'latency', labelKey: 'Latency: low to high' },
  { value: 'throughput', labelKey: 'Throughput: high to low' },
]

/**
 * Saved order first, then every remaining group appended in name order, so a
 * group that appears for the first time never loses its place silently.
 */
export function mergeCustomOrder(
  saved: string[],
  allGroups: string[]
): string[] {
  const merged: string[] = []
  const seen = new Set<string>()
  for (const group of saved) {
    if (seen.has(group)) continue
    seen.add(group)
    merged.push(group)
  }
  const rest = [...new Set(allGroups)]
    .filter((group) => !seen.has(group))
    .sort((a, b) => a.localeCompare(b))
  return [...merged, ...rest]
}

/**
 * Swaps a group with its visible neighbour. The offset is measured in the
 * filtered list while the swap happens in the merged list, so groups hidden by
 * the status filter keep their positions.
 */
export function moveCustomGroup(input: {
  saved: string[]
  allGroups: string[]
  visibleGroups: string[]
  group: string
  delta: number
}): string[] {
  const combined = mergeCustomOrder(input.saved, input.allGroups)
  const visibleIndex = input.visibleGroups.indexOf(input.group)
  if (visibleIndex < 0) return combined
  const target = input.visibleGroups[visibleIndex + input.delta]
  if (target === undefined) return combined
  const from = combined.indexOf(input.group)
  const to = combined.indexOf(target)
  if (from < 0 || to < 0) return combined
  const next = [...combined]
  next[from] = target
  next[to] = input.group
  return next
}

export function groupDisplayName(group: PerformanceGroupSummary): string {
  return group.name ?? group.group
}

function compareGroupName(
  a: PerformanceGroupSummary,
  b: PerformanceGroupSummary
): number {
  return groupDisplayName(a).localeCompare(groupDisplayName(b), undefined, {
    sensitivity: 'base',
  })
}

function toSortableNumber(value: number | undefined): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

/** Nulls always sort last, regardless of the direction. */
function compareNullable(
  a: number | null,
  b: number | null,
  direction: 'asc' | 'desc'
): number {
  if (a === null && b === null) return 0
  if (a === null) return 1
  if (b === null) return -1
  return direction === 'asc' ? a - b : b - a
}

/** Zero and negative values count as "no value" and sort last. */
function toPositiveOrNull(value: number): number | null {
  return value > 0 ? toSortableNumber(value) : null
}

function compareForSort(
  a: PerformanceGroupSummary,
  b: PerformanceGroupSummary,
  sortKey: GroupSortKey,
  orderIndex: Map<string, number>,
  pricingByGroup: Map<string, PerfGroupPricing | null>
): number {
  const observed = (group: PerformanceGroupSummary) =>
    pricingByGroup.get(group.group)?.observedInputMultiplier ?? null
  const effective = (group: PerformanceGroupSummary) =>
    pricingByGroup.get(group.group)?.effectiveRatio ?? null
  const codingCache = (group: PerformanceGroupSummary) =>
    group.coding_cache_observed === true
      ? toSortableNumber(group.coding_cache_hit_rate)
      : null

  switch (sortKey) {
    case 'requests':
      return compareNullable(
        toSortableNumber(a.request_count),
        toSortableNumber(b.request_count),
        'desc'
      )
    case 'observed-desc':
      return compareNullable(observed(a), observed(b), 'desc')
    case 'observed-asc':
      return compareNullable(observed(a), observed(b), 'asc')
    case 'effective-desc':
      return compareNullable(effective(a), effective(b), 'desc')
    case 'effective-asc':
      return compareNullable(effective(a), effective(b), 'asc')
    case 'name':
      return compareGroupName(a, b)
    case 'coding-cache':
      return compareNullable(codingCache(a), codingCache(b), 'desc')
    case 'success':
      return compareNullable(
        toSortableNumber(a.success_rate),
        toSortableNumber(b.success_rate),
        'desc'
      )
    case 'latency':
      return compareNullable(
        toPositiveOrNull(a.avg_latency_ms),
        toPositiveOrNull(b.avg_latency_ms),
        'asc'
      )
    case 'throughput':
      return compareNullable(
        toPositiveOrNull(a.avg_tps),
        toPositiveOrNull(b.avg_tps),
        'desc'
      )
    case 'custom':
      return compareNullable(
        orderIndex.get(a.group) ?? null,
        orderIndex.get(b.group) ?? null,
        'asc'
      )
    default:
      return 0
  }
}

export function sortGroups(input: {
  groups: PerformanceGroupSummary[]
  sortKey: GroupSortKey
  customOrder: string[]
  pricingByGroup: Map<string, PerfGroupPricing | null>
}): PerformanceGroupSummary[] {
  const mergedOrder = mergeCustomOrder(
    input.customOrder,
    input.groups.map((group) => group.group)
  )
  const orderIndex = new Map(mergedOrder.map((group, index) => [group, index]))
  const sorted = [...input.groups]
  sorted.sort((a, b) => {
    const result = compareForSort(
      a,
      b,
      input.sortKey,
      orderIndex,
      input.pricingByGroup
    )
    if (result !== 0) return result
    return compareGroupName(a, b)
  })
  return sorted
}
