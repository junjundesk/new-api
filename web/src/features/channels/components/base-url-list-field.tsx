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
import { Trash2 } from 'lucide-react'
import { useId, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { isValidBaseUrl, MAX_BASE_URLS } from '../lib/channel-form'

type BaseUrlListFieldProps = {
  value: string[]
  onChange: (value: string[]) => void
  onBlur?: () => void
  disabled?: boolean
  placeholder?: string
}

export function BaseUrlListField(props: BaseUrlListFieldProps) {
  const { t } = useTranslation()
  const id = useId()
  // Keep at least one row visible even when the field value is empty.
  const rows = props.value.length > 0 ? props.value : ['']
  // Stable per-row keys so removing a middle row does not shift DOM state.
  const rowIdsRef = useRef<string[]>([])
  const rowIdSeqRef = useRef(0)
  const rowIds = rowIdsRef.current
  if (rowIds.length < rows.length) {
    while (rowIds.length < rows.length) {
      rowIdSeqRef.current += 1
      rowIds.push(`${id}-row-${rowIdSeqRef.current}`)
    }
  } else if (rowIds.length > rows.length) {
    rowIds.length = rows.length
  }

  const updateRow = (index: number, nextValue: string) => {
    const nextRows = [...rows]
    nextRows[index] = nextValue
    props.onChange(nextRows)
  }

  const removeRow = (index: number) => {
    rowIds.splice(index, 1)
    const nextRows = rows.filter((_, rowIndex) => rowIndex !== index)
    props.onChange(nextRows.length > 0 ? nextRows : [''])
  }

  return (
    <div className='grid gap-2' onBlur={props.onBlur}>
      {rows.map((row, index) => {
        const trimmed = row.trim()
        const rowInvalid = trimmed !== '' && !isValidBaseUrl(trimmed)
        const rowErrorId = `${id}-${index}-error`
        return (
          <div key={rowIds[index]} className='flex min-w-0 items-start gap-2'>
            <span
              className='text-muted-foreground w-20 shrink-0 pt-2 text-sm'
              aria-hidden='true'
            >
              {index === 0 ? t('Primary address') : t('Backup address')}
            </span>
            <div className='grid min-w-0 flex-1 gap-1'>
              <Input
                id={`${id}-${index}`}
                value={row}
                disabled={props.disabled}
                placeholder={props.placeholder}
                aria-label={t('API address {{index}}', { index: index + 1 })}
                aria-invalid={rowInvalid || undefined}
                aria-describedby={rowInvalid ? rowErrorId : undefined}
                onChange={(event) => updateRow(index, event.target.value)}
              />
              {rowInvalid && (
                <p
                  id={rowErrorId}
                  role='alert'
                  className='text-destructive text-sm'
                >
                  {t('Please enter a valid http/https absolute URL')}
                </p>
              )}
            </div>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              disabled={props.disabled || rows.length <= 1}
              aria-label={t('Remove API address {{index}}', {
                index: index + 1,
              })}
              onClick={() => removeRow(index)}
            >
              <Trash2 aria-hidden='true' />
            </Button>
          </div>
        )
      })}
      <div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={props.disabled || rows.length >= MAX_BASE_URLS}
          onClick={() => props.onChange([...rows, ''])}
        >
          {t('Add backup address')}
        </Button>
      </div>
    </div>
  )
}
