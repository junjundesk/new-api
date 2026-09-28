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

import { cn } from '@/lib/utils'

import {
  GROUP_HEALTH,
  GROUP_HEALTH_ORDER,
  type GroupHealth,
} from '../lib/status'

export type StatusFilterChipsProps = {
  counts: Record<GroupHealth, number>
  active: GroupHealth | null
  shown: number
  total: number
  onToggle: (health: GroupHealth) => void
}

export function StatusFilterChips(props: StatusFilterChipsProps) {
  const { t } = useTranslation()
  return (
    <div
      role='group'
      aria-label={t('Filter groups by status')}
      className='perf-rise-in flex flex-wrap items-center gap-x-2 gap-y-1.5 text-xs'
    >
      {GROUP_HEALTH_ORDER.filter((health) => props.counts[health] > 0).map(
        (health) => {
          const status = GROUP_HEALTH[health]
          const active = props.active === health
          return (
            <button
              key={health}
              type='button'
              aria-pressed={active}
              onClick={() => props.onToggle(health)}
              className={cn(
                'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 font-medium',
                'transition-colors duration-200',
                'focus-visible:ring-ring focus-visible:ring-2 focus-visible:ring-offset-1 focus-visible:outline-none',
                status.textClass,
                active
                  ? 'border-border bg-muted'
                  : 'border-transparent hover:bg-muted/60'
              )}
            >
              <span
                className={cn('size-1.5 rounded-full', status.dotClass)}
                aria-hidden
              />
              {props.counts[health]} {t(status.labelKey)}
            </button>
          )
        }
      )}
      <span className='text-muted-foreground/70 pl-1'>
        {props.active === null
          ? t('{{count}} groups', { count: props.total })
          : t('{{shown}} of {{count}} groups', {
              shown: props.shown,
              count: props.total,
            })}
      </span>
    </div>
  )
}
