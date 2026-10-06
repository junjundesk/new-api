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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getEnabledModels } from '@/features/channels/api'
import { ModelMappingEditor } from '@/features/keys/components/model-mapping-editor'
import {
  createMappingRowId,
  getModelMappingsSchema,
  normalizeModelMappings,
} from '@/features/keys/lib/model-mapping'
import { tokenModelMappingPresetSchema } from '@/features/keys/types'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const MAX_PRESETS = 100

type Props = {
  value: string
}

export function TokenModelMappingPresetsCard(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const queryClient = useQueryClient()

  const modelsQuery = useQuery({
    queryKey: ['enabled-models'],
    queryFn: async () => {
      const res = await getEnabledModels()
      if (!res.success || !res.data) {
        throw new Error(res.message || 'Failed to load available models')
      }
      return [...new Set(res.data)].sort()
    },
    staleTime: 0,
  })

  const parsed = useMemo(() => {
    try {
      return z
        .array(tokenModelMappingPresetSchema)
        .safeParse(JSON.parse(props.value || '[]'))
    } catch {
      return null
    }
  }, [props.value])
  const invalidConfig = !parsed?.success

  const schema = useMemo(
    () =>
      z
        .object({
          presets: z
            .array(
              tokenModelMappingPresetSchema.extend({
                id: z
                  .string()
                  .trim()
                  .min(1)
                  .max(64)
                  .regex(/^[A-Za-z0-9._-]+$/),
                name: z
                  .string()
                  .trim()
                  .min(1, t('Please enter a preset name'))
                  .refine(
                    (value) => [...value].length <= 100,
                    t('Use at most 100 characters for the preset name')
                  ),
                description: z
                  .string()
                  .optional()
                  .refine(
                    (value) => [...(value?.trim() ?? '')].length <= 500,
                    t('Use at most 500 characters for the preset description')
                  ),
                mappings: getModelMappingsSchema(
                  t,
                  modelsQuery.isSuccess ? modelsQuery.data : undefined
                ).refine(
                  (mappings) => mappings.length > 0,
                  t('Add at least one mapping to this preset')
                ),
              })
            )
            .max(MAX_PRESETS, t('Use at most 100 presets')),
        })
        .superRefine((data, ctx) => {
          const seen = new Set<string>()
          data.presets.forEach((preset, index) => {
            if (seen.has(preset.id)) {
              ctx.addIssue({
                code: 'custom',
                path: ['presets', index, 'name'],
                message: t('Preset IDs must be unique'),
              })
            }
            seen.add(preset.id)
          })
        }),
    [t, modelsQuery.isSuccess, modelsQuery.data]
  )

  type FormValues = z.infer<typeof schema>

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { presets: parsed?.success ? parsed.data : [] },
  })
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'presets',
    keyName: 'fieldKey',
  })

  useEffect(() => {
    if (parsed?.success) form.reset({ presets: parsed.data })
  }, [parsed, form])

  const modelsPending = !modelsQuery.isSuccess || modelsQuery.isFetching
  const disabled = invalidConfig || updateOption.isPending

  const onSubmit = async (values: FormValues) => {
    if (invalidConfig || updateOption.isPending) return
    if (values.presets.length > 0 && modelsPending) return
    const result = await updateOption.mutateAsync({
      key: 'TokenModelMappingPresets',
      value: JSON.stringify(
        values.presets.map((preset) => ({
          ...preset,
          name: preset.name.trim(),
          description: preset.description?.trim(),
          mappings: normalizeModelMappings(preset.mappings),
        }))
      ),
    })
    if (result.success) {
      form.reset(values)
      queryClient.invalidateQueries({
        queryKey: ['token-model-mapping-options'],
      })
    }
  }

  return (
    <SettingsSection title={t('API Key Model Mapping Presets')}>
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className='flex min-w-0 flex-col gap-4'
        >
          <p className='text-muted-foreground text-sm'>
            {t(
              'Offer named mapping presets in the API key drawer. Users copy and edit them; their saved keys do not follow future preset changes.'
            )}
          </p>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={() => {
              if (parsed?.success) form.reset({ presets: parsed.data })
            }}
            isSaving={updateOption.isPending}
            isSaveDisabled={
              invalidConfig || (fields.length > 0 && modelsPending)
            }
          />

          {invalidConfig && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Invalid mapping preset configuration')}</AlertTitle>
              <AlertDescription>
                {t(
                  'The saved configuration could not be read. Correct TokenModelMappingPresets before editing it here.'
                )}
              </AlertDescription>
            </Alert>
          )}

          <Alert>
            <AlertTitle>
              {t('Use actual client and channel model names')}
            </AlertTitle>
            <AlertDescription>
              {t(
                'Source names must exactly match client requests. Targets come from enabled channel models. A preset appears for a key only when all its targets are available in that key’s group or group chain.'
              )}
            </AlertDescription>
          </Alert>

          {modelsQuery.isFetching && (
            <div
              role='status'
              aria-label={t('Loading available models')}
              className='flex flex-col gap-2'
            >
              <Skeleton className='h-4 w-2/3' />
              <Skeleton className='h-8 w-full' />
            </div>
          )}

          {modelsQuery.isError && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Could not load available models')}</AlertTitle>
              <AlertDescription>
                <p>
                  {t('Retry to select and validate target models for this group.')}
                </p>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => {
                    void modelsQuery.refetch()
                  }}
                >
                  {t('Retry')}
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {modelsQuery.isSuccess && modelsQuery.data.length === 0 && (
            <Alert>
              <AlertTitle>{t('No enabled channel models')}</AlertTitle>
              <AlertDescription>
                {t('Enable a channel model before adding preset targets.')}
              </AlertDescription>
            </Alert>
          )}

          {fields.length === 0 && !invalidConfig && (
            <Empty className='border'>
              <EmptyHeader>
                <EmptyTitle>{t('No mapping presets')}</EmptyTitle>
                <EmptyDescription>
                  {t(
                    'Add a named preset or start with the Codex / Luna source template and choose your own target models.'
                  )}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}

          {fields.map((preset, index) => (
            <div key={preset.fieldKey} className='flex min-w-0 flex-col gap-4'>
              {index > 0 && <Separator />}
              <div className='flex items-center justify-between gap-3'>
                <p className='text-sm font-medium'>
                  {t('Preset {{number}}', { number: index + 1 })}
                </p>
                <Button
                  type='button'
                  size='sm'
                  variant='ghost'
                  disabled={disabled}
                  onClick={() => remove(index)}
                >
                  <Trash2 data-icon='inline-start' />
                  {t('Remove preset')}
                </Button>
              </div>

              <FormField
                control={form.control}
                name={`presets.${index}.name`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Preset name')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        disabled={disabled}
                        placeholder={t('e.g. Codex / Luna')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name={`presets.${index}.description`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Description')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        value={field.value ?? ''}
                        disabled={disabled}
                        rows={2}
                        placeholder={t(
                          'Explain when users should apply this preset'
                        )}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name={`presets.${index}.recommended`}
                render={({ field }) => (
                  <FormItem className='flex items-center justify-between gap-3'>
                    <FormLabel>{t('Recommend this preset')}</FormLabel>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={disabled}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name={`presets.${index}.mappings`}
                render={({ field }) => (
                  <FormItem>
                    <ModelMappingEditor
                      value={field.value}
                      onChange={field.onChange}
                      models={modelsPending ? undefined : modelsQuery.data}
                      targetsDisabled={modelsPending}
                      disabled={disabled}
                      showValidation={form.formState.submitCount > 0}
                    />
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          ))}

          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={disabled || fields.length >= MAX_PRESETS}
              onClick={() =>
                append({
                  id: createMappingRowId(),
                  name: '',
                  description: '',
                  recommended: false,
                  mappings: [{ source_model: '', target_model: '' }],
                })
              }
            >
              <Plus data-icon='inline-start' />
              {t('Add preset')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={disabled || fields.length >= MAX_PRESETS}
              onClick={() =>
                append({
                  id: createMappingRowId(),
                  name: 'Codex / Luna',
                  description: '',
                  recommended: true,
                  mappings: [
                    {
                      source_model: 'gpt-5.6-luna',
                      target_model: '',
                      reasoning_effort: 'low',
                    },
                    {
                      source_model: 'gpt-6-luna',
                      target_model: '',
                      reasoning_effort: 'low',
                    },
                  ],
                })
              }
            >
              {t('Use Codex / Luna template')}
            </Button>
          </div>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Template source names are examples. Check the exact names your clients send, then select targets from your enabled models.'
            )}
          </p>
        </form>
      </Form>
    </SettingsSection>
  )
}
