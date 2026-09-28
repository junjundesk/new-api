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
  formatLatency,
  formatThroughput,
  formatUptimePct,
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'
import type { PerformanceGroupSummary } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

import { EMPTY_VALUE, formatCount, formatMultiplier } from '../lib/format'

const NUMERIC_CELL = 'px-2 py-1.5 text-right font-mono tabular-nums'
const HEADER_CELL = 'px-2 py-1.5 text-right font-medium'
const MOBILE_HIDDEN_SM = 'max-sm:hidden'

export type ModelTableProps = {
  summary: PerformanceGroupSummary
  pricing: PerfGroupPricing | null
}

export function ModelTable(props: ModelTableProps) {
  const { t } = useTranslation()
  const pricingByModel = useMemo(() => {
    const map = new Map(
      (props.pricing?.models ?? []).map((model) => [model.modelName, model])
    )
    return map
  }, [props.pricing])

  return (
    <div className='overflow-x-auto'>
      <table className='w-full text-sm'>
        <thead>
          <tr className='text-muted-foreground border-b text-xs'>
            <th className='px-2 py-1.5 text-left font-medium'>{t('Model')}</th>
            <th className={cn(HEADER_CELL, MOBILE_HIDDEN_SM)}>
              {t('Requests')}
            </th>
            <th className={HEADER_CELL}>{t('Success rate')}</th>
            <th className={HEADER_CELL}>{t('Cache hit')}</th>
            <th className={cn(HEADER_CELL, MOBILE_HIDDEN_SM)}>
              {t('Coding cache')}
            </th>
            <th className={cn(HEADER_CELL, MOBILE_HIDDEN_SM)}>
              {t('Reference-adjusted multiplier')}
            </th>
            <th className={cn(HEADER_CELL, MOBILE_HIDDEN_SM)}>
              {t('Average TTFT')}
            </th>
            <th className={cn(HEADER_CELL, 'max-md:hidden')}>
              {t('Average latency')}
            </th>
            <th className={cn(HEADER_CELL, MOBILE_HIDDEN_SM)}>TPS</th>
          </tr>
        </thead>
        <tbody>
          {props.summary.models.map((model) => {
            const modelPricing = pricingByModel.get(model.model_name)
            const coverage = (
              (modelPricing?.pricedInputShare ?? 0) * 100
            ).toFixed(1)
            return (
              <tr
                key={model.model_name}
                className='border-muted/60 hover:bg-muted/60 border-b last:border-0'
              >
                <td className='px-2 py-1.5'>
                  <div className='flex max-w-20 flex-col gap-1 sm:max-w-none'>
                    <span
                      className='text-foreground truncate font-mono text-xs'
                      title={model.model_name}
                    >
                      {model.model_name}
                    </span>
                    <span className='text-muted-foreground text-xs sm:hidden'>
                      {t('Reference-adjusted multiplier')}{' '}
                      {formatMultiplier(modelPricing?.effectiveRatio ?? null)}
                    </span>
                    {model.coding_cache_observed ? (
                      <span className='text-muted-foreground text-xs'>
                        {t('Cost coverage: {{coverage}}%', { coverage })}
                      </span>
                    ) : null}
                  </div>
                </td>
                <td className={cn(NUMERIC_CELL, MOBILE_HIDDEN_SM)}>
                  {formatCount(model.request_count)}
                </td>
                <td className={cn(NUMERIC_CELL, 'text-right')}>
                  <span className='inline-flex items-center justify-end gap-1.5'>
                    <span
                      className={cn(
                        'size-1.5 rounded-full',
                        getSuccessRateDotClass(model.success_rate)
                      )}
                      aria-hidden
                    />
                    <span
                      className={cn(
                        'font-mono tabular-nums',
                        getSuccessRateTextClass(model.success_rate)
                      )}
                    >
                      {formatUptimePct(model.success_rate)}
                    </span>
                  </span>
                </td>
                <td className={NUMERIC_CELL}>
                  <div className='flex flex-col items-end gap-1 font-mono tabular-nums'>
                    <span>
                      {model.cache_observed
                        ? formatUptimePct(model.cache_hit_rate)
                        : EMPTY_VALUE}
                    </span>
                    <span className='text-muted-foreground text-xs sm:hidden'>
                      {t('Coding cache')}{' '}
                      {model.coding_cache_observed
                        ? formatUptimePct(
                            model.coding_cache_hit_rate ?? Number.NaN
                          )
                        : EMPTY_VALUE}
                    </span>
                  </div>
                </td>
                <td className={cn(NUMERIC_CELL, MOBILE_HIDDEN_SM)}>
                  {model.coding_cache_observed
                    ? formatUptimePct(model.coding_cache_hit_rate ?? Number.NaN)
                    : EMPTY_VALUE}
                </td>
                <td className={cn(NUMERIC_CELL, MOBILE_HIDDEN_SM)}>
                  {formatMultiplier(modelPricing?.effectiveRatio ?? null)}
                </td>
                <td className={cn(NUMERIC_CELL, MOBILE_HIDDEN_SM)}>
                  {formatLatency(model.avg_ttft_ms)}
                </td>
                <td className={cn(NUMERIC_CELL, 'max-md:hidden')}>
                  {formatLatency(model.avg_latency_ms)}
                </td>
                <td className={cn(NUMERIC_CELL, MOBILE_HIDDEN_SM)}>
                  {formatThroughput(model.avg_tps)}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
