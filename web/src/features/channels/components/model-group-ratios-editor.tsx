import { useId } from 'react'
import { useFieldArray, useWatch, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Combobox,
  ComboboxContent,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox'
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
  MAX_MODEL_GROUP_RATIO,
  type ChannelFormValues,
} from '../lib/channel-form'

type Props = {
  form: UseFormReturn<ChannelFormValues>
  models: string[]
  disabled?: boolean
}

export function ModelGroupRatiosEditor(props: Props) {
  const { t } = useTranslation()
  const id = useId()
  const { fields, append, remove } = useFieldArray({
    control: props.form.control,
    name: 'model_group_ratios',
  })
  const values =
    useWatch({ control: props.form.control, name: 'model_group_ratios' }) ?? []
  const errors = props.form.formState.errors.model_group_ratios

  return (
    <FieldSet disabled={props.disabled} className='rounded-lg border p-4'>
      <FieldLegend>{t('Model billing ratios')}</FieldLegend>
      <FieldDescription id={`${id}-help`}>
        {t(
          'Override the effective group ratio for this channel and published model, before model mapping. Blank inherits the original price; 0 is free and 1 is an explicit 1× ratio. Applies to every group on this channel.'
        )}
      </FieldDescription>
      <FieldGroup className='gap-3'>
        {fields.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('All models inherit their group pricing.')}
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
                  {t('Published model')}
                </FieldLabel>
                <Combobox
                  items={options}
                  value={model || null}
                  disabled={props.disabled}
                  onValueChange={(value) =>
                    props.form.setValue(
                      `model_group_ratios.${index}.model`,
                      value ?? '',
                      { shouldDirty: true, shouldValidate: true }
                    )
                  }
                >
                  <ComboboxInput
                    showTrigger={false}
                    id={`${id}-${field.id}-model`}
                    disabled={props.disabled}
                    aria-invalid={Boolean(modelError)}
                    aria-describedby={
                      modelError
                        ? `${id}-${field.id}-model-error`
                        : `${id}-help`
                    }
                    placeholder={t('Select a model')}
                    className='w-full min-w-0'
                  />
                  <ComboboxContent>
                    <ComboboxList>
                      {(item: string) => (
                        <ComboboxItem key={item} value={item}>
                          {item}
                        </ComboboxItem>
                      )}
                    </ComboboxList>
                  </ComboboxContent>
                </Combobox>
                {modelError && (
                  <p
                    id={`${id}-${field.id}-model-error`}
                    role='alert'
                    className='text-destructive text-sm'
                  >
                    {t(modelError)}
                  </p>
                )}
              </Field>
              <Field data-invalid={Boolean(ratioError)}>
                <FieldLabel htmlFor={`${id}-${field.id}-ratio`}>
                  {t('Billing ratio')}
                </FieldLabel>
                <Input
                  id={`${id}-${field.id}-ratio`}
                  type='text'
                  inputMode='decimal'
                  disabled={props.disabled}
                  placeholder={t('Inherit')}
                  aria-invalid={Boolean(ratioError)}
                  aria-describedby={
                    ratioError ? `${id}-${field.id}-ratio-error` : `${id}-help`
                  }
                  {...props.form.register(`model_group_ratios.${index}.ratio`)}
                />
                {ratioError && (
                  <p
                    id={`${id}-${field.id}-ratio-error`}
                    role='alert'
                    className='text-destructive text-sm'
                  >
                    {t(ratioError)}
                  </p>
                )}
                {values[index]?.ratio.trim() !== '' &&
                  Number(values[index]?.ratio) === 0 && (
                    <FieldDescription>
                      {t('Free on this channel')}
                    </FieldDescription>
                  )}
              </Field>
              <Button
                type='button'
                variant='outline'
                disabled={props.disabled}
                className='sm:mt-6'
                aria-label={t('Remove billing ratio {{index}}', {
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
      <div className='flex flex-wrap items-center gap-3'>
        <Button
          type='button'
          variant='outline'
          disabled={props.disabled || props.models.length === 0}
          onClick={() => append({ model: '', ratio: '' })}
        >
          {t('Add model billing ratio')}
        </Button>
        <span className='text-muted-foreground text-xs'>
          {t('Allowed range: 0–{{max}}', { max: MAX_MODEL_GROUP_RATIO })}
        </span>
      </div>
    </FieldSet>
  )
}
