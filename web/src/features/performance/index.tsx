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
  HeartPulse,
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

type TimeWindow = 24 | 168

type GroupSummary = PerformanceGroupSummary & {
  modelCount: number
}

type PerformanceSnapshot = {
  groups: GroupSummary[]
  cacheObserved: boolean
  cacheHitRate: number
}

type GroupStatus = 'running' | 'degraded' | 'noData'

const TIME_WINDOWS: Array<{ value: TimeWindow; label: string }> = [
  { value: 24, label: '24h' },
  { value: 168, label: '7d' },
]

function getGroupStatus(group: PerformanceGroupSummary): GroupStatus {
  if (group.series.length === 0) return 'noData'
  if (!Number.isFinite(group.success_rate)) return 'noData'
  if (group.success_rate >= 90) return 'running'
  return 'degraded'
}

async function fetchSnapshot(hours: TimeWindow): Promise<PerformanceSnapshot> {
  const response = await getPerfMetricsGroups(hours)
  const data = response.data
  const groups = data?.groups ?? []
  return {
    groups: groups.map((group) => ({
      ...group,
      modelCount: group.models.length,
    })),
    cacheObserved: data?.cache_observed ?? false,
    cacheHitRate: data?.cache_hit_rate ?? 0,
  }
}

export function Performance() {
  const { t } = useTranslation()
  const [hours, setHours] = useState<TimeWindow>(24)
  const snapshotQuery = useQuery({
    queryKey: ['performance-page', hours],
    queryFn: () => fetchSnapshot(hours),
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
      { running: 0, degraded: 0, noData: 0 } as Record<GroupStatus, number>
    )
  }, [groups])

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
  } else if (snapshotQuery.isError) {
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
  } else if (groups.length === 0) {
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
        {groups.map((group) => (
          <PerformanceGroupCard key={group.group} group={group} />
        ))}
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Performance')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-1 rounded-lg border p-0.5'>
          {TIME_WINDOWS.map((window) => (
            <Button
              key={window.value}
              size='xs'
              variant={hours === window.value ? 'secondary' : 'ghost'}
              aria-pressed={hours === window.value}
              onClick={() => setHours(window.value)}
            >
              {window.label}
            </Button>
          ))}
        </div>
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
              {hours === 24
                ? t('Performance metrics for the last 24 hours')
                : t('Performance metrics for the last 7 days')}
            </span>
            <span aria-hidden='true'>·</span>
            <span>
              {t('Updated')} {updatedLabel}
            </span>
            <span aria-hidden='true'>·</span>
            <span>{t('Auto refresh')} · 1m</span>
          </div>

          <CacheDiscountCard
            cacheObserved={snapshot?.cacheObserved ?? false}
            cacheHitRate={snapshot?.cacheHitRate ?? 0}
          />

          <div className='flex flex-wrap items-center gap-2'>
            <StatusBadge
              status='running'
              count={statusCounts.running}
              label={t('Running')}
            />
            <StatusBadge
              status='degraded'
              count={statusCounts.degraded}
              label={t('Fluctuating')}
            />
            <StatusBadge
              status='noData'
              count={statusCounts.noData}
              label={t('No data')}
            />
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

function CacheDiscountCard(props: {
  cacheObserved: boolean
  cacheHitRate: number
}) {
  const { t } = useTranslation()
  return (
    <Card className='from-primary/10 via-card to-card border-primary/20 bg-gradient-to-r'>
      <CardHeader className='flex flex-row items-start gap-3 space-y-0 border-b pb-4'>
        <div className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-lg'>
          <Zap className='size-4' />
        </div>
        <div className='min-w-0'>
          <CardTitle className='text-sm'>{t('Cache discount')}</CardTitle>
          <CardDescription className='mt-1 text-xs'>
            {t('Cache discount description')}
          </CardDescription>
        </div>
        <Badge variant='outline' className='ms-auto shrink-0'>
          {t('Cache')}
        </Badge>
      </CardHeader>
      <CardContent className='flex flex-wrap items-center gap-x-4 gap-y-2 pt-4'>
        <div className='flex items-center gap-2'>
          <HeartPulse className='size-4 text-emerald-500' />
          <span className='text-sm font-medium'>
            {t('Prompt cache hit rate')}
          </span>
        </div>
        {props.cacheObserved ? (
          <span
            className={cn(
              'font-mono text-lg font-semibold tabular-nums',
              getSuccessRateTextClass(props.cacheHitRate)
            )}
          >
            {formatUptimePct(props.cacheHitRate)}
          </span>
        ) : (
          <span className='text-muted-foreground text-sm'>
            {t('No cache data yet')}
          </span>
        )}
      </CardContent>
    </Card>
  )
}

function StatusBadge(props: {
  status: GroupStatus
  count: number
  label: string
}) {
  const icon = {
    running: CheckCircle2,
    degraded: CircleAlert,
    noData: Clock3,
  }[props.status]
  const Icon = icon
  const className = {
    running:
      'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
    degraded:
      'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300',
    noData: 'border-muted-foreground/20 bg-muted text-muted-foreground',
  }[props.status]

  return (
    <Badge variant='outline' className={cn('gap-1.5', className)}>
      <Icon className='size-3.5' />
      <span>{props.count}</span>
      <span className='max-w-44 truncate'>{props.label}</span>
    </Badge>
  )
}

function PerformanceGroupCard(props: { group: GroupSummary }) {
  const { t } = useTranslation()
  const group = props.group
  const status = getGroupStatus(group)
  const [modelsExpanded, setModelsExpanded] = useState(false)
  const statusLabel = {
    running: t('Running'),
    degraded: t('Fluctuating'),
    noData: t('No data'),
  }[status]
  const latestSeries = group.series.slice(-12)

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
                {formatRatio(group.ratio)}
              </span>
            </CardTitle>
            <CardDescription className='mt-1 flex min-w-0 items-center gap-1.5 text-xs'>
              <span className='truncate' title={group.description}>
                {group.description}
              </span>
              <span aria-hidden='true'>·</span>
              <span>
                {group.modelCount} {t('Models')}
              </span>
            </CardDescription>
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
            {formatCompactCount(group.request_count)} {t('Requests')}
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
              group.cache_observed
                ? getSuccessRateTextClass(group.cache_hit_rate)
                : 'text-muted-foreground'
            )}
          >
            {formatUptimePct(group.cache_hit_rate)}
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

function statusColor(status: GroupStatus) {
  if (status === 'running') {
    return 'border-emerald-500/30 text-emerald-700 dark:text-emerald-300'
  }
  if (status === 'degraded') {
    return 'border-amber-500/30 text-amber-700 dark:text-amber-300'
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
