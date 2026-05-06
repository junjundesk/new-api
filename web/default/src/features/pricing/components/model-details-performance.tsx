import { useMemo } from 'react'
import {
  Activity,
  AlertTriangle,
  Gauge,
  HeartPulse,
  Timer,
  TrendingUp,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { GroupBadge } from '@/components/group-badge'
import {
  aggregateUptime,
  buildGroupPerformance,
  buildLatencyTimeSeries,
  buildUptimeSeries,
  formatLatency,
  formatThroughput,
  formatUptimePct,
  type UptimeDayPoint,
} from '../lib/mock-stats'
import type { PricingModel } from '../types'
import {
  LatencyTrendChart,
  ThroughputBarChart,
  UptimeBarChart,
} from './model-details-charts'
import { UptimeSparkline } from './model-details-uptime-sparkline'

const COMPACT_NUMBER = new Intl.NumberFormat(undefined, {
  notation: 'compact',
  maximumFractionDigits: 1,
})

function StatCard(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: React.ReactNode
  hint?: string
  intent?: 'default' | 'warning' | 'success'
}) {
  const Icon = props.icon
  const intent = props.intent ?? 'default'
  return (
    <div className='bg-background flex flex-col gap-1 rounded-lg border p-3'>
      <span className='text-muted-foreground inline-flex items-center gap-1.5 text-[10px] font-medium tracking-wider uppercase'>
        <Icon className='size-3' />
        {props.label}
      </span>
      <span
        className={cn(
          'text-foreground font-mono text-lg font-semibold tabular-nums',
          intent === 'warning' && 'text-amber-600 dark:text-amber-400',
          intent === 'success' && 'text-emerald-600 dark:text-emerald-400'
        )}
      >
        {props.value}
      </span>
      {props.hint && (
        <span className='text-muted-foreground/70 text-[11px]'>
          {props.hint}
        </span>
      )}
    </div>
  )
}

