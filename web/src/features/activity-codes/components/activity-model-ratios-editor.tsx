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
import { useId } from 'react'
import { useFieldArray, useWatch, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

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

import { ACTIVITY_VALIDATION } from '../constants'
import type { ActivityCodeFormValues } from '../lib/activity-form'

type Props = {
  form: UseFormReturn<ActivityCodeFormValues>
  models: string[]
  disabled?: boolean
}

// 「指定模型」类型的模型倍率编辑器：每个模型各配一个倍率，命中即无视原分组定价。
export function ActivityModelRatiosEditor(props: Props) {
  const { t } = useTranslation()
  const id = useId()
  const { fields, append, remove } = useFieldArray({
    control: props.form.control,
    name: 'model_ratio_overrides',
  })
  const values =
    useWatch({ control: props.form.control, name: 'model_ratio_overrides' }) ?? []
  const errors = props.form.formState.errors.model_ratio_overrides
  // 数组级错误（模型列表为空或超出上限）由 superRefine 挂在数组本身，而不是某一行。
  const listError = Array.isArray(errors)
    ? undefined
    : (errors as { message?: string } | undefined)?.message

  return (
    <FieldSet disabled={props.disabled} className='rounded-lg border p-4'>
      <FieldLegend>{t('Model ratio overrides')}</FieldLegend>
      <FieldDescription id={`${id}-help`}>
        {t(
          'Each listed model is charged at its own ratio and ignores the group pricing. Models you do not list keep their normal price.'
        )}
      </FieldDescription>
      <FieldGroup className='gap-3'>
        {fields.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('No model ratio override yet.')}
          </p>
        )}
        {fields.map((field, index) => {
          const modelError = errors?.[index]?.model?.message
          const ratioError = errors?.[index]?.ratio?.message
          const model = values[index]?.model ?? ''
          const options = [...new Set([...props.models, model].filter(Boolean))]
          return (
            <div
              key={field.id}
              className='grid min-w-0 gap-2 sm:grid-cols-[minmax(0,1fr)_10rem_auto] sm:items-start'
            >
              <Field data-invalid={Boolean(modelError)}>
                <FieldLabel htmlFor={`${id}-${field.id}-model`}>
                  {t('Model')}
                </FieldLabel>
                {/* 旧版 options 形态支持 allowCustomValue：允许填写尚未上架的模型名。 */}
                <Combobox
                  id={`${id}-${field.id}-model`}
                  options={options.map((name) => ({ value: name, label: name }))}
                  value={model}
                  allowCustomValue
                  placeholder={t('Select a model')}
                  className='w-full min-w-0'
                  onValueChange={(value) =>
                    props.form.setValue(
                      `model_ratio_overrides.${index}.model`,
                      value ?? '',
                      { shouldDirty: true, shouldValidate: true }
                    )
                  }
                />
                {modelError && (
                  <p role='alert' className='text-destructive text-sm'>
                    {t(modelError)}
                  </p>
                )}
              </Field>
              <Field data-invalid={Boolean(ratioError)}>
                <FieldLabel htmlFor={`${id}-${field.id}-ratio`}>
                  {t('Model ratio')}
                </FieldLabel>
                <Input
                  id={`${id}-${field.id}-ratio`}
                  type='text'
                  inputMode='decimal'
                  disabled={props.disabled}
                  aria-invalid={Boolean(ratioError)}
                  aria-describedby={`${id}-help`}
                  {...props.form.register(`model_ratio_overrides.${index}.ratio`)}
                />
                {ratioError && (
                  <p role='alert' className='text-destructive text-sm'>
                    {t(ratioError)}
                  </p>
                )}
                {values[index]?.ratio.trim() !== '' &&
                  Number(values[index]?.ratio) === 0 && (
                    <FieldDescription>
                      {t('0 means this model is completely free.')}
                    </FieldDescription>
                  )}
              </Field>
              <Button
                type='button'
                variant='outline'
                disabled={props.disabled}
                className='sm:mt-6'
                aria-label={t('Remove model ratio override {{index}}', {
                  index: index + 1,
                })}
                onClick={() => remove(index)}
              >
                {t('Delete')}
              </Button>
            </div>
          )
        })}
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
            fields.length >= ACTIVITY_VALIDATION.MODEL_RATIO_MAX_ENTRIES
          }
          onClick={() => append({ model: '', ratio: '' })}
        >
          {t('Add model ratio override')}
        </Button>
        <span className='text-muted-foreground text-xs'>
          {t('Allowed range: 0–{{max}}', {
            max: ACTIVITY_VALIDATION.RATIO_MAX,
          })}
        </span>
      </div>
    </FieldSet>
  )
}
