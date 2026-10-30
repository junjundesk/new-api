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
import { Trash2 } from 'lucide-react'
import { useId } from 'react'
import {
  type FieldErrors,
  useFieldArray,
  useWatch,
  type UseFormReturn,
} from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
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
import { formatTokenAmount, parseTokenAmountInput } from '@/lib/token-amount'

import { getGroupModels } from '../api'
import {
  ACTIVITY_BENEFIT_SCOPE,
  ACTIVITY_BENEFIT_TOKEN_MODE,
  ACTIVITY_VALIDATION,
  ERROR_MESSAGES,
  getActivityBenefitScopeOptions,
  getActivityBenefitTokenModeOptions,
} from '../constants'
import {
  createActivityBenefitRow,
  type ActivityBenefitRow,
} from '../lib/activity-benefit'
import type { ActivityCodeFormValues } from '../lib/activity-form'

type Props = {
  form: UseFormReturn<ActivityCodeFormValues>
  groupNames: string[]
  disabled?: boolean
}

type RowEditorProps = {
  form: UseFormReturn<ActivityCodeFormValues>
  groupNames: string[]
  index: number
  row: ActivityBenefitRow
  errors?: FieldErrors<ActivityBenefitRow>
  disabled?: boolean
  onRemove: () => void
}

// 「免费权益」类型的编辑器：每行配置分组、整组/单模型、RPM 与 Token 总量。
// Token 总量输入是文本，支持 K/M/B 与千/万/亿，提交前解析为整数。
export function ActivityBenefitsEditor(props: Props) {
  const { t } = useTranslation()
  const { fields, append, remove } = useFieldArray({
    control: props.form.control,
    name: 'benefits',
  })
  const rows =
    useWatch({ control: props.form.control, name: 'benefits' }) ?? []
  const fieldErrors = props.form.formState.errors.benefits
  const listError = Array.isArray(fieldErrors)
    ? undefined
    : (fieldErrors as { message?: string } | undefined)?.message
  const rowErrors = Array.isArray(fieldErrors)
    ? (fieldErrors as unknown as Array<FieldErrors<ActivityBenefitRow> | undefined>)
    : undefined

  return (
    <FieldSet disabled={props.disabled} className='rounded-lg border p-4'>
      <FieldLegend>{t('Free benefits')}</FieldLegend>
      <FieldDescription>
        {t(
          'Each benefit makes a whole group or a single model free. A token package caps the free usage, and usage beyond it falls back to normal pricing.'
        )}
      </FieldDescription>
      <FieldGroup className='gap-3'>
        {fields.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('No benefits yet.')}
          </p>
        )}
        {fields.map((field, index) => (
          <ActivityBenefitRowEditor
            key={field.id}
            form={props.form}
            groupNames={props.groupNames}
            index={index}
            row={rows[index] ?? createActivityBenefitRow()}
            errors={rowErrors?.[index]}
            disabled={props.disabled}
            onRemove={() => remove(index)}
          />
        ))}
      </FieldGroup>
      {listError && (
        <p role='alert' className='text-destructive text-sm'>
          {t(listError)}
        </p>
      )}
      <div className='flex flex-wrap items-center gap-3'>
        <Button
          type='button'
          variant='outline'
          disabled={
            props.disabled ||
            fields.length >= ACTIVITY_VALIDATION.BENEFIT_MAX_ENTRIES
          }
          onClick={() => append(createActivityBenefitRow())}
        >
          {t('Add benefit')}
        </Button>
        <span className='text-muted-foreground text-xs'>
          {t('At most {{count}} benefits', {
            count: ACTIVITY_VALIDATION.BENEFIT_MAX_ENTRIES,
          })}
        </span>
      </div>
    </FieldSet>
  )
}

