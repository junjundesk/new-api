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
import { useTranslation } from 'react-i18next'

import { formatUptimePct } from '@/features/performance-metrics/lib/format'
import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'
import type { PerformanceGroupSummary } from '@/features/performance-metrics/types'

import { formatMultiplier } from '../lib/format'

export type CachePricingGridProps = {
  summary: PerformanceGroupSummary
  pricing: PerfGroupPricing | null
}

/** Cache hit / Coding cache / observed and reference-adjusted multipliers. */
export function CachePricingGrid(props: CachePricingGridProps) {
  const { t } = useTranslation()
  const pricing = props.pricing
  const coverage =
    pricing === null ? '' : (100 * pricing.pricedInputShare).toFixed(1)
  return (
    <dl className='grid grid-cols-2 gap-3 border-t pt-3 sm:grid-cols-4'>
      <div className='flex min-w-0 flex-col gap-1'>
        <dt className='text-muted-foreground text-xs'>{t('Cache hit')}</dt>
        <dd className='font-mono text-sm font-semibold tabular-nums'>
          {props.summary.cache_observed
            ? formatUptimePct(props.summary.cache_hit_rate)
            : '\u2014'}
        </dd>
      </div>
      <div className='flex min-w-0 flex-col gap-1'>
        <dt className='text-muted-foreground text-xs'>{t('Coding cache')}</dt>
        <dd className='font-mono text-sm font-semibold tabular-nums'>
          {props.summary.coding_cache_observed
            ? formatUptimePct(props.summary.coding_cache_hit_rate ?? Number.NaN)
            : '\u2014'}
        </dd>
      </div>
      <div className='flex min-w-0 flex-col gap-1'>
        <dt className='text-muted-foreground text-xs'>
          {t('Observed input multiplier')}
        </dt>
        <dd className='font-mono text-sm font-semibold tabular-nums'>
          {formatMultiplier(pricing?.observedInputMultiplier ?? null)}
        </dd>
      </div>
      <div className='flex min-w-0 flex-col gap-1'>
        <dt className='text-muted-foreground text-xs'>
          {t('Reference-adjusted multiplier')}
        </dt>
        <dd className='font-mono text-sm font-semibold tabular-nums'>
          {formatMultiplier(pricing?.effectiveRatio ?? null)}
          {pricing?.coverage === 'insufficient' ? (
            <span className='sr-only'>
              {t('Pricing coverage is insufficient')}
            </span>
          ) : null}
        </dd>
        {pricing?.coverage === 'insufficient' ? (
          <span className='text-muted-foreground text-xs'>
            {t('Cost coverage: {{coverage}}%', { coverage })}
          </span>
        ) : null}
      </div>
    </dl>
  )
}