export function ModelDetailsPerformance(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const performances = useMemo(
    () => buildGroupPerformance(props.model),
    [props.model]
  )
  const latencySeries = useMemo(
    () => buildLatencyTimeSeries(props.model),
    [props.model]
  )
  const uptimeSeries = useMemo(
    () => buildUptimeSeries(props.model),
    [props.model]
  )
  const aggregated = useMemo(
    () => aggregateUptime(uptimeSeries),
    [uptimeSeries]
  )
  const uptimeByGroup = useMemo<Record<string, UptimeDayPoint[]>>(() => {
    const map: Record<string, UptimeDayPoint[]> = {}
    for (const perf of performances) {
      map[perf.group] = buildUptimeSeries(props.model, perf.group)
    }
    return map
  }, [performances, props.model])

  if (performances.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
        {t('Performance data is not yet available for this model.')}
      </div>
    )
  }

  const bestTtft = Math.min(...performances.map((p) => p.ttft_p50_ms))
  const bestThroughput = Math.max(...performances.map((p) => p.throughput_tps))
  const totalRequests = performances.reduce(
    (s, p) => s + p.request_volume_24h,
    0
  )
  const intent =
    aggregated.uptime_pct >= 99.9
      ? 'success'
      : aggregated.uptime_pct >= 99
        ? 'default'
        : 'warning'

  const headerCellClass =
    'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase'

  return (
    <div className='flex flex-col gap-4'>
      <div className='grid grid-cols-2 gap-2 lg:grid-cols-4'>
        <StatCard
          icon={Timer}
          label={t('Best TTFT')}
          value={formatLatency(bestTtft)}
          hint={t('Lowest median first-token latency')}
        />
        <StatCard
          icon={Gauge}
          label={t('Peak throughput')}
          value={formatThroughput(bestThroughput)}
          hint={t('Across all groups')}
        />
        <StatCard
          icon={HeartPulse}
          label={t('Uptime (30d)')}
          value={formatUptimePct(aggregated.uptime_pct)}
          hint={
            aggregated.incidents > 0
              ? t('{{count}} incidents in the last 30 days', {
                  count: aggregated.incidents,
                })
              : t('No incidents in the last 30 days')
          }
          intent={intent}
        />
        <StatCard
          icon={TrendingUp}
          label={t('Requests (24h)')}
          value={COMPACT_NUMBER.format(totalRequests)}
          hint={t('Aggregated across enabled groups')}
        />
      </div>

      <section>
        <SectionHeader
          icon={Activity}
          title={t('Per-group performance')}
          description={t(
            'TTFT percentiles, throughput, and 30-day uptime by group'
          )}
        />
        <div className='overflow-x-auto rounded-lg border'>
          <Table className='text-sm'>
            <TableHeader>
              <TableRow className='hover:bg-transparent'>
                <TableHead className={headerCellClass}>{t('Group')}</TableHead>
                <TableHead className={`${headerCellClass} text-right`}>
                  {t('TTFT P50')}
                </TableHead>
                <TableHead className={`${headerCellClass} text-right`}>
                  {t('TTFT P95')}
                </TableHead>
                <TableHead className={`${headerCellClass} text-right`}>
                  {t('TTFT P99')}
                </TableHead>
                <TableHead className={`${headerCellClass} text-right`}>
                  {t('Throughput')}
                </TableHead>
                <TableHead
                  className={`${headerCellClass} min-w-[160px] text-left`}
                >
                  {t('Uptime (30d)')}
                </TableHead>
                <TableHead className={`${headerCellClass} text-right`}>
                  {t('Requests / 24h')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {performances.map((perf) => {
                const isBestTtft = perf.ttft_p50_ms === bestTtft
                const isBestTput = perf.throughput_tps === bestThroughput
                return (
                  <TableRow key={perf.group}>
                    <TableCell className='py-2.5'>
                      <GroupBadge group={perf.group} size='sm' />
                    </TableCell>
                    <TableCell
                      className={cn(
                        'py-2.5 text-right font-mono',
                        isBestTtft && 'text-emerald-600 dark:text-emerald-400'
                      )}
                    >
                      {formatLatency(perf.ttft_p50_ms)}
                    </TableCell>
                    <TableCell className='text-muted-foreground py-2.5 text-right font-mono'>
                      {formatLatency(perf.ttft_p95_ms)}
                    </TableCell>
                    <TableCell className='text-muted-foreground py-2.5 text-right font-mono'>
                      {formatLatency(perf.ttft_p99_ms)}
                    </TableCell>
                    <TableCell
                      className={cn(
                        'py-2.5 text-right font-mono',
                        isBestTput &&
                          perf.throughput_tps > 0 &&
                          'text-emerald-600 dark:text-emerald-400'
                      )}
                    >
                      {formatThroughput(perf.throughput_tps)}
                    </TableCell>
                    <TableCell className='py-2.5'>
                      <UptimeSparkline
                        size='sm'
                        series={uptimeByGroup[perf.group] ?? []}
                      />
                    </TableCell>
                    <TableCell className='text-muted-foreground py-2.5 text-right font-mono'>
                      {COMPACT_NUMBER.format(perf.request_volume_24h)}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      </section>

      <section>
        <SectionHeader
          icon={Timer}
          title={t('Latency trend (last 24h)')}
          description={t(
            'Median time-to-first-token (TTFT) sampled hourly per group'
          )}
        />
        <LatencyTrendChart series={latencySeries} />
      </section>

      {bestThroughput > 0 && (
        <section>
          <SectionHeader
            icon={Gauge}
            title={t('Throughput by group')}
            description={t('Average tokens per second sustained per group')}
          />
          <ThroughputBarChart rows={performances} />
        </section>
      )}

      <section>
        <SectionHeader
          icon={HeartPulse}
          title={t('Uptime (last 30 days)')}
          description={
            aggregated.incidents > 0
              ? t(
                  'Daily uptime; {{incidents}} incidents totalling {{minutes}} minutes',
                  {
                    incidents: aggregated.incidents,
                    minutes: aggregated.outage_minutes,
                  }
                )
              : t('Daily uptime over the last 30 days')
          }
          accent={
            aggregated.incidents > 0 ? (
              <span className='inline-flex items-center gap-1 text-amber-600 dark:text-amber-400'>
                <AlertTriangle className='size-3.5' />
                {t('{{count}} incidents', {
                  count: aggregated.incidents,
                })}
              </span>
            ) : null
          }
        />
        <UptimeBarChart series={uptimeSeries} />
      </section>

      <p className='text-muted-foreground/60 text-[11px] leading-relaxed'>
        {t(
          'Performance metrics shown here are simulated for preview purposes and will be replaced with live observability data once the backend integration is complete.'
        )}
      </p>
    </div>
  )
}

function SectionHeader(props: {
  icon: React.ComponentType<{ className?: string }>
  title: string
  description?: string
  accent?: React.ReactNode
}) {
  const Icon = props.icon
  return (
    <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
      <div className='flex min-w-0 items-center gap-2'>
        <Icon className='text-muted-foreground/70 size-3.5 shrink-0' />
        <div className='min-w-0'>
          <div className='text-foreground text-sm font-semibold'>
            {props.title}
          </div>
          {props.description && (
            <p className='text-muted-foreground/80 text-xs'>
              {props.description}
            </p>
          )}
        </div>
      </div>
      {props.accent && (
        <div className='shrink-0 text-xs font-medium'>{props.accent}</div>
      )}
    </div>
  )
}
