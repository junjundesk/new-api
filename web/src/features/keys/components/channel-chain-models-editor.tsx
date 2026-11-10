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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import {
  createUserChannelChainItemRow,
  getItemsBytes,
  toUserChannelChainItems,
  type UserChannelChainItemRow,
} from '../lib/user-channel-chain'
import type { ApiKeyGroupOption } from './api-key-group-combobox'
import { ChannelChainModelRow } from './channel-chain-model-row'

type RowErrors = {
  group?: string
  models?: string
}

type ChannelChainModelsEditorProps = {
  items: UserChannelChainItemRow[]
  groupOptions: ApiKeyGroupOption[]
  maxModelsPerItem: number
  maxGroupsPerChain: number
  maxItemsBytes: number
  open: boolean
  disabled?: boolean
  rootError?: string
  rowErrors: RowErrors[]
  onChange: (items: UserChannelChainItemRow[]) => void
}

export function ChannelChainModelsEditor(props: ChannelChainModelsEditorProps) {
  const { t } = useTranslation()
  const usedGroups = new Set(
    props.items.map((item) => item.group).filter(Boolean)
  )
  const optionsWithSaved = [...props.groupOptions]
  const knownGroups = new Set(optionsWithSaved.map((option) => option.value))
  for (const item of props.items) {
    if (item.group && !knownGroups.has(item.group)) {
      knownGroups.add(item.group)
      optionsWithSaved.push({
        value: item.group,
        label: `${item.group} (${t('Unavailable')})`,
      })
    }
  }

  const bytes = getItemsBytes(toUserChannelChainItems(props.items))
  const hasFreeGroup = optionsWithSaved.some(
    (option) => !usedGroups.has(option.value)
  )
  const canAddRow =
    !props.disabled &&
    props.items.length < props.maxGroupsPerChain &&
    (hasFreeGroup || props.items.length === 0)

  const updateRow = (index: number, row: UserChannelChainItemRow) => {
    const next = props.items.map((item, current) =>
      current === index ? row : item
    )
    props.onChange(next)
  }
  const moveRow = (index: number, offset: number) => {
    const target = index + offset
    if (target < 0 || target >= props.items.length) return
    const next = [...props.items]
    ;[next[index], next[target]] = [next[target], next[index]]
    props.onChange(next)
  }

  return (
    <div className='space-y-2'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <p className='text-muted-foreground text-xs'>
          {t('Rows: {{used}}/{{max}}', {
            used: props.items.length,
            max: props.maxGroupsPerChain,
          })}
          {' · '}
          {t('{{used}} KB of {{max}} KB used', {
            used: (bytes / 1024).toFixed(1),
            max: Math.round(props.maxItemsBytes / 1024),
          })}
        </p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={!canAddRow}
          onClick={() =>
            props.onChange([...props.items, createUserChannelChainItemRow()])
          }
        >
          <Plus aria-hidden='true' className='size-4' />
          {t('Add model row')}
        </Button>
      </div>
      {props.rootError ? (
        <p className='text-destructive text-xs' role='alert'>
          {props.rootError}
        </p>
      ) : null}
      {props.items.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t('Add at least one model row to the chain')}
        </p>
      ) : (
        <div className='space-y-2'>
          {props.items.map((row, index) => (
            <ChannelChainModelRow
              key={row.id}
              row={row}
              index={index}
              total={props.items.length}
              groupOptions={optionsWithSaved.filter(
                (option) =>
                  option.value === row.group || !usedGroups.has(option.value)
              )}
              maxModelsPerItem={props.maxModelsPerItem}
              open={props.open}
              disabled={props.disabled}
              groupError={props.rowErrors[index]?.group}
              modelsError={props.rowErrors[index]?.models}
              onGroupChange={(group) => {
                if (group === row.group) return
                updateRow(index, { ...row, group, models: [] })
              }}
              onModelsChange={(models) => updateRow(index, { ...row, models })}
              onMoveUp={() => moveRow(index, -1)}
              onMoveDown={() => moveRow(index, 1)}
              onRemove={() =>
                props.onChange(
                  props.items.filter((_, current) => current !== index)
                )
              }
            />
          ))}
        </div>
      )}
      {!hasFreeGroup && props.items.length > 0 ? (
        <p className='text-muted-foreground text-xs'>
          {t('No more selectable groups are available.')}
        </p>
      ) : null}
    </div>
  )
}
