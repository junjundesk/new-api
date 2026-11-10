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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { Lock } from 'lucide-react'
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
} from 'react'
import { useForm, useWatch, type FieldPath } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { cn } from '@/lib/utils'

import {
  createChannelChain,
  getGroupModelNames,
  updateChannelChain,
} from '../api'
import {
  buildChannelChainInput,
  DEFAULT_USER_CHANNEL_CHAIN_LIMITS,
  getChainFormValues,
  getDefaultUserChannelChainFormValues,
  getUserChannelChainFormSchema,
  type UserChannelChainFormValues,
  type UserChannelChainLimits,
} from '../lib/user-channel-chain'
import type { ChannelChain } from '../types'
import type { ApiKeyGroupOption } from './api-key-group-combobox'
import { ChannelChainGroupsEditor } from './channel-chain-groups-editor'
import { ChannelChainModelsEditor } from './channel-chain-models-editor'

export const CHANNEL_CHAIN_FORM_ID = 'channel-chain-form'

export type ChannelChainEditorHandle = {
  submit: () => void
}

type ChannelChainEditorProps = {
  mode: 'create' | 'edit'
  chain?: ChannelChain
  open: boolean
  groupOptions: ApiKeyGroupOption[]
  // False while the selectable groups are still loading or failed to load:
  // submission must not mistake an empty option list for stale selections.
  groupOptionsReady?: boolean
  limits?: UserChannelChainLimits
  onSaved: () => void
  onCancel: () => void
  onSubmittingChange: (submitting: boolean) => void
}

type ChannelChainTypeOption = {
  value: 'groups' | 'models'
  label: string
  description: string
}

export const ChannelChainEditor = forwardRef<
  ChannelChainEditorHandle,
  ChannelChainEditorProps
