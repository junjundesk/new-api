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
import { ArrowRight } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  formatLatency,
  formatThroughput,
  formatUptimePct,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'
import type { PerformanceGroupSummary } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

import { EMPTY_VALUE, formatGroupRatio, perfDelayStyle } from '../lib/format'
import { bucketAvailability, type AvailabilityWindow } from '../lib/series'
import { getGroupHealth, getGroupHealthBorderClass } from '../lib/status'
import { AnimatedPercent } from './animated-percent'
import { AvailabilityChart } from './availability-chart'
import { CachePricingGrid } from './cache-pricing-grid'
import { GroupLabel } from './group-label'
import { HealthBadge } from './health-badge'
import { MetricTile } from './metric-tile'

export type GroupCardProps = {
  summary: PerformanceGroupSummary
  pricing: PerfGroupPricing | null
  window: AvailabilityWindow
  entranceDelayMs: number
  onOpenModels: () => void
}

export function GroupCard(props: GroupCardProps) {
  const { t } = useTranslation()
  const summary = props.summary
  const health = getGroupHealth(summary.request_count, summary.success_rate)
  const hasRequests =
    Number.isFinite(summary.request_count) && summary.request_count > 0
  const slots = useMemo(
    () => bucketAvailability(summary.series, props.window),
    [summary.series, props.window]
  )
  const description = summary.description
  const showDescription =
    Boolean(description) &&
    description !== summary.group &&
    description !== summary.name

  return (
    <article
      role='button'
      tabIndex={0}
      aria-haspopup='dialog'
      style={perfDelayStyle(props.entranceDelayMs)}
      onClick={props.onOpenModels}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          props.onOpenModels()
        }
      }}
      className={cn(
        'group bg-card perf-rise-in flex cursor-pointer flex-col gap-3.5 rounded-xl border p-4 text-left',
        'transition-[border-color,box-shadow,transform] duration-200 hover:shadow-sm active:scale-[0.99]',
        'focus-visible:ring-ring focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none',
        getGroupHealthBorderClass(health),
        'hover:border-ring/40'
      )}
    >
      <header className='flex items-start justify-between gap-2'>
        <div className='flex min-w-0 flex-col gap-1'>
          <div className='flex items-center gap-2'>
            <GroupLabel group={summary.group} label={summary.name} />
            {summary.ratio > 0 ? (
              <span
                className='text-muted-foreground/70 font-mono text-[11px] tabular-nums'
                title={t('Group ratio')}
              >
                {formatGroupRatio(summary.ratio)}
              </span>
            ) : null}
          </div>
          {showDescription ? (
            <p
              className='text-muted-foreground truncate text-xs'
              title={description}
            >
              {description}
            </p>
          ) : null}
        </div>
        <HealthBadge health={health} />
      </header>

      <div className='grid grid-cols-3 gap-2'>
        <MetricTile
          label={t('Latency')}
          value={formatLatency(summary.avg_latency_ms)}
        />
        <MetricTile label='TTFT' value={formatLatency(summary.avg_ttft_ms)} />
        <MetricTile label='TPS' value={formatThroughput(summary.avg_tps)} />
      </div>

      <div className='bg-muted/40 flex items-center justify-between gap-3 rounded-lg px-3 py-2.5'>
        <div className='flex min-w-0 flex-col gap-0.5'>
          <span className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
            {t('Availability')}
          </span>
          <span className='text-muted-foreground truncate text-xs'>
            {hasRequests
              ? t('{{success}}/{{total}} requests succeeded', {
                  success: summary.success_count,
                  total: summary.request_count,
                })
              : t('No requests in this window')}
          </span>
        </div>
        <span
          className={cn(
            'shrink-0 font-mono text-2xl font-semibold tabular-nums',
            hasRequests
              ? getSuccessRateTextClass(summary.success_rate)
              : 'text-muted-foreground/50'
          )}
        >
          <span
            key={`${String(summary.success_rate)}-${String(summary.request_count)}`}
            className='perf-value-roll'
          >
            {hasRequests ? (
              <AnimatedPercent value={summary.success_rate} />
            ) : (
              EMPTY_VALUE
            )}
          </span>
        </span>
      </div>

      <AvailabilityChart
        slots={slots}
        ariaLabel={`${summary.group} ${formatUptimePct(summary.success_rate)}`}
      />

      <CachePricingGrid summary={summary} pricing={props.pricing} />

      <footer className='text-muted-foreground flex items-center justify-between text-xs'>
        <span>
          {t('{{count}} models with traffic', { count: summary.models.length })}
        </span>
        <span className='text-foreground/70 group-hover:text-foreground inline-flex items-center gap-0.5 font-medium transition-colors duration-200'>
          {t('View models')}
          <ArrowRight
            className='size-3.5 transition-transform duration-200 group-hover:translate-x-0.5'
            aria-hidden
          />
        </span>
      </footer>
    </article>
  )
}