function ActivityBenefitRowEditor(props: RowEditorProps) {
  const { t, i18n } = useTranslation()
  const id = useId()
  const isModelScope = props.row.scope === ACTIVITY_BENEFIT_SCOPE.MODEL
  const isTokenPackage =
    props.row.token_mode === ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE

  const { data: groupModels = [] } = useQuery({
    queryKey: ['activity-group-models', props.row.group_name],
    enabled: props.row.group_name !== '' && isModelScope,
    queryFn: async () => {
      const result = await getGroupModels(props.row.group_name)
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.GROUP_MODELS_FAILED))
        return []
      }
      return (result.data ?? []).map((member) => member.model_name)
    },
  })

  const parsedTokenTotal = parseTokenAmountInput(props.row.token_total_input)
  const tokenPreview =
    isTokenPackage && parsedTokenTotal.ok
      ? formatTokenAmount(parsedTokenTotal.value, i18n.language)
      : null
  const tokenInputInvalid =
    isTokenPackage &&
    props.row.token_total_input.trim() !== '' &&
    !parsedTokenTotal.ok

  // 整行更新：切换分组或范围时清空模型名，避免残留旧分组的模型。
  const updateRow = (patch: Partial<ActivityBenefitRow>) => {
    props.form.setValue(
      `benefits.${props.index}`,
      { ...props.row, ...patch },
      { shouldDirty: true, shouldValidate: true }
    )
  }

  // 允许展示已保存但当前分组列表里已不存在的分组。
  const groupOptions = [
    ...new Set([...props.groupNames, props.row.group_name].filter(Boolean)),
  ]
  const modelOptions = [
    ...new Set([...groupModels, props.row.model_name].filter(Boolean)),
  ]
  const groupError = props.errors?.group_name?.message
  const modelError = props.errors?.model_name?.message
  const rpmError = props.errors?.rpm_limit?.message
  const tokenError = props.errors?.token_total_input?.message

  return (
    <div className='grid min-w-0 gap-3 rounded-md border p-3'>
      <div className='grid gap-3 sm:grid-cols-2'>
        <Field data-invalid={Boolean(groupError)}>
          <FieldLabel htmlFor={`${id}-group`}>
            {t('Group for benefit {{index}}', { index: props.index + 1 })}
          </FieldLabel>
          <Select
            items={groupOptions.map((group) => ({
              value: group,
              label: group,
            }))}
            value={props.row.group_name}
            onValueChange={(value) =>
              updateRow({ group_name: value ?? '', model_name: '' })
            }
          >
            <SelectTrigger
              id={`${id}-group`}
              className='w-full'
              aria-invalid={Boolean(groupError)}
            >
              <SelectValue placeholder={t('Select a group')} />
            </SelectTrigger>
            <SelectContent
              alignItemWithTrigger={false}
              className='max-h-[min(22rem,var(--available-height))] w-max min-w-(--anchor-width) max-w-[min(32rem,var(--available-width))]'
            >
              <SelectGroup>
                {groupOptions.map((group) => (
                  <SelectItem key={group} value={group}>
                    {group}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          {groupError && (
            <p role='alert' className='text-destructive text-sm'>
              {t(groupError)}
            </p>
          )}
        </Field>

        <Field>
          <FieldLabel htmlFor={`${id}-scope`}>
            {t('Scope for benefit {{index}}', { index: props.index + 1 })}
          </FieldLabel>
          <Select
            items={getActivityBenefitScopeOptions(t)}
            value={props.row.scope}
            onValueChange={(value) =>
              updateRow({
                scope: (value ?? '') as ActivityBenefitRow['scope'],
                model_name: '',
              })
            }
          >
            <SelectTrigger id={`${id}-scope`} className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {getActivityBenefitScopeOptions(t).map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
      </div>

      <div
        className={
          isModelScope
            ? 'grid gap-3 sm:grid-cols-2'
            : 'grid gap-3 sm:grid-cols-1'
        }
      >
        {isModelScope && (
          <Field data-invalid={Boolean(modelError)}>
            <FieldLabel htmlFor={`${id}-model`}>
              {t('Model for benefit {{index}}', { index: props.index + 1 })}
            </FieldLabel>
            <Combobox
              id={`${id}-model`}
              options={modelOptions.map((name) => ({
                value: name,
                label: name,
              }))}
              value={props.row.model_name}
              allowCustomValue
              placeholder={t('Select a model')}
              className='w-full min-w-0'
              onValueChange={(value) => updateRow({ model_name: value ?? '' })}
            />
            {modelError && (
              <p role='alert' className='text-destructive text-sm'>
                {t(modelError)}
              </p>
            )}
          </Field>
        )}

        <Field data-invalid={Boolean(rpmError)}>
          <FieldLabel htmlFor={`${id}-rpm`}>
            {t('RPM for benefit {{index}}', { index: props.index + 1 })}
          </FieldLabel>
          <Input
            id={`${id}-rpm`}
            type='number'
            min={0}
            step={1}
            value={props.row.rpm_limit}
            aria-invalid={Boolean(rpmError)}
            onChange={(event) =>
              updateRow({ rpm_limit: event.target.valueAsNumber || 0 })
            }
          />
          <FieldDescription>
            {t('Requests per minute. 0 means unlimited.')}
          </FieldDescription>
          {rpmError && (
            <p role='alert' className='text-destructive text-sm'>
              {t(rpmError)}
            </p>
          )}
        </Field>
      </div>

      <div className='grid gap-3 sm:grid-cols-2'>
        <Field>
          <FieldLabel htmlFor={`${id}-token-mode`}>
            {t('Quota mode for benefit {{index}}', { index: props.index + 1 })}
          </FieldLabel>
          <Select
            items={getActivityBenefitTokenModeOptions(t)}
            value={props.row.token_mode}
            onValueChange={(value) =>
              updateRow({
                token_mode: (value ?? '') as ActivityBenefitRow['token_mode'],
              })
            }
          >
            <SelectTrigger id={`${id}-token-mode`} className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {getActivityBenefitTokenModeOptions(t).map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>

        {isTokenPackage && (
          <Field data-invalid={Boolean(tokenError)}>
            <FieldLabel htmlFor={`${id}-token`}>
              {t('Token amount for benefit {{index}}', {
                index: props.index + 1,
              })}
            </FieldLabel>
            <Input
              id={`${id}-token`}
              type='text'
              inputMode='decimal'
              value={props.row.token_total_input}
              placeholder={t('For example: 1000000, 1M, 100万')}
              aria-invalid={Boolean(tokenError)}
              onChange={(event) =>
                updateRow({ token_total_input: event.target.value })
              }
            />
            {tokenPreview && (
              <FieldDescription aria-live='polite'>
                {t('Normalized amount: {{amount}}', { amount: tokenPreview })}
              </FieldDescription>
            )}
            {tokenInputInvalid && (
              <FieldDescription className='text-destructive'>
                {t(
                  'Enter a valid token amount, for example 1000000, 1M or 100万'
                )}
              </FieldDescription>
            )}
            {tokenError && (
              <p role='alert' className='text-destructive text-sm'>
                {t(tokenError)}
              </p>
            )}
          </Field>
        )}
      </div>

      <div className='flex justify-end'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={props.disabled}
          aria-label={t('Remove benefit {{index}}', {
            index: props.index + 1,
          })}
          onClick={props.onRemove}
        >
          <Trash2 className='h-4 w-4' />
          {t('Delete')}
        </Button>
      </div>
    </div>
  )
}
