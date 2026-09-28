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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { resolvePerfMetricsLang } from '../lib/lang'
import {
  DEFAULT_REFERENCE_CACHE_RATE_INPUT,
  parseReferenceCacheRate,
} from '../lib/reference-cache-rate'
import { DEFAULT_BUCKET_SECONDS, type AvailabilityWindow } from '../lib/series'
import {
  mergeCustomOrder,
  moveCustomGroup,
  sortGroups,
  type GroupSortKey,
} from '../lib/sort'
import {
  readStoredCustomOrder,
  readStoredGroupSort,
  writeStoredCustomOrder,
  writeStoredGroupSort,
} from '../lib/sort-storage'
import {
  countGroupHealth,
  getGroupHealth,
  type GroupHealth,
} from '../lib/status'
import {
  DEFAULT_PERF_TIME_RANGE,
  type PerfTimeRangeHours,
} from '../lib/time-range'
import { useGroupPricing, usePerfGroups } from './use-perf-groups'

const FALLBACK_WINDOW: AvailabilityWindow = {
  bucketSeconds: DEFAULT_BUCKET_SECONDS,
  startTs: 0,
  endTs: 0,
}

/**
 * All state, persistence and derived data behind the performance page: the
 * time range, the persisted sort order, the custom order, the status filter,
 * the reference cache rate input and the selected group for the dialog.
 */
export function usePerformancePage() {
  const { i18n } = useTranslation()
  const [hours, setHours] = useState<PerfTimeRangeHours>(
    DEFAULT_PERF_TIME_RANGE
  )
  const [sortKey, setSortKey] = useState<GroupSortKey>(readStoredGroupSort)
  const [customOrder, setCustomOrder] = useState<string[]>(
    readStoredCustomOrder
  )
  const [statusFilter, setStatusFilter] = useState<GroupHealth | null>(null)
  const [referenceCacheRateInput, setReferenceCacheRateInput] = useState(
    DEFAULT_REFERENCE_CACHE_RATE_INPUT
  )
  const [selectedGroup, setSelectedGroup] = useState<string | null>(null)

  const lang = resolvePerfMetricsLang(i18n.resolvedLanguage ?? i18n.language)
  const query = usePerfGroups(hours, lang)
  const groups = useMemo(() => query.data?.groups ?? [], [query.data])
  const availabilityWindow = useMemo(
    () => query.data?.window ?? FALLBACK_WINDOW,
    [query.data]
  )
  const referenceCacheRate = useMemo(
    () => parseReferenceCacheRate(referenceCacheRateInput),
    [referenceCacheRateInput]
  )
  const pricingByGroup = useGroupPricing(groups, referenceCacheRate)
  const sortedGroups = useMemo(
    () => sortGroups({ groups, sortKey, customOrder, pricingByGroup }),
    [groups, sortKey, customOrder, pricingByGroup]
  )
  const statusCounts = useMemo(
    () => countGroupHealth(sortedGroups),
    [sortedGroups]
  )
  const activeFilter =
    statusFilter !== null && statusCounts[statusFilter] > 0
      ? statusFilter
      : null
  const visibleGroups = useMemo(() => {
    if (activeFilter === null) return sortedGroups
    return sortedGroups.filter(
      (group) =>
        getGroupHealth(group.request_count, group.success_rate) === activeFilter
    )
  }, [activeFilter, sortedGroups])
  const allGroupKeys = useMemo(
    () => groups.map((group) => group.group),
    [groups]
  )

  useEffect(() => {
    writeStoredGroupSort(sortKey)
  }, [sortKey])

  useEffect(() => {
    writeStoredCustomOrder(customOrder)
  }, [customOrder])

  useEffect(() => {
    if (!query.isSuccess || query.isPlaceholderData) return
    const merged = mergeCustomOrder(customOrder, allGroupKeys)
    const changed =
      merged.length !== customOrder.length ||
      merged.some((group, index) => group !== customOrder[index])
    if (changed) setCustomOrder(merged)
  }, [allGroupKeys, customOrder, query.isPlaceholderData, query.isSuccess])

  const refresh = useCallback(() => {
    void query.refetch()
  }, [query])

  const moveGroup = useCallback(
    (group: string, delta: number) => {
      setCustomOrder((current) =>
        moveCustomGroup({
          saved: current,
          allGroups: allGroupKeys,
          visibleGroups: visibleGroups.map((item) => item.group),
          group,
          delta,
        })
      )
    },
    [allGroupKeys, visibleGroups]
  )

  const toggleStatusFilter = useCallback((health: GroupHealth) => {
    setStatusFilter((current) => (current === health ? null : health))
  }, [])

  const selectedSummary =
    selectedGroup === null
      ? null
      : (groups.find((group) => group.group === selectedGroup) ?? null)
  const selectedPricing =
    selectedGroup === null ? null : (pricingByGroup.get(selectedGroup) ?? null)

  return {
    hours,
    setHours,
    sortKey,
    setSortKey,
    referenceCacheRateInput,
    setReferenceCacheRateInput,
    query,
    groups,
    sortedGroups,
    visibleGroups,
    statusCounts,
    activeFilter,
    toggleStatusFilter,
    pricingByGroup,
    availabilityWindow,
    allGroupKeys,
    customOrder,
    moveGroup,
    refresh,
    selectedSummary,
    selectedPricing,
    selectedGroup,
    setSelectedGroup,
  }
}
