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

import { StatusBadge, type StatusBadgeProps } from './status-badge'

type GroupBadgeProps = Omit<StatusBadgeProps, 'children' | 'variant'> & {
  group?: string | null
  label?: string
  ratio?: number | null
}

function getGroupRatioVariant(
  ratio: number
): NonNullable<StatusBadgeProps['variant']> {
  if (ratio > 1) return 'warning'
  if (ratio < 1) return 'info'
  return 'neutral'
}

function getGroupLabel(params: {
  labelOverride?: string
  groupName?: string
  isAutoGroup: boolean
  isEmptyGroup: boolean
  t: (key: string) => string
}): string {
  if (params.labelOverride) return params.labelOverride
  if (params.isEmptyGroup) return params.t('User Group')
  if (params.isAutoGroup) return params.t('Auto')
  return params.groupName ?? ''
}

export function GroupBadge(props: GroupBadgeProps) {
  const { t } = useTranslation()
  const { group, label: labelOverride, ratio, className, ...badgeProps } = props
  const groupName = group?.trim()
  const isAutoGroup = groupName === 'auto'
  const isEmptyGroup = !groupName
  const label = getGroupLabel({
    labelOverride,
    groupName,
    isAutoGroup,
    isEmptyGroup,
    t,
  })

  const badge = (
    <StatusBadge
      {...badgeProps}
      variant='neutral'
      className={cn('min-w-0 shrink overflow-hidden', className)}
    >
      {label}
    </StatusBadge>
  )

  if (ratio == null) {
    return badge
  }

  return (
    <span className='inline-flex max-w-full min-w-0 items-center gap-2 text-xs'>
      <span className='max-w-full min-w-0 overflow-hidden'>{badge}</span>
      <StatusBadge
        variant={getGroupRatioVariant(ratio)}
        className='shrink-0 tabular-nums'
      >
        {ratio}x
      </StatusBadge>
    </span>
  )
}
