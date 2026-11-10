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
import { useQuery } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, Trash2 } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { getGroupModelNames } from '../api'
import type { UserChannelChainItemRow } from '../lib/user-channel-chain'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'

type ChannelChainModelRowProps = {
  row: UserChannelChainItemRow
  index: number
  total: number
  groupOptions: ApiKeyGroupOption[]
  maxModelsPerItem: number
  open: boolean
  disabled?: boolean
  groupError?: string
  modelsError?: string
  onGroupChange: (value: string) => void
  onModelsChange: (models: string[]) => void
  onMoveUp: () => void
  onMoveDown: () => void
  onRemove: () => void
}

export function ChannelChainModelRow(props: ChannelChainModelRowProps) {
  const { t } = useTranslation()
  const modelsQuery = useQuery({
    queryKey: ['group-models', props.row.group],
    queryFn: () => getGroupModelNames(props.row.group),
    enabled: props.open && !!props.row.group,
    staleTime: 60_000,
  })

  const models = modelsQuery.data
  const modelOptions = useMemo(() => {
    const available = models ?? []
    const known = new Set(available)
    const options = available.map((model) => ({ value: model, label: model }))
    for (const saved of props.row.models) {
      if (!known.has(saved)) {
        options.push({
          value: saved,
          label: `${saved} (${t('Unavailable')})`,
        })
      }
    }
    return options
  }, [models, props.row.models, t])

  const loading = !!props.row.group && modelsQuery.isPending
  const failed = !!props.row.group && modelsQuery.isError
  const empty =
    !!props.row.group && !loading && !failed && (models?.length ?? 0) === 0

  let placeholder = t('Select models for this row')
  if (!props.row.group) {
    placeholder = t('Select a group first')
  } else if (loading) {
    placeholder = t('Loading models...')
  } else if (failed) {
    placeholder = t('Failed to load models')
  } else if (empty) {
    placeholder = t('No available model found')
  }

  return (
    <div
      data-channel-chain-model-row=''
      className='flex flex-col gap-2 rounded-lg border p-2 sm:flex-row sm:items-start'
    >
      <div className='flex min-w-0 flex-1 flex-col gap-1.5 sm:flex-row sm:items-start'>
        {/* The group cell sizes to the selected group label (capped at half
            the row) so long group names stay readable, while the model picker
            keeps the remaining space and stacks below on narrow screens. */}
        <div
          data-channel-chain-group-cell=''
          className='w-full min-w-0 sm:w-auto sm:max-w-[50%] sm:min-w-[8rem] sm:shrink-0'
        >
          <ApiKeyGroupCombobox
            options={props.groupOptions}
            value={props.row.group || undefined}
            onValueChange={props.onGroupChange}
            placeholder={t('Select a group')}
            disabled={props.disabled}
            ariaLabel={t('Group for model row')}
            compact
          />
          {props.groupError ? (
            <p className='text-destructive mt-1 text-xs' role='alert'>
              {props.groupError}
            </p>
          ) : null}
        </div>

        <div className='flex w-full min-w-0 flex-1 flex-col gap-1.5'>
          {loading ? (
            <div className='flex items-center gap-2' role='status'>
              <Skeleton className='h-9 w-full' />
              <span className='sr-only'>{t('Loading models...')}</span>
            </div>
          ) : (
            <MultiSelect
              options={modelOptions}
              selected={props.row.models}
              onChange={props.onModelsChange}
              placeholder={placeholder}
              ariaLabel={t('Models for this row')}
              disabled={props.disabled || !props.row.group || failed}
              maxVisibleChips={3}
            />
          )}
          {failed ? (
            <div className='flex items-center gap-1 text-xs'>
              <span className='text-destructive'>
                {t('Could not load models')}
              </span>
              <Button
                type='button'
                variant='link'
                size='sm'
                className='h-auto px-1 py-0 text-xs'
                onClick={() => modelsQuery.refetch()}
              >
                {t('Retry')}
              </Button>
            </div>
          ) : null}
          {props.modelsError ? (
            <p className='text-destructive text-xs' role='alert'>
              {props.modelsError}
            </p>
          ) : null}
          <span className='text-muted-foreground text-xs'>
            {t('Models: {{used}}/{{max}}', {
              used: props.row.models.length,
              max: props.maxModelsPerItem,
            })}
          </span>
        </div>
      </div>

      <div className='flex shrink-0 items-center justify-end gap-1'>
        <span className='text-muted-foreground mr-auto text-xs tabular-nums sm:hidden'>
          {t('Row {{index}}', { index: props.index + 1 })}
        </span>
        <Button
          type='button'
          variant='outline'
          size='icon'
          className={cn('size-8')}
          aria-label={t('Move model row up')}
          disabled={props.disabled || props.index === 0}
          onClick={props.onMoveUp}
        >
          <ArrowUp aria-hidden='true' className='size-4' />
        </Button>
        <Button
          type='button'
          variant='outline'
          size='icon'
          className='size-8'
          aria-label={t('Move model row down')}
          disabled={props.disabled || props.index === props.total - 1}
          onClick={props.onMoveDown}
        >
          <ArrowDown aria-hidden='true' className='size-4' />
        </Button>
        <Button
          type='button'
          variant='outline'
          size='icon'
          className='text-destructive size-8'
          aria-label={t('Remove model row')}
          disabled={props.disabled}
          onClick={props.onRemove}
        >
          <Trash2 aria-hidden='true' className='size-4' />
        </Button>
      </div>
    </div>
  )
}
