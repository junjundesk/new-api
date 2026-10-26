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
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'

import type { FallbackAttemptInfo, FallbackLogInfo } from '../types'

interface LogFallbackBadgeProps {
  fallback?: FallbackLogInfo
  adminAttempts?: FallbackAttemptInfo[]
  isAdmin?: boolean
  className?: string
}

export function LogFallbackBadge(props: LogFallbackBadgeProps) {
  const { t } = useTranslation()
  const fallback = props.fallback
  if (!fallback || (!fallback.from_model && !fallback.to_model)) return null

  const attempts = props.isAdmin ? (props.adminAttempts ?? []) : []

  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type='button'
            aria-label={t('Fallback request')}
            className={cn(
              'inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-md border border-warning/35 bg-warning/10 px-1.5 text-[11px] font-bold leading-none text-warning transition-colors hover:bg-warning/15 focus-visible:ring-ring outline-none focus-visible:ring-2',
              props.className
            )}
            onClick={(event) => event.stopPropagation()}
          >
            {t('Fallback badge')}
          </button>
        }
      />
      <PopoverContent
        className='w-80 max-w-[calc(100vw-2rem)] p-3'
        align='start'
        onClick={(event) => event.stopPropagation()}
      >
        <div className='flex flex-col gap-2.5'>
          <div className='flex items-center gap-2'>
            <span className='inline-flex h-5 min-w-5 items-center justify-center rounded-md border border-warning/35 bg-warning/10 px-1.5 text-[11px] font-bold leading-none text-warning'>
              {t('Fallback badge')}
            </span>
            <span className='text-sm font-semibold'>{t('Fallback request')}</span>
            {fallback.attempt != null && (
              <span className='text-muted-foreground ml-auto text-xs'>
                {t('Attempt {{n}}', { n: fallback.attempt })}
              </span>
            )}
          </div>

          <div className='grid grid-cols-[5rem_minmax(0,1fr)] gap-x-3 gap-y-1.5 text-xs'>
            <span className='text-muted-foreground'>{t('Original model')}</span>
            <span className='min-w-0 break-all font-medium'>
              {fallback.from_model || '-'}
              {fallback.from_group ? (
                <span className='text-muted-foreground'> · {fallback.from_group}</span>
              ) : null}
            </span>
            <span className='text-muted-foreground'>{t('Fallback model')}</span>
            <span className='min-w-0 break-all font-medium'>
              {fallback.to_model || '-'}
              {fallback.to_group ? (
                <span className='text-muted-foreground'> · {fallback.to_group}</span>
              ) : null}
            </span>
            {fallback.failed && (
              <>
                <span className='text-muted-foreground'>{t('Result')}</span>
                <span className='text-destructive'>
                  {t('All fallback models failed ({{count}} attempts)', {
                    count: fallback.attempts ?? 0,
                  })}
                </span>
              </>
            )}
          </div>

          {attempts.length > 0 && (
            <div className='border-t pt-2'>
              <p className='text-muted-foreground mb-1 text-[11px] font-medium'>
                {t('Attempt history (admin)')}
              </p>
              <div className='flex flex-col gap-1'>
                {attempts.map((attempt) => (
                  <div
                    key={`${attempt.index}-${attempt.group}-${attempt.model}`}
                    className='flex items-start gap-2 rounded bg-muted/40 px-2 py-1 font-mono text-[10px] leading-relaxed'
                  >
                    <span className='text-muted-foreground shrink-0'>
                      #{attempt.index}
                    </span>
                    <span className='min-w-0 break-all'>
                      {attempt.group}/{attempt.model}
                      {attempt.channel_id ? ` · #${attempt.channel_id}` : ''}
                      {attempt.status_code ? ` · ${attempt.status_code}` : ''}
                      {attempt.error ? ` · ${attempt.error}` : ''}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
