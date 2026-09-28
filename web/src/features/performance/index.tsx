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
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  CircleAlert,
  CircleX,
  Clock3,
  Database,
  Gauge,
  Info,
  RefreshCw,
  Timer,
  Zap,
} from 'lucide-react'
import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getPerfMetricsGroups } from '@/features/performance-metrics/api'
import {
  formatCompactCount,
  formatLatency,
  formatRatio,
  formatThroughput,
  formatUptimePct,
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { PerformanceGroupSummary } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

type GroupSummary = PerformanceGroupSummary & {
  modelCount: number
}

type PerformanceSnapshot = {
  groups: GroupSummary[]
  cacheObserved: boolean
  cacheHitRate: number
}

// Status taxonomy mirrors the reference performance page:
// running >= 90% / fluctuating 70-90% / error < 70% / no data.
type GroupStatus = 'running' | 'fluctuating' | 'error' | 'noData'

const STATUS_ORDER: GroupStatus[] = ['running', 'fluctuating', 'error', 'noData']

function getGroupStatus(group: PerformanceGroupSummary): GroupStatus {
  if (!group.series?.length) return 'noData'
  if (!Number.isFinite(group.success_rate)) return 'noData'
  if (group.success_rate >= 90) return 'running'
  if (group.success_rate >= 70) return 'fluctuating'
  return 'error'
}

type TimeRange = 1 | 24 | 168

type SortKey = 'requests' | 'success' | 'latency' | 'ttft' | 'tps'

const SORT_OPTIONS: { value: SortKey; labelKey: string }[] = [
  { value: 'requests', labelKey: 'Requests' },
  { value: 'success', labelKey: 'Success rate' },
  { value: 'latency', labelKey: 'Latency' },
  { value: 'ttft', labelKey: 'TTFT' },
  { value: 'tps', labelKey: 'Throughput' },
]

const CACHE_RATE_THRESHOLDS = [80, 85, 90, 95] as const

async function fetchSnapshot(hours: TimeRange): Promise<PerformanceSnapshot> {
  const response = await getPerfMetricsGroups(hours)
  const data = response.data
  const groups = data?.groups ?? []
  return {
    groups: groups.map((group) => ({
      ...group,
      series: group.series ?? [],
      models: group.models ?? [],
      modelCount: group.models?.length ?? 0,
    })),
    cacheObserved: data?.cache_observed ?? false,
    cacheHitRate: data?.cache_hit_rate ?? 0,
  }
}

export function Performance() {
  const { t } = useTranslation()
  const [timeRange, setTimeRange] = useState<TimeRange>(24)
  const [statusFilter, setStatusFilter] = useState<GroupStatus | null>(null)
  const [sortKey, setSortKey] = useState<SortKey>('requests')
  const [cacheThreshold, setCacheThreshold] = useState<number>(90)
  const snapshotQuery = useQuery({
    queryKey: ['performance-page', timeRange],
    queryFn: () => fetchSnapshot(timeRange),
    staleTime: 60 * 1000,
    refetchInterval: 60 * 1000,
    retry: false,
  })

  const refresh = useCallback(() => {
    void snapshotQuery.refetch()
  }, [snapshotQuery])

  const snapshot = snapshotQuery.data
  const groups = useMemo(() => snapshot?.groups ?? [], [snapshot])
  const statusCounts = useMemo(() => {
    return groups.reduce(
      (counts, group) => {
        counts[getGroupStatus(group)] += 1
        return counts
      },
      {
        running: 0,
        fluctuating: 0,
        error: 0,
        noData: 0,
      } as Record<GroupStatus, number>
    )
  }, [groups])

  const visibleGroups = useMemo(() => {
    const filtered = statusFilter
      ? groups.filter((group) => getGroupStatus(group) === statusFilter)
      : groups
    const sorted = [...filtered]
    sorted.sort((a, b) => {
      switch (sortKey) {
        case 'success':
          return (b.success_rate ?? 0) - (a.success_rate ?? 0)
        case 'latency':
          return (a.avg_latency_ms ?? 0) - (b.avg_latency_ms ?? 0)
        case 'ttft':
          return (a.avg_ttft_ms ?? 0) - (b.avg_ttft_ms ?? 0)
        case 'tps':
          return (b.avg_tps ?? 0) - (a.avg_tps ?? 0)
        default:
          return (b.request_count ?? 0) - (a.request_count ?? 0)
      }
    })
    return sorted
  }, [groups, statusFilter, sortKey])

  const updatedLabel = snapshotQuery.dataUpdatedAt
    ? new Date(snapshotQuery.dataUpdatedAt).toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    : '—'

  let performanceBody: ReactNode
  if (snapshotQuery.isLoading) {
    performanceBody = (
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
        {['one', 'two', 'three', 'four', 'five', 'six'].map((key) => (
          <PerformanceCardSkeleton key={key} />
        ))}
      </div>
    )
  } else if (snapshotQuery.isError && !snapshotQuery.data) {
    performanceBody = (
      <Card>
        <CardContent className='text-muted-foreground flex flex-col items-center gap-2 py-12 text-center text-sm'>
          <CircleX className='text-destructive size-8' />
          <p>{t('Request failed')}</p>
          <Button variant='outline' size='sm' onClick={refresh}>
            {t('Retry')}
          </Button>
        </CardContent>
      </Card>
    )
  } else if (visibleGroups.length === 0) {
    performanceBody = (
      <Card>
        <CardContent className='text-muted-foreground flex flex-col items-center gap-2 py-12 text-center text-sm'>
          <Activity className='size-8' />
          <p>{t('No performance data available')}</p>
        </CardContent>
      </Card>
    )
  } else {
    performanceBody = (
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
        {visibleGroups.map((group) => (
          <PerformanceGroupCard
            key={group.group}
            group={group}
            cacheThreshold={cacheThreshold}
          />
        ))}
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Performance')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <TimeRangeSwitch value={timeRange} onChange={setTimeRange} />
        <Button
          variant='outline'
          size='sm'
          onClick={refresh}
          disabled={snapshotQuery.isFetching}
        >
          <RefreshCw
            className={cn(snapshotQuery.isFetching && 'animate-spin')}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-1 text-xs'>
            <span>
              {t('Real-time availability and latency for your usable groups.')}
            </span>
            <span aria-hidden='true'>·</span>
            <span>
              {t('Updated')} {updatedLabel}
            </span>
            <span aria-hidden='true'>·</span>
            <span>{t('Auto refresh')} · 1m</span>
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <SortSelect value={sortKey} onChange={setSortKey} />
            <CacheHelpPopover />
            <CacheThresholdButton
              value={cacheThreshold}
              onChange={setCacheThreshold}
            />
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            {STATUS_ORDER.map((status) => (
              <StatusFilterButton
                key={status}
                status={status}
                count={statusCounts[status]}
                active={statusFilter === status}
                onToggle={() =>
                  setStatusFilter((current) =>
                    current === status ? null : status
                  )
                }
              />
            ))}
            <span className='text-muted-foreground ms-auto text-xs'>
              {groups.length} {t('Groups')}
            </span>
          </div>

          {performanceBody}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function TimeRangeSwitch(props: {
  value: TimeRange
  onChange: (value: TimeRange) => void
}) {
  const { t } = useTranslation()
  return (
    <div
      role='group'
      aria-label={t('Time range')}
      className='border-border/60 inline-flex items-center rounded-md border p-0.5'
    >
      {TIME_RANGES.map((range) => (
        <button
          key={range.value}
          type='button'
          aria-pressed={props.value === range.value}
          onClick={() => props.onChange(range.value)}
          className={cn(
            'rounded px-2.5 py-1 text-xs font-medium transition-colors',
            props.value === range.value
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground'
          )}
        >
          {range.label}
        </button>
      ))}
    </div>
  )
}

const TIME_RANGES: { value: TimeRange; label: string }[] = [
  { value: 1, label: '1h' },
  { value: 24, label: '24h' },
  { value: 168, label: '7d' },
]

function SortSelect(props: {
  value: SortKey
  onChange: (value: SortKey) => void
}) {
  const { t } = useTranslation()
  const current = SORT_OPTIONS.find((option) => option.value === props.value)
  return (
    <Select
      items={SORT_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.labelKey),
      }))}
      value={props.value}
      onValueChange={(value) => {
        if (typeof value === 'string') props.onChange(value as SortKey)
      }}
    >
      <SelectTrigger size='sm' aria-label={t('Sort groups')} className='h-8 w-40'>
        <SelectValue>
          <span className='text-muted-foreground'>{t('Sort groups')}:</span>
          {t(current?.labelKey ?? 'Requests')}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {SORT_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.labelKey)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function CacheHelpPopover() {
  const { t } = useTranslation()
  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button variant='ghost' size='sm'>
            <Info className='size-3.5' />
            {t('Cache stats explanation')}
          </Button>
        }
      />
      <PopoverContent align='start' className='w-96 text-sm'>
        <p className='text-sm font-semibold'>{t('Cache stats explanation')}</p>
        <div className='text-muted-foreground space-y-2 text-xs'>
          <p>
            {t(
              'Cache hit rate is the share of prompt input tokens served from the prompt cache. Higher means cheaper and faster.'
            )}
          </p>
          <p>
            {t(
              'Coding cache is the share of cacheable tokens written to the cache (5m/1h creation). Coding workloads reuse context heavily, so a healthy creation share keeps future hits high.'
            )}
          </p>
          <p>
            {t(
              'The reference cache rate marks the healthy level for this gateway. Groups below it are highlighted.'
            )}
          </p>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function CacheThresholdButton(props: {
  value: number
  onChange: (value: number) => void
}) {
  const { t } = useTranslation()
  const next = () => {
    const idx = CACHE_RATE_THRESHOLDS.indexOf(
      props.value as (typeof CACHE_RATE_THRESHOLDS)[number]
    )
    props.onChange(
      CACHE_RATE_THRESHOLDS[(idx + 1) % CACHE_RATE_THRESHOLDS.length]
    )
  }
  return (
    <Button variant='outline' size='sm' onClick={next}>
      <Zap className='size-3.5 text-amber-500' />
      {t('Reference cache rate')}: {props.value}%
    </Button>
  )
}

function StatusFilterButton(props: {
  status: GroupStatus
  count: number
  active: boolean
  onToggle: () => void
}) {
  const { t } = useTranslation()
  const label = {
    running: t('Running'),
    fluctuating: t('Fluctuating'),
    error: t('Error status'),
    noData: t('No data'),
  }[props.status]
  const Icon = {
    running: CheckCircle2,
    fluctuating: CircleAlert,
    error: CircleX,
    noData: Clock3,
  }[props.status]
  const activeClass = {
    running:
      'border-emerald-500/40 bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
    fluctuating:
      'border-amber-500/40 bg-amber-500/15 text-amber-700 dark:text-amber-300',
    error: 'border-destructive/40 bg-destructive/10 text-destructive',
    noData: 'border-muted-foreground/30 bg-muted text-foreground',
  }[props.status]
  return (
    <button
      type='button'
      aria-pressed={props.active}
      onClick={props.onToggle}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors',
        props.active
          ? activeClass
          : 'border-border/60 bg-background text-muted-foreground hover:text-foreground'
      )}
    >
      <Icon className='size-3.5' aria-hidden='true' />
      <span className='font-mono'>{props.count}</span>
      <span>{label}</span>
    </button>
  )
}
function PerformanceGroupCard(props: {
  group: GroupSummary
  cacheThreshold: number
}) {
  const { t } = useTranslation()
  const group = props.group
  const status = getGroupStatus(group)
  const [modelsExpanded, setModelsExpanded] = useState(false)
  const statusLabel = {
    running: t('Running'),
    fluctuating: t('Fluctuating'),
    error: t('Error status'),
    noData: t('No data'),
  }[status]
  const latestSeries = group.series.slice(-24)
  const cacheBelowReference =
    group.cache_observed && group.cache_hit_rate < props.cacheThreshold

  return (
    <Card className='group transition-shadow hover:shadow-md'>
      <CardHeader className='gap-2 border-b pb-3'>
        <div className='flex items-start gap-2'>
          <div className='min-w-0 flex-1'>
            <CardTitle
              className='truncate text-sm font-semibold'
              title={group.group}
            >
              {group.group}
              <span className='text-muted-foreground ms-1.5 font-mono text-xs font-medium'>
                ×{formatRatio(group.ratio)}
              </span>
            </CardTitle>
            {group.description && (
              <CardDescription className='mt-1 line-clamp-2 text-xs'>
                {group.description}
              </CardDescription>
            )}
          </div>
          <Badge variant='outline' className={cn(statusColor(status))}>
            <span
              className={cn(
                'size-1.5 rounded-full',
                getSuccessRateDotClass(group.success_rate)
              )}
            />
            {statusLabel}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-3 pt-3'>
        <div className='grid grid-cols-3 gap-2'>
          <Metric
            label={t('Latency')}
            value={formatLatency(group.avg_latency_ms)}
            icon={Timer}
          />
          <Metric
            label='TTFT'
            value={formatLatency(group.avg_ttft_ms)}
            icon={Clock3}
          />
          <Metric
            label={t('Throughput short')}
            value={formatThroughput(group.avg_tps)}
            icon={Gauge}
          />
        </div>
        <div
          className='flex items-end gap-1'
          aria-label={`${group.group} ${formatUptimePct(group.success_rate)}`}
        >
          {latestSeries.length > 0 ? (
            latestSeries.map((point) => (
              <span
                key={`${point.ts}-${point.success_rate}-${point.avg_latency_ms}`}
                className={cn(
                  'h-7 min-w-1 flex-1 rounded-sm opacity-80',
                  getSuccessRateDotClass(point.success_rate)
                )}
                style={{
                  height: `${Math.max(18, Math.min(100, point.success_rate || 0))}%`,
                }}
                title={formatUptimePct(point.success_rate)}
              />
            ))
          ) : (
            <span className='bg-muted h-7 w-full rounded-sm' />
          )}
        </div>
        <div className='text-muted-foreground flex items-center justify-between gap-2 text-xs'>
          <span>
            {formatCompactCount(group.success_count)}/
            {formatCompactCount(group.request_count)} {t('Requests success')}
          </span>
          <span
            className={cn(
              'font-mono font-semibold tabular-nums',
              getSuccessRateTextClass(group.success_rate)
            )}
          >
            {t('Success')} {formatUptimePct(group.success_rate)}
          </span>
        </div>
        <div className='text-muted-foreground flex items-center justify-between gap-2 border-t pt-2 text-xs'>
          <span className='flex items-center gap-1.5'>
            <Database className='size-3.5' />
            {t('Cache hit rate')}
          </span>
          <span
            className={cn(
              'font-mono font-semibold tabular-nums',
              cacheRateClass(group, cacheBelowReference)
            )}
          >
            {formatUptimePct(group.cache_hit_rate)}
          </span>
        </div>
        <div className='text-muted-foreground flex items-center justify-between gap-2 text-xs'>
          <span className='flex items-center gap-1.5'>
            <Zap className='size-3.5' />
            {t('Coding cache')}
          </span>
          <span
            className={cn(
              'font-mono font-semibold tabular-nums',
              !group.coding_cache_observed
                ? 'text-muted-foreground'
                : getSuccessRateTextClass(group.coding_cache_hit_rate ?? 0)
            )}
          >
            {formatUptimePct(group.coding_cache_hit_rate ?? 0)}
          </span>
        </div>
        <div className='flex items-center justify-between gap-2 border-t pt-2'>
          <span className='text-muted-foreground text-xs'>
            {group.modelCount} {t('Models with traffic')}
          </span>
          {group.modelCount > 0 && (
            <Button
              variant='ghost'
              size='xs'
              aria-expanded={modelsExpanded}
              onClick={() => setModelsExpanded((value) => !value)}
            >
              {modelsExpanded ? (
                <ChevronUp className='size-3.5' />
              ) : (
                <ChevronDown className='size-3.5' />
              )}
              {t('View models')}
            </Button>
          )}
        </div>
        {modelsExpanded && <ModelTable group={group} />}
      </CardContent>
    </Card>
  )
}

