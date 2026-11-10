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
import { ArrowDown, ArrowUp, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'

import { GroupRatioBadge, type GroupRatio } from './auto-group-visuals'

type ChannelChainGroupsEditorProps = {
  groups: string[]
  options: { label: string; value: string; ratio?: GroupRatio }[]
  maxGroups: number
  disabled?: boolean
  error?: string
  onChange: (groups: string[]) => void
}

export function ChannelChainGroupsEditor(props: ChannelChainGroupsEditorProps) {
  const { t } = useTranslation()
  const removeGroup = (index: number) => {
    props.onChange(props.groups.filter((_, current) => current !== index))
  }
  const moveGroup = (index: number, offset: number) => {
    const next = [...props.groups]
    const target = index + offset
    if (target < 0 || target >= next.length) return
    ;[next[index], next[target]] = [next[target], next[index]]
    props.onChange(next)
  }

  return (
    <div className='space-y-2'>
      <MultiSelect
        id='channel-chain-groups'
        options={props.options}
        selected={[]}
        onChange={(next) => {
          const added = next.filter((group) => !props.groups.includes(group))
          if (added.length > 0) props.onChange([...props.groups, ...added])
        }}
        placeholder={t('Add a group to the chain')}
        disabled={props.disabled || props.groups.length >= props.maxGroups}
        maxVisibleChips={3}
      />
      <p className='text-muted-foreground text-xs'>
        {t('Groups: {{used}}/{{max}}', {
          used: props.groups.length,
          max: props.maxGroups,
        })}
      </p>
      {props.error ? (
        <p className='text-destructive text-xs' role='alert'>
          {props.error}
        </p>
      ) : null}
      {props.groups.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t('Select at least one group for the chain')}
        </p>
      ) : (
        <ul className='space-y-1.5'>
          {props.groups.map((group, index) => (
            <li
              key={group}
              className='flex min-w-0 items-center gap-2 rounded-md border px-2 py-1.5'
            >
              <span className='text-muted-foreground w-5 shrink-0 text-xs tabular-nums'>
                {index + 1}.
              </span>
              <span className='min-w-0 flex-1 truncate text-sm' title={group}>
                {group}
              </span>
              <GroupRatioBadge
                ratio={props.options.find((o) => o.value === group)?.ratio}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-7'
                aria-label={t('Move up')}
                disabled={props.disabled || index === 0}
                onClick={() => moveGroup(index, -1)}
              >
                <ArrowUp aria-hidden='true' className='size-4' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-7'
                aria-label={t('Move down')}
                disabled={props.disabled || index === props.groups.length - 1}
                onClick={() => moveGroup(index, 1)}
              >
                <ArrowDown aria-hidden='true' className='size-4' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='text-destructive size-7'
                aria-label={t('Remove')}
                disabled={props.disabled}
                onClick={() => removeGroup(index)}
              >
                <Trash2 aria-hidden='true' className='size-4' />
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
