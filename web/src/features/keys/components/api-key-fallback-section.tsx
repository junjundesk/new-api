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
import { ChevronDown, ChevronUp, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Combobox,
  ComboboxCollection,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'

import { getFallbackModelOptions } from '../api'
import {
  MAX_FALLBACK_MODELS,
  createFallbackModel,
  getFallbackIssueMessage,
  getFallbackIssues,
  type FallbackIssue,
} from '../lib/fallback'
import type { TokenFallbackModel } from '../types'
import type { ApiKeyGroupOption } from './api-key-group-combobox'

type FallbackRowProps = {
  value: TokenFallbackModel
  index: number
  total: number
  groupOptions: ApiKeyGroupOption[]
  sourceModels: string[]
  disabled?: boolean
  issues: FallbackIssue[]
  onChange: (value: TokenFallbackModel) => void
  onMove: (index: number, direction: 'up' | 'down') => void
  onRemove: () => void
}

function FallbackRow(props: FallbackRowProps) {
  const { t } = useTranslation()
  const group = props.value.group.trim()
  const {
    data,
    isFetching,
    isError,
    refetch,
  } = useQuery({
    queryKey: ['fallback-models', group],
    queryFn: () => getFallbackModelOptions(group),
    enabled: !!group,
    staleTime: 60_000,
  })
  const models = data?.models ?? []
  const savedModel = props.value.model.trim()
  const modelItems =
    savedModel && !models.includes(savedModel) ? [savedModel, ...models] : models
  const sourceValue = props.value.source_model?.trim() ?? ''
  const sourceItems =
    sourceValue && !props.sourceModels.includes(sourceValue)
      ? [sourceValue, ...props.sourceModels]
      : props.sourceModels
  const groupItem = props.groupOptions.find((option) => option.value === group)
  const groupLabel = group ? (groupItem?.label ?? group) : ''

  const issue = (field: FallbackIssue['field']) =>
    props.issues.find((item) => item.field === field)

  const groupIssue = issue('group')
  const modelIssue = issue('model')

  return (
    <div className='flex min-w-0 flex-col gap-2 rounded-lg border p-3'>
      <div className='grid min-w-0 gap-2 sm:grid-cols-[minmax(0,0.9fr)_minmax(0,1fr)_minmax(0,1.2fr)_auto]'>
        <Field data-invalid={!!issue('source_model')} className='min-w-0'>
          <FieldLabel className='text-xs'>
            {t('Source model (optional)')}
          </FieldLabel>
          <Combobox
            items={sourceItems}
            value={sourceValue || null}
            onValueChange={(value: string | null) =>
              props.onChange({
                ...props.value,
                source_model: value ?? '',
              })
            }
          >
            <ComboboxInput
              className='w-full min-w-0'
              disabled={props.disabled}
              placeholder={t('All models')}
              aria-label={t('Source model (optional)')}
            />
            <ComboboxContent>
              <ComboboxEmpty>{t('No available model found')}</ComboboxEmpty>
              <ComboboxList>
                <ComboboxCollection>
                  {(item: string) => (
                    <ComboboxItem key={item} value={item}>
                      <span className='min-w-0 break-all'>{item}</span>
                    </ComboboxItem>
                  )}
                </ComboboxCollection>
              </ComboboxList>
            </ComboboxContent>
          </Combobox>
        </Field>

        <Field data-invalid={!!groupIssue} className='min-w-0'>
          <FieldLabel className='text-xs'>{t('Fallback group')}</FieldLabel>
          <Select
            items={props.groupOptions.map((option) => ({
              value: option.value,
              label: option.ratio != null
                ? `${option.label} · ${option.ratio}x`
                : option.label,
            }))}
            value={group || null}
            disabled={props.disabled}
            onValueChange={(value: string | null) =>
              props.onChange({
                ...props.value,
                group: value ?? '',
                model: '',
              })
            }
          >
            <SelectTrigger
              className='w-full min-w-0'
              aria-invalid={!!groupIssue}
            >
              <SelectValue placeholder={t('Select a group')}>
                {groupLabel}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {props.groupOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    <span className='flex min-w-0 items-center gap-2'>
                      <span className='truncate'>{option.label}</span>
                      {option.ratio != null && (
                        <span className='text-muted-foreground text-xs'>
                          {option.ratio}x
                        </span>
                      )}
                    </span>
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>

        <Field data-invalid={!!modelIssue} className='min-w-0'>
          <FieldLabel className='text-xs'>{t('Fallback model')}</FieldLabel>
          <Combobox
            items={modelItems}
            value={savedModel || null}
            onValueChange={(value: string | null) =>
              props.onChange({ ...props.value, model: value ?? '' })
            }
          >
            <ComboboxInput
              className='w-full min-w-0'
              title={savedModel}
              disabled={props.disabled || !group || isFetching}
              placeholder={
                group ? t('Search available models') : t('Select a group first')
              }
              aria-label={t('Fallback model')}
              aria-invalid={!!modelIssue}
            />
            <ComboboxContent>
              <ComboboxEmpty>
                {isError ? t('Could not load models') : t('No available model found')}
              </ComboboxEmpty>
              {isError && (
                <div className='p-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => void refetch()}
                  >
                    {t('Retry')}
                  </Button>
                </div>
              )}
              {isFetching && !data && (
                <div className='p-2'>
                  <Skeleton className='h-5 w-full' />
                </div>
              )}
              <ComboboxList>
                <ComboboxCollection>
                  {(item: string) => (
                    <ComboboxItem key={item} value={item}>
                      <span className='min-w-0 break-all'>{item}</span>
                    </ComboboxItem>
                  )}
                </ComboboxCollection>
              </ComboboxList>
            </ComboboxContent>
          </Combobox>
        </Field>

        <div className='flex items-end justify-end gap-1'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            disabled={props.disabled || props.index === 0}
            aria-label={t('Move fallback up')}
            onClick={() => props.onMove(props.index, 'up')}
          >
            <ChevronUp />
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            disabled={props.disabled || props.index === props.total - 1}
            aria-label={t('Move fallback down')}
            onClick={() => props.onMove(props.index, 'down')}
          >
            <ChevronDown />
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            disabled={props.disabled}
            aria-label={t('Remove fallback')}
            onClick={props.onRemove}
          >
            <Trash2 />
          </Button>
        </div>
      </div>

      {groupIssue && (
        <FieldError className='text-xs'>
          {getFallbackIssueMessage(groupIssue, t)}
        </FieldError>
      )}
      {modelIssue && (
        <FieldError className='text-xs'>
          {getFallbackIssueMessage(modelIssue, t)}
        </FieldError>
      )}
      {!group && (
        <FieldDescription className='text-xs'>
          {t('Choose a group, then select any model available in that group.')}
        </FieldDescription>
      )}
    </div>
  )
}

export type ApiKeyFallbackSectionProps = {
  enabled: boolean
  onEnabledChange: (enabled: boolean) => void
  value: TokenFallbackModel[]
  onChange: (value: TokenFallbackModel[]) => void
  groupOptions: ApiKeyGroupOption[]
  sourceModels: string[]
  disabled?: boolean
  showValidation?: boolean
}

export function ApiKeyFallbackSection(props: ApiKeyFallbackSectionProps) {
  const { t } = useTranslation()
  const issues = getFallbackIssues(props.value, props.showValidation ?? false)

  const updateAt = (index: number, next: TokenFallbackModel) => {
    props.onChange(props.value.map((item, i) => (i === index ? next : item)))
  }

  const move = (index: number, direction: 'up' | 'down') => {
    const target = direction === 'up' ? index - 1 : index + 1
    if (target < 0 || target >= props.value.length) return
    const next = [...props.value]
    ;[next[index], next[target]] = [next[target], next[index]]
    props.onChange(next)
  }

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <p className='text-muted-foreground text-sm'>
        {t(
          'When the original model is unavailable, try these models in order. Billing uses the model that actually serves the request.'
        )}
      </p>

      <div className='flex items-center justify-between gap-3 rounded-lg border bg-muted/20 px-3 py-2.5'>
        <div className='min-w-0'>
          <p className='text-sm font-medium'>{t('Enable fallback')}</p>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t('Keep the list when disabled; failed requests will not switch.')}
          </p>
        </div>
        <Switch
          aria-label={t('Enable fallback')}
          checked={props.enabled}
          disabled={props.disabled}
          onCheckedChange={props.onEnabledChange}
        />
      </div>

      {props.enabled && (
        <>
          {props.value.length >= MAX_FALLBACK_MODELS && (
            <Alert>
              <AlertDescription>
                {t('Use at most {{max}} fallback models', {
                  max: MAX_FALLBACK_MODELS,
                })}
              </AlertDescription>
            </Alert>
          )}

          {props.value.map((fallback, index) => (
            <FallbackRow
              key={fallback.id}
              value={fallback}
              index={index}
              total={props.value.length}
              groupOptions={props.groupOptions}
              sourceModels={props.sourceModels}
              disabled={props.disabled}
              issues={issues.filter((issue) => issue.index === index)}
              onChange={(next) => updateAt(index, next)}
              onMove={move}
              onRemove={() =>
                props.onChange(props.value.filter((_, i) => i !== index))
              }
            />
          ))}

          <div className='flex items-center justify-between gap-2'>
            <p className='text-muted-foreground text-xs'>
              {t('{{count}} / {{max}} fallback models', {
                count: props.value.length,
                max: MAX_FALLBACK_MODELS,
              })}
            </p>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.disabled || props.value.length >= MAX_FALLBACK_MODELS}
              onClick={() =>
                props.onChange([
                  ...props.value,
                  createFallbackModel(),
                ])
              }
            >
              <Plus data-icon='inline-start' />
              {t('Add fallback model')}
            </Button>
          </div>

          <FieldDescription className='text-xs'>
            {t(
              'Triggered only when the original request fails with a non-2xx response and produces no output tokens. A stream that already started is never switched.'
            )}
          </FieldDescription>
        </>
      )}
    </div>
  )
}
