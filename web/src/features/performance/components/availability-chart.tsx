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

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  formatLatency,
  getSuccessRateDotClass,
} from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'

import { formatSlotRange, perfDelayStyle } from '../lib/format'
import {
  getAvailabilityBarHeightClass,
  type AvailabilitySlot,
} from '../lib/series'

export type AvailabilityChartProps = {
  slots: AvailabilitySlot[]
  ariaLabel: string
  baseDelayMs?: number
}

/** Pure-DOM availability bars: one bar per aggregated bucket, newest last. */
export function AvailabilityChart(props: AvailabilityChartProps) {
  const { t } = useTranslation()
  const baseDelayMs = props.baseDelayMs ?? 0
  return (
    <div className='flex flex-col gap-1'>
      {props.slots.length === 0 ? (
        <div className='bg-muted/40 h-9 w-full rounded-md' aria-hidden />
      ) : (
        <div
          className='flex h-9 w-full items-end gap-[2px]'
          role='img'
          aria-label={props.ariaLabel}
        >
          {props.slots.map((slot, index) => (
            <Tooltip key={slot.ts}>
              <TooltipTrigger
                render={
                  <div className='flex h-full min-w-[2px] flex-1 items-end transition-opacity hover:opacity-75' />
                }
              >
                <div
                  style={perfDelayStyle(
                    baseDelayMs + Math.min(12 * index, 300)
                  )}
                  className={cn(
                    'perf-bar-grow w-full rounded-[2px]',
                    slot.successRate === null
                      ? 'bg-muted-foreground/15'
                      : getSuccessRateDotClass(slot.successRate),
                    getAvailabilityBarHeightClass(slot.successRate)
                  )}
                  aria-hidden
                />
              </TooltipTrigger>
              <TooltipContent side='top' className='font-mono text-xs'>
                <div className='font-medium'>
                  {formatSlotRange(slot.ts, slot.spanSeconds)}
                </div>
                {slot.successRate === null ? (
                  <div className='text-muted-foreground'>{t('No data')}</div>
                ) : (
                  <>
                    <div>{slot.successRate.toFixed(2)}%</div>
                    <div className='text-muted-foreground'>
                      {t('{{count}} requests', { count: slot.requestCount })}
                      {slot.avgLatencyMs > 0
                        ? ` \u00b7 ${formatLatency(slot.avgLatencyMs)}`
                        : ''}
                    </div>
                  </>
                )}
              </TooltipContent>
            </Tooltip>
          ))}
        </div>
      )}
      <div className='text-muted-foreground/60 flex items-center justify-between text-[10px] font-medium tracking-wider uppercase'>
        <span>{t('Past')}</span>
        <span>{t('Now')}</span>
      </div>
    </div>
  )
}
