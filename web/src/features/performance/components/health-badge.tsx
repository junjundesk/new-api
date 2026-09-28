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

import { GROUP_HEALTH, type GroupHealth } from '../lib/status'

export type HealthBadgeProps = {
  health: GroupHealth
  className?: string
}

export function HealthBadge(props: HealthBadgeProps) {
  const { t } = useTranslation()
  const status = GROUP_HEALTH[props.health]
  const pulsing = props.health !== 'nodata'
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium',
        status.textClass,
        props.className
      )}
    >
      <span
        className={cn(
          'size-1.5 rounded-full',
          status.dotClass,
          pulsing && 'animate-pulse motion-reduce:animate-none'
        )}
        aria-hidden
      />
      {t(status.labelKey)}
    </span>
  )
}
