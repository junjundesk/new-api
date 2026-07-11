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
import { CircleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatUseTime } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { LogOtherData } from '../types'

interface TimingMetricsCellProps {
  useTimeSec: number
  frtMs?: number
  isStream: boolean
  className?: string
}

export function TimingMetricsCell(props: TimingMetricsCellProps) {
  const { t } = useTranslation()
  const showFirstToken = props.isStream
  const hasFrt = props.frtMs != null && props.frtMs > 0
  const firstTokenLabel = hasFrt
    ? formatUseTime(props.frtMs! / 1000)
    : t('N/A')
  const totalTimeLabel = formatUseTime(props.useTimeSec)

  return (
    <div className={cn('flex items-stretch gap-2', props.className)}>
      <span
        aria-hidden
        className={cn(
          'w-1 shrink-0 rounded-full',
          showFirstToken
            ? 'bg-linear-to-b from-success to-warning'
            : 'bg-warning'
        )}
      />
      <div className='flex min-w-0 flex-col justify-center gap-0.5 text-xs leading-tight'>
        <div
          className={cn(
            'flex items-baseline gap-1.5',
            !showFirstToken && 'invisible'
          )}
          aria-hidden={!showFirstToken}
        >
          <span className='text-muted-foreground shrink-0'>
            {t('First token')}
          </span>
          <span
            className={cn(
              'tabular-nums',
              hasFrt ? 'text-success' : 'text-muted-foreground'
            )}
          >
            {showFirstToken ? firstTokenLabel : '—'}
          </span>
        </div>
        <div className='flex items-baseline gap-1.5'>
          <span className='text-muted-foreground shrink-0'>
            {t('Duration')}
          </span>
          <span className='text-warning tabular-nums'>{totalTimeLabel}</span>
        </div>
      </div>
    </div>
  )
}

interface StreamTpsCellProps {
  isStream: boolean
  tokensPerSecond?: number | null
  streamStatus?: LogOtherData['stream_status']
  className?: string
}

export function StreamTpsCell(props: StreamTpsCellProps) {
  const { t } = useTranslation()
  const showStreamError =
    props.isStream &&
    props.streamStatus &&
    props.streamStatus.status !== 'ok'
  const tpsLabel =
    props.tokensPerSecond != null
      ? `${Math.round(props.tokensPerSecond)} t/s`
      : '—'

  return (
    <div
      className={cn(
        'flex shrink-0 flex-col items-start justify-center gap-0.5 text-xs leading-tight',
        props.className
      )}
    >
      <span className='border-border/60 bg-muted/30 text-muted-foreground inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 leading-none'>
        {props.isStream ? t('Stream') : t('Non-stream')}
        {showStreamError && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger
                render={<CircleAlert className='text-destructive size-3' />}
              />
              <TooltipContent>
                <div className='space-y-0.5 text-xs'>
                  <p>
                    {t('Stream Status')}: {t('Error')}
                  </p>
                  <p>{props.streamStatus?.end_reason || 'unknown'}</p>
                  {(props.streamStatus?.error_count ?? 0) > 0 && (
                    <p>
                      {t('Soft Errors')}: {props.streamStatus?.error_count}
                    </p>
                  )}
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
      </span>
      <span className='text-muted-foreground/60 px-0.5 tabular-nums'>
        {tpsLabel}
      </span>
    </div>
  )
}
