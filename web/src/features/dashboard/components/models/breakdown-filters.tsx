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
import { ChevronDown, Filter } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import type { BreakdownFilterOption } from '@/features/dashboard/lib'
import { formatQuota } from '@/lib/format'

export interface BreakdownFilterSelectProps {
  // Accessible name of the control, e.g. "Filter by username".
  label: string
  // Label of the "no narrowing" choice, e.g. "All users".
  allLabel: string
  searchPlaceholder: string
  options: BreakdownFilterOption[]
  // Empty string means "do not narrow by this dimension".
  value: string
  onChange: (value: string) => void
}

export function BreakdownFilterSelect(props: BreakdownFilterSelectProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const selectedOption = props.options.find(
    (option) => option.value === props.value
  )
  const triggerLabel =
    props.value === ''
      ? props.allLabel
      : (selectedOption?.label ?? props.value)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            size='sm'
            aria-label={props.label}
          />
        }
      >
        <Filter data-icon='inline-start' aria-hidden='true' />
        <span className='max-w-40 truncate'>{triggerLabel}</span>
        <ChevronDown data-icon='inline-end' aria-hidden='true' />
      </PopoverTrigger>
      <PopoverContent
        className='w-[min(20rem,calc(100vw-2rem))] p-0'
        align='start'
      >
        <Command>
          <CommandInput placeholder={props.searchPlaceholder} />
          <CommandList>
            <CommandEmpty>{t('No data')}</CommandEmpty>
            <CommandItem
              value={props.allLabel}
              data-checked={props.value === ''}
              onSelect={() => {
                props.onChange('')
                setOpen(false)
              }}
            >
              <span className='min-w-0 flex-1 truncate'>{props.allLabel}</span>
            </CommandItem>
            {props.options.map((option) => (
              <CommandItem
                key={option.value}
                value={option.label}
                data-checked={props.value === option.value}
                onSelect={() => {
                  props.onChange(option.value)
                  setOpen(false)
                }}
              >
                <span className='min-w-0 flex-1 truncate'>{option.label}</span>
                <span className='text-muted-foreground shrink-0 font-mono text-xs tabular-nums'>
                  {formatQuota(option.quota)}
                </span>
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