function ModelTable(props: { group: GroupSummary }) {
  const { t } = useTranslation()
  const models = props.group.models
  return (
    <div className='bg-muted/40 overflow-hidden rounded-lg'>
      <table className='w-full text-xs'>
        <thead>
          <tr className='text-muted-foreground border-b text-left'>
            <th className='px-3 py-1.5 font-medium'>{t('Model')}</th>
            <th className='px-2 py-1.5 text-right font-medium'>
              {t('Requests')}
            </th>
            <th className='px-2 py-1.5 text-right font-medium'>
              {t('Success')}
            </th>
            <th className='px-3 py-1.5 text-right font-medium'>
              {t('Cache hit rate')}
            </th>
          </tr>
        </thead>
        <tbody>
          {models.map((model) => (
            <tr
              key={model.model_name}
              className='border-muted/60 hover:bg-muted/60 border-b last:border-0'
            >
              <td
                className='max-w-40 truncate px-3 py-1.5'
                title={model.model_name}
              >
                {model.model_name}
              </td>
              <td className='px-2 py-1.5 text-right font-mono tabular-nums'>
                {formatCompactCount(model.request_count)}
              </td>
              <td
                className={cn(
                  'font-mono px-2 py-1.5 text-right tabular-nums',
                  getSuccessRateTextClass(model.success_rate)
                )}
              >
                {formatUptimePct(model.success_rate)}
              </td>
              <td
                className={cn(
                  'font-mono px-3 py-1.5 text-right tabular-nums',
                  model.cache_observed
                    ? getSuccessRateTextClass(model.cache_hit_rate)
                    : 'text-muted-foreground'
                )}
              >
                {formatUptimePct(model.cache_hit_rate)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function cacheRateClass(group: GroupSummary, belowReference: boolean) {
  if (!group.cache_observed) return 'text-muted-foreground'
  if (belowReference) return 'text-warning'
  return getSuccessRateTextClass(group.cache_hit_rate)
}

function statusColor(status: GroupStatus) {
  if (status === 'running') {
    return 'border-emerald-500/30 text-emerald-700 dark:text-emerald-300'
  }
  if (status === 'fluctuating') {
    return 'border-amber-500/30 text-amber-700 dark:text-amber-300'
  }
  if (status === 'error') {
    return 'border-destructive/40 text-destructive'
  }
  return 'border-muted-foreground/20 text-muted-foreground'
}

function Metric(props: {
  label: string
  value: string
  icon: React.ComponentType<{ className?: string }>
}) {
  const Icon = props.icon
  return (
    <div className='bg-muted/40 rounded-lg p-2'>
      <div className='text-muted-foreground flex items-center gap-1 text-[10px] font-medium'>
        <Icon className='size-3' />
        <span className='truncate'>{props.label}</span>
      </div>
      <div className='mt-1 truncate font-mono text-xs font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function PerformanceCardSkeleton() {
  return (
    <Card>
      <CardHeader className='border-b pb-3'>
        <Skeleton className='h-4 w-2/3' />
        <Skeleton className='h-3 w-1/3' />
      </CardHeader>
      <CardContent className='space-y-3 pt-3'>
        <div className='grid grid-cols-3 gap-2'>
          <Skeleton className='h-14' />
          <Skeleton className='h-14' />
          <Skeleton className='h-14' />
        </div>
        <Skeleton className='h-7 w-full' />
        <Skeleton className='h-4 w-full' />
        <Skeleton className='h-4 w-full' />
      </CardContent>
    </Card>
  )
}
