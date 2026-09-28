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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import { getGroupHealth } from '../lib/status'
import {
  getTimeRangeLabelKey,
  type PerfTimeRangeHours,
} from '../lib/time-range'
import { AnimatedPercent } from './animated-percent'
import { AvailabilityChart } from './availability-chart'
import { CacheMetricsPopover } from './cache-metrics-popover'
import { CachePricingGrid } from './cache-pricing-grid'
import { GroupLabel } from './group-label'
import { HealthBadge } from './health-badge'
import { InputPriceEstimate } from './input-price-estimate'
import { MetricTile } from './metric-tile'
import { ModelTable } from './model-table'

export type ModelBreakdownDialogProps = {
  summary: PerformanceGroupSummary | null
  pricing: PerfGroupPricing | null
  window: AvailabilityWindow
  hours: PerfTimeRangeHours
  onClose: () => void
}

export function ModelBreakdownDialog(props: ModelBreakdownDialogProps) {
  const { t } = useTranslation()
  const summary = props.summary
  const slots = useMemo(
    () => bucketAvailability(summary?.series ?? [], props.window),
    [summary, props.window]
  )

  if (summary === null) return null

  const health = getGroupHealth(summary.request_count, summary.success_rate)
  const hasRequests =
    Number.isFinite(summary.request_count) && summary.request_count > 0
  const description = summary.description
  const showDescription =
    Boolean(description) &&
    description !== summary.group &&
    description !== summary.name
  const windowLabel = t(getTimeRangeLabelKey(props.hours))

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
    >
      <DialogContent className='perf-dialog-pop flex max-h-[85dvh] flex-col gap-0 p-0 sm:max-w-4xl'>
        <DialogHeader className='gap-1.5 border-b p-5 pb-4'>
          <DialogTitle className='flex flex-wrap items-center gap-2 pr-8'>
            <GroupLabel group={summary.group} label={summary.name} />
            {summary.ratio > 0 ? (
              <span
                className='text-muted-foreground/70 font-mono text-[11px] font-normal tabular-nums'
                title={t('Group ratio')}
              >
                {formatGroupRatio(summary.ratio)}
              </span>
            ) : null}
            <HealthBadge health={health} />
          </DialogTitle>
          <DialogDescription>
            {showDescription
              ? description
              : t('Per-model availability within this group')}
          </DialogDescription>
        </DialogHeader>
        <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-5'>
          <div
            className='perf-rise-in flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'
            style={perfDelayStyle(60)}
          >
            <div className='flex min-w-0 flex-col gap-0.5'>
              <span className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
                {t('Availability')}
              </span>
              <span
                className={cn(
                  'font-mono text-3xl font-semibold tracking-tight tabular-nums',
                  hasRequests
                    ? getSuccessRateTextClass(summary.success_rate)
                    : 'text-muted-foreground/50'
                )}
              >
                {hasRequests ? (
                  <AnimatedPercent value={summary.success_rate} />
                ) : (
                  EMPTY_VALUE
                )}
              </span>
              <span className='text-muted-foreground truncate text-xs'>
                {hasRequests
                  ? t('{{success}}/{{total}} requests succeeded', {
                      success: summary.success_count,
                      total: summary.request_count,
                    })
                  : t('No requests in this window')}
                {' \u00b7 '}
                {windowLabel}
              </span>
            </div>
            <div className='grid shrink-0 grid-cols-3 gap-2 sm:w-72'>
              <MetricTile
                label={t('Latency')}
                value={formatLatency(summary.avg_latency_ms)}
              />
              <MetricTile
                label='TTFT'
                value={formatLatency(summary.avg_ttft_ms)}
              />
              <MetricTile
                label='TPS'
                value={formatThroughput(summary.avg_tps)}
              />
            </div>
          </div>

          <div className='flex flex-col gap-2'>
            <CachePricingGrid summary={summary} pricing={props.pricing} />
            <div className='flex justify-end'>
              <CacheMetricsPopover />
            </div>
          </div>

          <AvailabilityChart
            slots={slots}
            baseDelayMs={140}
            ariaLabel={`${summary.group} ${formatUptimePct(summary.success_rate)}`}
          />

          <div
            className='perf-rise-in flex flex-col gap-2'
            style={perfDelayStyle(120)}
          >
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <h3 className='text-foreground text-sm font-semibold'>
                {t('Model breakdown')}
              </h3>
              <span className='text-muted-foreground/80 text-xs'>
                {t('{{count}} models with traffic', {
                  count: summary.models.length,
                })}
              </span>
            </div>
            {summary.models.length === 0 ? (
              <div className='text-muted-foreground rounded-xl border border-dashed p-6 text-center text-sm'>
                {t('No model traffic in this group yet.')}
              </div>
            ) : (
              <ModelTable summary={summary} pricing={props.pricing} />
            )}
            <InputPriceEstimate pricing={props.pricing} />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
