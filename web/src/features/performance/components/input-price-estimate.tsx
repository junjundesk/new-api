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

import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'
import { toIntlLocale } from '@/i18n/languages'

import { EMPTY_VALUE, formatMultiplier } from '../lib/format'

export type InputPriceEstimateProps = {
  pricing: PerfGroupPricing | null
}

/** Six derived USD/1M figures plus the coverage notes behind them. */
export function InputPriceEstimate(props: InputPriceEstimateProps) {
  const { t, i18n } = useTranslation()
  // zhCN is an interface code, not a BCP-47 tag: feeding it to Intl throws
  // RangeError, so it is canonicalized first.
  const language = toIntlLocale(i18n.resolvedLanguage ?? i18n.language)
  const currency = useMemo(
    () =>
      new Intl.NumberFormat(language, {
        style: 'currency',
        currency: 'USD',
        minimumFractionDigits: 2,
        maximumFractionDigits: 4,
      }),
    [language]
  )
  const pricing = props.pricing

  if (pricing === null) {
    return (
      <p className='text-muted-foreground text-xs'>
        {t('No valid Coding input was observed in this window.')}
      </p>
    )
  }

  const money = (value: number | null) =>
    value === null || !Number.isFinite(value)
      ? EMPTY_VALUE
      : currency.format(value)
  const rate =
    pricing.referenceCacheRate === null
      ? EMPTY_VALUE
      : String(Number((100 * pricing.referenceCacheRate).toFixed(2)))
  const observed = String(Number((100 * pricing.observedCacheRate).toFixed(2)))
  const coverage = (100 * pricing.pricedInputShare).toFixed(1)

  const figures: { labelKey: string; value: string; strong: boolean }[] = [
    {
      labelKey: 'Input price at 0% reference cache',
      value: money(pricing.baseInputPricePerMillion),
      strong: false,
    },
    {
      labelKey: 'Input price at 100% reference cache',
      value: money(pricing.baseCacheReadPricePerMillion),
      strong: false,
    },
    {
      labelKey: 'Observed input price',
      value: money(pricing.estimatedInputPricePerMillion),
      strong: true,
    },
    {
      labelKey: 'Reference input price',
      value: money(pricing.referenceInputPricePerMillion),
      strong: false,
    },
    {
      labelKey: 'Observed input multiplier',
      value: formatMultiplier(pricing.observedInputMultiplier),
      strong: false,
    },
    {
      labelKey: 'Reference-adjusted multiplier',
      value: formatMultiplier(pricing.effectiveRatio),
      strong: false,
    },
  ]

  return (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-wrap items-baseline justify-between gap-2'>
        <h3 className='text-sm font-semibold'>{t('Input price estimate')}</h3>
        <span className='text-muted-foreground text-xs'>
          {t('USD per 1M input tokens')}
        </span>
      </div>
      <dl className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
        {figures.map((figure) => (
          <div key={figure.labelKey} className='flex min-w-0 flex-col gap-1'>
            <dt className='text-muted-foreground text-xs'>
              {t(figure.labelKey)}
            </dt>
            <dd
              className={
                figure.strong
                  ? 'font-mono text-sm font-semibold tabular-nums'
                  : 'font-mono text-sm tabular-nums'
              }
            >
              {figure.value}
            </dd>
          </div>
        ))}
      </dl>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Reference cache {{rate}}% \u00b7 all Coding cache {{observed}}% \u00b7 input cost coverage {{coverage}}%',
          { rate, observed, coverage }
        )}
      </p>
      {pricing.coverage === 'insufficient' ? (
        <p className='text-muted-foreground text-xs'>
          {t(
            'Prices describe the observed subset. Multipliers require 90% input cost coverage for the group or model being shown.'
          )}
        </p>
      ) : null}
      <p className='text-muted-foreground text-xs'>
        {t(
          'Observed input cost uses each request\u2019s settlement prices and multiplier. Reference cost uses the same pricing rules before the group multiplier; these are site prices, not verified official prices.'
        )}
      </p>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Input costs include regular input, cache reads, and cache writes. Output, tools, and recharge discounts are excluded. Historical usage without cost observations and unsupported pricing reduce coverage.'
        )}
      </p>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Costs are calculated before quota rounding. Nonlinear, cache-dependent, media, and per-request pricing may remain unobserved.'
        )}
      </p>
    </div>
  )
}
