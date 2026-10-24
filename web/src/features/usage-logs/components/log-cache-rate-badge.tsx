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

import { Badge } from '@/components/ui/badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { LOG_TYPE_ENUM } from '../constants'
import type { UsageLog } from '../data/schema'
import {
  formatCacheHitRate,
  getCacheHitRateLevel,
  getLogCacheUsage,
} from '../lib/cache-rate'

/**
 * Cache hit rate of a single consume log. The badge is hidden whenever the row
 * carries no cache reads or no usable usage data, because the rate would then
 * describe a request whose cache behaviour cannot be identified.
 */
export function LogCacheRateBadge(props: { log: UsageLog }) {
  const { t, i18n } = useTranslation()

  if (props.log.type !== LOG_TYPE_ENUM.CONSUME) return null

  const usage = getLogCacheUsage(props.log)
  if (!usage || usage.cachedTokens <= 0) return null

  const rateText = formatCacheHitRate(usage, i18n.language)
  const title = t('Cache hit rate: {{rate}}', { rate: rateText })

  return (
    <TooltipProvider delay={200}>
      <Tooltip>
        <TooltipTrigger
          render={
            <Badge
              variant={getCacheHitRateLevel(usage.rate)}
              render={<button type='button' />}
              className='shrink-0 rounded-md px-1.5 text-sm leading-none'
              aria-label={title}
              onClick={(event) => event.stopPropagation()}
            >
              <span className='font-mono tabular-nums'>{rateText}</span>
            </Badge>
          }
        />
        <TooltipContent className='max-w-72'>
          <div className='flex flex-col gap-1.5'>
            <p className='font-medium'>{title}</p>
            <p>
              {t('Cache reads {{cached}} / total input {{input}} tokens', {
                cached: usage.cachedTokens.toLocaleString(i18n.language),
                input: usage.inputTokens.toLocaleString(i18n.language),
              })}
            </p>
            <p>
              {t(
                'Cache rate = total cached input tokens ÷ total input tokens. Cache writes count as input, not cache hits; output tokens are excluded.'
              )}
            </p>
            <p>{t('Green: >90%. Yellow: 50–90%. Red: <50%.')}</p>
            <p>
              {t(
                'Hidden when no cache reads are recorded or usage is unavailable; a first request cannot be identified reliably.'
              )}
            </p>
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
