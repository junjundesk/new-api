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
import { ChevronDown } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/design-system/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

export type DataTableContentMode = 'full' | 'wrap' | 'summary'

interface DataTableCardFieldProps {
  children: ReactNode
  className?: string
  contentMode?: DataTableContentMode
  label?: ReactNode
  span?: 1 | 2
  valueClassName?: string
}

export function DataTableCardField({
  children,
  className,
  contentMode = 'wrap',
  label,
  span = 1,
  valueClassName,
}: DataTableCardFieldProps) {
  return (
    <div
      data-slot='data-table-card-field'
      className={cn('min-w-0', span === 2 && 'col-span-2', className)}
    >
      {label && (
        <div className='text-muted-foreground mb-1 text-xs leading-none font-medium select-none'>
          {label}
        </div>
      )}
      <div
        data-slot='data-table-card-value'
        className={cn(
          'min-w-0 text-sm leading-snug',
          (contentMode === 'full' || contentMode === 'wrap') &&
            'whitespace-normal break-words [overflow-wrap:anywhere] [&_.truncate]:overflow-visible [&_.truncate]:text-clip [&_.truncate]:whitespace-normal [&_.whitespace-nowrap]:whitespace-normal [&_[data-slot=status-badge]]:h-auto [&_[data-slot=status-badge]]:overflow-visible [&_[data-slot=status-badge-label]]:overflow-visible [&_[data-slot=status-badge-label]]:text-clip [&_[data-slot=status-badge-label]]:whitespace-normal',
          contentMode === 'full' && 'break-all',
          valueClassName
        )}
      >
        {children ?? '-'}
      </div>
    </div>
  )
}

interface DataTableCardDetailsProps {
  children: ReactNode
  className?: string
  count?: number
  defaultOpen?: boolean
}

export function DataTableCardDetails({
  children,
  className,
  count,
  defaultOpen = false,
}: DataTableCardDetailsProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(defaultOpen)

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className={cn('mt-2', className)}
    >
      <CollapsibleTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='xs'
            className='text-muted-foreground hover:text-foreground group/details -ml-2'
          />
        }
      >
        {open ? t('Less') : t('More')}
        {!open && count != null && count > 0 && (
          <span className='tabular-nums'>({count})</span>
        )}
        <ChevronDown className='size-3.5 transition-transform duration-150 group-data-[panel-open]/details:rotate-180' />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className='mt-1.5 grid grid-cols-2 gap-x-3 gap-y-2 border-t pt-2'>
          {children}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}
