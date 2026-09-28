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
import { getSuccessRateLevel } from '@/features/performance-metrics/lib/format'

export type GroupHealth = 'operational' | 'degraded' | 'critical' | 'nodata'

export type GroupHealthMeta = {
  labelKey: string
  dotClass: string
  textClass: string
}

/** Fixed chip order for the status filter row. */
export const GROUP_HEALTH_ORDER: GroupHealth[] = [
  'operational',
  'degraded',
  'critical',
  'nodata',
]

export const GROUP_HEALTH: Record<GroupHealth, GroupHealthMeta> = {
  operational: {
    labelKey: 'Operational',
    dotClass: 'bg-emerald-500',
    textClass: 'text-emerald-600 dark:text-emerald-400',
  },
  degraded: {
    labelKey: 'Degraded',
    dotClass: 'bg-amber-500',
    textClass: 'text-amber-600 dark:text-amber-400',
  },
  critical: {
    labelKey: 'Unavailable',
    dotClass: 'bg-red-500',
    textClass: 'text-red-600 dark:text-red-400',
  },
  nodata: {
    labelKey: 'No data',
    dotClass: 'bg-muted-foreground/40',
    textClass: 'text-muted-foreground',
  },
}

/**
 * Health of a group: no traffic at all is nodata, otherwise the success rate
 * grade decides (>= 90 operational, >= 70 degraded, below 70 critical).
 * A non-finite success rate is graded critical instead of crashing the badge.
 */
export function getGroupHealth(
  requestCount: number,
  successRate: number
): GroupHealth {
  if (!Number.isFinite(requestCount) || requestCount <= 0) return 'nodata'
  const level = getSuccessRateLevel(successRate)
  if (level === 'excellent' || level === 'good') return 'operational'
  if (level === 'warning') return 'degraded'
  return 'critical'
}

export function getGroupHealthBorderClass(health: GroupHealth): string {
  if (health === 'critical') return 'border-red-500/40'
  if (health === 'degraded') return 'border-amber-500/40'
  return ''
}

export function countGroupHealth(
  groups: { request_count: number; success_rate: number }[]
): Record<GroupHealth, number> {
  const counts: Record<GroupHealth, number> = {
    operational: 0,
    degraded: 0,
    critical: 0,
    nodata: 0,
  }
  for (const group of groups) {
    counts[getGroupHealth(group.request_count, group.success_rate)] += 1
  }
  return counts
}
