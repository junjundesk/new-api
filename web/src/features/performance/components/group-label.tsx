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

export type GroupLabelProps = {
  group: string
  /** Backend display name; falls back to the group key. */
  label?: string
  className?: string
}

/**
 * Group badge: an empty group key renders as User Group and the auto group as
 * Auto; an explicit label always wins.
 */
export function GroupLabel(props: GroupLabelProps) {
  const { t } = useTranslation()
  const isEmptyGroup = props.group === ''
  let text = props.label ?? props.group
  if (props.label === undefined) {
    if (isEmptyGroup) text = t('User Group')
    else if (props.group === 'auto') text = t('Auto')
  }
  return (
    <span
      className={cn('min-w-0 truncate text-xs font-medium', props.className)}
    >
      {text}
    </span>
  )
}