>(function ChannelChainEditor(props, ref) {
  const { t } = useTranslation()
  const limits = props.limits ?? DEFAULT_USER_CHANNEL_CHAIN_LIMITS
  const schema = useMemo(
    () => getUserChannelChainFormSchema(t, limits),
    [t, limits]
  )
  const form = useForm<UserChannelChainFormValues>({
    resolver: zodResolver(schema),
    mode: 'onSubmit',
    defaultValues: props.chain
      ? getChainFormValues(props.chain)
      : getDefaultUserChannelChainFormValues(),
  })
  const type = useWatch({ control: form.control, name: 'type' })
  const groups = useWatch({ control: form.control, name: 'groups' }) ?? []
  const items = useWatch({ control: form.control, name: 'items' }) ?? []
  const typeLocked = props.mode === 'edit'
  const onSubmittingChange = props.onSubmittingChange
  const groupOptionsReady = props.groupOptionsReady ?? true
  const queryClient = useQueryClient()

  useEffect(() => {
    onSubmittingChange(form.formState.isSubmitting)
  }, [form.formState.isSubmitting, onSubmittingChange])

  // A saved chain unmounts this editor while isSubmitting is still true; tell
  // the dialog the request finished so a new edit is not stuck saving.
  useEffect(() => {
    return () => onSubmittingChange(false)
  }, [onSubmittingChange])

  const onSubmit = async (values: UserChannelChainFormValues) => {
    form.clearErrors()

    if (!groupOptionsReady) {
      form.setError(
        values.type === 'groups'
          ? 'groups'
          : ('items.root' as FieldPath<UserChannelChainFormValues>),
        {
          type: 'manual',
          message: t(
            'Could not verify the selected options. Check your connection and try again.'
          ),
        }
      )
      toast.error(t('Please review the highlighted fields.'))
      return
    }

    const allowedGroups = new Set(
      props.groupOptions.map((option) => option.value)
    )
    const staleGroupMessage = t(
      'This group is no longer available for chains. Review the chain before saving.'
    )

    if (values.type === 'groups') {
      if (values.groups.some((group) => !allowedGroups.has(group))) {
        form.setError('groups', {
          type: 'manual',
          message: staleGroupMessage,
        })
        toast.error(t('Please review the highlighted fields.'))
        return
      }
    } else {
      let staleSelection = false
      for (let index = 0; index < values.items.length; index += 1) {
        const item = values.items[index]
        const group = item.group.trim()
        if (!allowedGroups.has(group)) {
          form.setError(
            `items.${index}.group` as FieldPath<UserChannelChainFormValues>,
            { type: 'manual', message: staleGroupMessage }
          )
          staleSelection = true
          continue
        }

        let availableModels: string[]
        try {
          availableModels = await queryClient.fetchQuery({
            queryKey: ['group-models', group],
            queryFn: () => getGroupModelNames(group),
            staleTime: 0,
          })
        } catch {
          form.setError('items.root' as FieldPath<UserChannelChainFormValues>, {
            type: 'manual',
            message: t(
              'Could not verify the selected options. Check your connection and try again.'
            ),
          })
          toast.error(t('Please review the highlighted fields.'))
          return
        }

        const available = new Set(availableModels)
        if (item.models.some((model) => !available.has(model.trim()))) {
          form.setError(
            `items.${index}.models` as FieldPath<UserChannelChainFormValues>,
            {
              type: 'manual',
              message: t(
                'Some models are no longer available. Remove or replace them before saving.'
              ),
            }
          )
          staleSelection = true
        }
      }

      if (staleSelection) {
        toast.error(t('Please review the highlighted fields.'))
        return
      }
    }

    try {
      const payload = buildChannelChainInput(values)
      const result =
        props.mode === 'edit' && props.chain
          ? await updateChannelChain(props.chain.id, payload)
          : await createChannelChain(payload)
      if (result.success) {
        toast.success(t('Group chain saved'))
        props.onSaved()
        return
      }
      toast.error(result.message || t('An unexpected error occurred'))
    } catch {
      toast.error(t('An unexpected error occurred'))
    }
  }

  const submit = () => {
    void form.handleSubmit(onSubmit, () => {
      toast.error(t('Please review the highlighted fields.'))
    })()
  }
  const submitRef = useRef(submit)
  useEffect(() => {
    submitRef.current = submit
  })
  useImperativeHandle(ref, () => ({ submit: () => submitRef.current() }), [])

  const typeOptions: ChannelChainTypeOption[] = [
    {
      value: 'groups',
      label: t('Groups'),
      description: t('Fail over across the selected groups in order.'),
    },
    {
      value: 'models',
      label: t('Models'),
      description: t('Route each requested model to its own ordered groups.'),
    },
  ]

  const setType = (next: string) => {
    if (typeLocked) return
    if (next !== 'groups' && next !== 'models') return
    form.setValue('type', next, { shouldDirty: true, shouldValidate: false })
  }

  const itemsError = form.formState.errors.items as unknown
  // Field-array errors can contain both numeric row entries and a root error.
  const itemsFieldError = itemsError as
    | { root?: { message?: string }; message?: string }
    | undefined
  const itemsRootError =
    itemsFieldError?.root?.message ?? itemsFieldError?.message
  const rowErrors = Array.isArray(itemsError)
    ? (
        itemsError as Array<{
          group?: { message?: string }
          models?: { message?: string }
        }>
      ).map((row) => ({
        group: row?.group?.message,
        models: row?.models?.message,
      }))
    : items.map(() => ({}))

  const groupOptionsWithRatios: ApiKeyGroupOption[] = props.groupOptions

  return (
    <Form {...form}>
      <form
        id={CHANNEL_CHAIN_FORM_ID}
        className='space-y-4'
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
      >
        <FormField
          control={form.control}
          name='name'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Chain name')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('e.g., primary -> backup')}
                  maxLength={64}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <fieldset className='space-y-2'>
          <legend className='text-sm font-medium'>{t('Chain type')}</legend>
          <RadioGroup
            value={type}
            onValueChange={setType}
            disabled={typeLocked}
            aria-label={t('Chain type')}
            className='grid grid-cols-1 gap-2 sm:grid-cols-2'
          >
            {typeOptions.map((option) => {
              const checked = type === option.value
              return (
                <div
                  key={option.value}
                  className={cn(
                    'flex items-start gap-2 rounded-lg border p-3 transition-colors',
                    checked && 'border-primary bg-primary/5',
                    typeLocked ? 'opacity-80' : 'cursor-pointer'
                  )}
                  onClick={() => setType(option.value)}
                >
                  <RadioGroupItem
                    value={option.value}
                    aria-label={option.label}
                    className='mt-0.5'
                  />
                  <div className='min-w-0'>
                    <span className='block text-sm font-medium'>
                      {option.label}
                    </span>
                    <span className='text-muted-foreground block text-xs'>
                      {option.description}
                    </span>
                  </div>
                </div>
              )
            })}
          </RadioGroup>
          {typeLocked ? (
            <p className='text-muted-foreground flex items-center gap-1 text-xs'>
              <Lock aria-hidden='true' className='size-3.5' />
              {t('The chain type cannot be changed after creation.')}
            </p>
          ) : null}
        </fieldset>

        {type === 'groups' ? (
          <div className='space-y-2'>
            <p className='text-sm font-medium'>{t('Group order')}</p>
            <ChannelChainGroupsEditor
              groups={groups}
              options={groupOptionsWithRatios.map((option) => ({
                value: option.value,
                label: option.label,
                ratio: option.ratio,
              }))}
              maxGroups={limits.maxGroupsPerChain}
              disabled={form.formState.isSubmitting}
              error={form.formState.errors.groups?.message}
              onChange={(next) =>
                form.setValue('groups', next, {
                  shouldDirty: true,
                  shouldValidate: false,
                })
              }
            />
          </div>
        ) : (
          <div className='space-y-2'>
            <p className='text-sm font-medium'>{t('Model rows')}</p>
            <ChannelChainModelsEditor
              items={items}
              groupOptions={groupOptionsWithRatios}
              maxModelsPerItem={limits.maxModelsPerItem}
              maxGroupsPerChain={limits.maxGroupsPerChain}
              maxItemsBytes={limits.maxItemsBytes}
              open={props.open}
              disabled={form.formState.isSubmitting}
              rootError={itemsRootError}
              rowErrors={rowErrors}
              onChange={(next) =>
                form.setValue('items', next, {
                  shouldDirty: true,
                  shouldValidate: false,
                })
              }
            />
          </div>
        )}
      </form>
    </Form>
  )
})
