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
import { Pencil } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'

import {
  getExposedNames,
  parseExposedNameInput,
  setUpstreamExposedName,
} from '../../lib/model-mapping-edit'

type ModelMappingPopoverProps = {
  /** Upstream model name shown in the fetched model list. */
  model: string
  modelMapping: string
  disabled?: boolean
  onApply: (modelMapping: string, exposedName: string) => void
}

const INVALID_MAPPING_MESSAGE =
  'Fix the existing model mapping JSON in the channel form first'

type ModelMappingError = 'invalid-mapping' | 'invalid-name' | 'conflict'

export function ModelMappingPopover(props: ModelMappingPopoverProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState('')
  const [previousName, setPreviousName] = useState('')
  const [error, setError] = useState<ModelMappingError | null>(null)
  const [blockedByInvalidJson, setBlockedByInvalidJson] = useState(false)

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    setError(null)
    if (!nextOpen) return

    const exposedNames = getExposedNames(props.modelMapping, props.model)
    setBlockedByInvalidJson(exposedNames === null)
    if (exposedNames === null) {
      setDraft('')
      setPreviousName('')
      setError('invalid-mapping')
      return
    }
    setPreviousName(exposedNames[0] ?? '')
    setDraft(exposedNames[0] ?? '')
  }

  const handleApply = () => {
    const parsed = parseExposedNameInput(draft)
    if (parsed.error) {
      setError(parsed.error)
      return
    }

    const result = setUpstreamExposedName(
      props.modelMapping,
      props.model,
      previousName,
      parsed.name
    )
    if (result.conflict) {
      setError('conflict')
      return
    }
    if (result.mapping === null) {
      setBlockedByInvalidJson(true)
      setError('invalid-mapping')
      return
    }

    props.onApply(result.mapping, parsed.name)
    setOpen(false)
  }

  let errorMessage: string | null = null
  if (error === 'invalid-mapping') {
    errorMessage = t(INVALID_MAPPING_MESSAGE)
  } else if (error === 'invalid-name') {
    errorMessage = t(
      'Model names must not contain spaces or exceed 255 characters'
    )
  } else if (error === 'conflict') {
    errorMessage = t('This name is already mapped to another upstream model')
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground size-6'
            disabled={props.disabled}
            aria-label={t('Edit model mapping')}
            title={t('Edit model mapping')}
          />
        }
      >
        <Pencil className='size-3.5' aria-hidden='true' />
      </PopoverTrigger>
      <PopoverContent
        align='end'
        className='w-[min(20rem,calc(100vw-2rem))] gap-3 p-3'
      >
        <PopoverHeader className='gap-1'>
          <PopoverTitle>{t('Model Mapping')}</PopoverTitle>
          <PopoverDescription className='text-xs'>
            {t('Upstream model: {{model}}', { model: props.model })}
          </PopoverDescription>
        </PopoverHeader>
        <Input
          aria-label={t('Public model name')}
          placeholder={t('Public model name')}
          value={draft}
          disabled={blockedByInvalidJson}
          aria-invalid={error !== null}
          onChange={(event) => {
            setDraft(event.target.value)
            setError(null)
          }}
        />
        <p className='text-muted-foreground text-xs leading-relaxed'>
          {t(
            'Clients request this name; the request is forwarded to {{model}}.',
            { model: props.model }
          )}
        </p>
        {errorMessage && (
          <p className='text-destructive text-xs'>{errorMessage}</p>
        )}
        <div className='flex justify-end gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => setOpen(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            size='sm'
            disabled={blockedByInvalidJson}
            onClick={handleApply}
          >
            {t('Save')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
