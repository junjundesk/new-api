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

/**
 * Original (1x ratio) price crossed out next to the effective ratio price.
 *
 * The crossed-out line is red so a discounted or surcharged price stays
 * recognizable at a glance; the amount itself keeps the muted styling of
 * secondary price text.
 */
export function OriginalPrice(props: { value: string; className?: string }) {
  const { t } = useTranslation()
  return (
    <span
      data-slot='original-price'
      title={t('Original price')}
      className={cn(
        'text-muted-foreground/60 font-mono line-through decoration-red-500',
        props.className
      )}
    >
      {props.value}
    </span>
  )
}
