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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { getPerfMetricsGroups } from '@/features/performance-metrics/api'
import {
  computeGroupPricing,
  type PerfGroupPricing,
} from '@/features/performance-metrics/lib/pricing'
import type {
  PerfGroupsData,
  PerformanceGroupSummary,
} from '@/features/performance-metrics/types'

import { DEFAULT_BUCKET_SECONDS, type AvailabilityWindow } from '../lib/series'

export const PERF_GROUPS_STALE_TIME_MS = 55_000
export const PERF_GROUPS_REFETCH_INTERVAL_MS = 60_000

export type PerfGroupsSnapshot = {
  groups: PerformanceGroupSummary[]
  window: AvailabilityWindow
}

type PerfGroupsPayload = PerfGroupsData['data'] | undefined

/**
 * The backend may omit fields while it is being extended, so every list and
 * every numeric field is normalized before it reaches the rendering layer.
 */
function toSnapshot(payload: PerfGroupsPayload): PerfGroupsSnapshot {
  const listed = Array.isArray(payload?.groups) ? payload.groups : []
  const groups = listed.map((group) => ({
    ...group,
    series: Array.isArray(group.series) ? group.series : [],
    models: Array.isArray(group.models) ? group.models : [],
  }))
  return {
    groups,
    window: {
      bucketSeconds: payload?.bucket_seconds ?? DEFAULT_BUCKET_SECONDS,
      startTs: payload?.start_ts ?? 0,
      endTs: payload?.end_ts ?? 0,
    },
  }
}

export function usePerfGroups(hours: number, lang: string) {
  return useQuery({
    queryKey: ['perf-metrics-groups', hours, lang],
    queryFn: async (): Promise<PerfGroupsSnapshot> => {
      const response = await getPerfMetricsGroups(hours, lang)
      const envelope: PerfGroupsData | undefined = response
      return toSnapshot(envelope?.data)
    },
    staleTime: PERF_GROUPS_STALE_TIME_MS,
    refetchInterval: PERF_GROUPS_REFETCH_INTERVAL_MS,
    placeholderData: keepPreviousData,
  })
}

/**
 * Derived pricing for every group, keyed by group. A group without any valid
 * sample maps to null, which renders as a dash instead of NaN.
 */
export function useGroupPricing(
  groups: PerformanceGroupSummary[],
  referenceCacheRate: number | null
): Map<string, PerfGroupPricing | null> {
  return useMemo(() => {
    const pricingByGroup = new Map<string, PerfGroupPricing | null>()
    for (const group of groups) {
      pricingByGroup.set(
        group.group,
        computeGroupPricing({
          samples: group.models,
          groupRatio: group.ratio,
          referenceCacheRate,
        })
      )
    }
    return pricingByGroup
  }, [groups, referenceCacheRate])
}
