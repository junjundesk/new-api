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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
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
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'

import {
  MAX_MODEL_MAPPINGS,
  REASONING_EFFORT_PRESETS,
  createMappingRowId,
  getModelMappingIssueMessage,
  getModelMappingIssues,
  type ModelMappingIssue,
} from '../lib/model-mapping'
import type { TokenModelMapping } from '../types'

type ModelMappingRowProps = {
  mapping: TokenModelMapping
  models?: string[]
  disabled?: boolean
  targetsDisabled?: boolean
  issues: ModelMappingIssue[]
  onChange: (mapping: TokenModelMapping) => void
  onRemove: () => void
}

function ModelMappingRow(props: ModelMappingRowProps) {
  const { t } = useTranslation()
  const id = useId()
  const effort = props.mapping.reasoning_effort ?? ''
  const [customEffort, setCustomEffort] = useState(
    !REASONING_EFFORT_PRESETS.has(effort)
  )
  // Remembers the value this row emitted so the sync effect below only reacts
  // to external changes (preset apply, form reset), not to the row's own edits.
  const lastEmitted = useRef<TokenModelMapping | null>(null)

  useEffect(() => {
    const emitted = lastEmitted.current
    if (
      emitted &&
      emitted.source_model === props.mapping.source_model &&
      emitted.target_model === props.mapping.target_model &&
      emitted.reasoning_effort === props.mapping.reasoning_effort
    ) {
      lastEmitted.current = null
      return
    }
    setCustomEffort(
      !REASONING_EFFORT_PRESETS.has(props.mapping.reasoning_effort ?? '')
    )
  }, [
    props.mapping.source_model,
    props.mapping.target_model,
    props.mapping.reasoning_effort,
  ])

  const emit = (mapping: TokenModelMapping) => {
    lastEmitted.current = mapping
    props.onChange(mapping)
  }

  const sourceIssue = props.issues.find((i) => i.field === 'source_model')
  const targetIssue = props.issues.find((i) => i.field === 'target_model')
  const effortIssue = props.issues.find((i) => i.field === 'reasoning_effort')

  const models = props.models ?? []
  const target = props.mapping.target_model
  // Keep a saved-but-unavailable target visible so users can see and fix it.
  const targetItems =
    target && !models.includes(target) ? [target, ...models] : [...models]

  const effortOptions = [
    { value: 'inherit', label: t('Default (follow client)') },
    { value: 'none', label: t('None') },
    { value: 'minimal', label: t('Minimal') },
    { value: 'low', label: t('Low') },
    { value: 'medium', label: t('Medium') },
    { value: 'high', label: t('High') },
    { value: 'xhigh', label: t('Extra high') },
    { value: 'max', label: t('Maximum') },
    { value: 'custom', label: t('Custom') },
  ]

  return (
    <FieldGroup className='gap-3'>
      <div className='grid min-w-0 gap-3 sm:grid-cols-2'>
        <Field data-invalid={!!sourceIssue}>
          <FieldLabel htmlFor={`${id}-source`}>{t('Source model')}</FieldLabel>
          <Input
            id={`${id}-source`}
            value={props.mapping.source_model}
            disabled={props.disabled}
            aria-invalid={!!sourceIssue}
            aria-describedby={sourceIssue ? `${id}-source-error` : undefined}
            placeholder={t('e.g. gpt-6-luna')}
            onChange={(e) =>
              emit({ ...props.mapping, source_model: e.target.value })
            }
          />
          {sourceIssue && (
            <FieldError id={`${id}-source-error`}>
              {getModelMappingIssueMessage(sourceIssue, t)}
            </FieldError>
          )}
        </Field>
        <Field data-invalid={!!targetIssue}>
          <FieldLabel htmlFor={`${id}-target`}>{t('Target model')}</FieldLabel>
          <Combobox
            items={targetItems}
            value={target || null}
            onValueChange={(value: string | null) =>
              emit({ ...props.mapping, target_model: value ?? '' })
            }
          >
            <ComboboxInput
              id={`${id}-target`}
              className='w-full min-w-0'
              title={target}
              disabled={props.disabled || props.targetsDisabled}
              aria-invalid={!!targetIssue}
              aria-describedby={targetIssue ? `${id}-target-error` : undefined}
              placeholder={t('Search available models')}
            />
            <ComboboxContent>
              <ComboboxEmpty>{t('No available model found')}</ComboboxEmpty>
              <ComboboxList>
                <ComboboxCollection>
                  {(item: string) => (
                    <ComboboxItem
                      key={item}
                      value={item}
                      disabled={!models.includes(item)}
                    >
                      <span className='min-w-0 break-all' title={item}>
                        {item}
                      </span>
                    </ComboboxItem>
                  )}
                </ComboboxCollection>
              </ComboboxList>
            </ComboboxContent>
          </Combobox>
          {targetIssue && (
            <FieldError id={`${id}-target-error`}>
              {getModelMappingIssueMessage(targetIssue, t)}
            </FieldError>
          )}
        </Field>
      </div>
      <div className='flex min-w-0 items-end gap-2'>
        <Field className='min-w-0 flex-1'>
          <FieldLabel htmlFor={`${id}-effort`}>
            {t('Reasoning effort')}
          </FieldLabel>
          <Select
            items={effortOptions}
            value={customEffort ? 'custom' : effort || 'inherit'}
            disabled={props.disabled}
            onValueChange={(value: string | null) => {
              if (value === 'custom') {
                setCustomEffort(true)
                return
              }
              setCustomEffort(false)
              emit({
                ...props.mapping,
                reasoning_effort: value === 'inherit' ? '' : (value ?? ''),
              })
            }}
          >
            <SelectTrigger id={`${id}-effort`} className='w-full min-w-0'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {effortOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          disabled={props.disabled}
          aria-label={t('Remove mapping')}
          onClick={props.onRemove}
        >
          <Trash2 data-icon='inline-start' />
        </Button>
      </div>
      {customEffort && (
        <Field data-invalid={!!effortIssue}>
          <FieldLabel htmlFor={`${id}-custom-effort`}>
            {t('Custom reasoning effort')}
          </FieldLabel>
          <Input
            id={`${id}-custom-effort`}
            value={effort}
            disabled={props.disabled}
            aria-invalid={!!effortIssue}
            aria-describedby={effortIssue ? `${id}-effort-error` : undefined}
            placeholder={t('Enter an effort supported by the target model')}
            onChange={(e) =>
              emit({ ...props.mapping, reasoning_effort: e.target.value })
            }
          />
          {effortIssue && (
            <FieldError id={`${id}-effort-error`}>
              {getModelMappingIssueMessage(effortIssue, t)}
            </FieldError>
          )}
        </Field>
      )}
    </FieldGroup>
  )
}

export type ModelMappingEditorProps = {
  value: TokenModelMapping[]
  onChange: (value: TokenModelMapping[]) => void
  models?: string[]
  disabled?: boolean
  targetsDisabled?: boolean
  showValidation?: boolean
}

export function ModelMappingEditor(props: ModelMappingEditorProps) {
  const { t } = useTranslation()
  const [rowIds, setRowIds] = useState(() =>
    props.value.map(() => createMappingRowId())
  )

  if (rowIds.length !== props.value.length) {
    setRowIds(props.value.map((_, i) => rowIds[i] ?? createMappingRowId()))
    return null
  }

  // Required-field errors only appear after the first submit attempt.
  const issues = getModelMappingIssues(props.value, props.models).filter(
    (issue) =>
      props.showValidation ||
      (issue.code !== 'source-required' && issue.code !== 'target-required')
  )

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      {props.value.length > MAX_MODEL_MAPPINGS && (
        <Alert variant='destructive'>
          <AlertDescription>
            {t('Use at most 100 model mappings')}
          </AlertDescription>
        </Alert>
      )}
      {props.value.length === 0 && (
        <Empty className='border py-5'>
          <EmptyHeader>
            <EmptyTitle>{t('No model mappings')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'This key uses the model names sent by your client. Add a mapping to route a specific name to another model.'
              )}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}
      {props.value.map((mapping, index) => (
        <div key={rowIds[index]} className='flex min-w-0 flex-col gap-4'>
          {index > 0 && <Separator />}
          <ModelMappingRow
            mapping={mapping}
            models={props.models}
            disabled={props.disabled}
            targetsDisabled={props.targetsDisabled}
            issues={issues.filter((issue) => issue.index === index)}
            onChange={(next) =>
              props.onChange(
                props.value.map((item, i) => (i === index ? next : item))
              )
            }
            onRemove={() => {
              setRowIds(rowIds.filter((_, i) => i !== index))
              props.onChange(props.value.filter((_, i) => i !== index))
            }}
          />
        </div>
      ))}
      {props.value.length > 0 && (
        <FieldDescription>
          {t(
            'An empty effort keeps the client value. The target model must support any override you enter.'
          )}
        </FieldDescription>
      )}
      <Button
        type='button'
        variant='outline'
        size='sm'
        className='self-start'
        disabled={
          props.disabled || props.value.length >= MAX_MODEL_MAPPINGS
        }
        onClick={() =>
          props.onChange([
            ...props.value,
            { source_model: '', target_model: '' },
          ])
        }
      >
        <Plus data-icon='inline-start' />
        {t('Add mapping')}
      </Button>
    </div>
  )
}
