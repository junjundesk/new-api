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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useRef } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSaveOptions } from '../hooks/use-save-options'
import type { UpdateOptionRequest } from '../types'

const MAX_RULES = 200

type ErrorMappingRulePayload = {
  enabled: boolean
  status_code: number
  match_message?: string
  target_status?: number
  target_message: string
}

type ErrorMappingRuleFormItem = {
  enabled: boolean
  status_code: string
  match_message: string
  target_status: string
  target_message: string
}

type ErrorMappingFormValues = {
  enabled: boolean
  rules: ErrorMappingRuleFormItem[]
}

export type ErrorMappingSectionProps = {
  defaultValues: {
    'error_mapping_setting.enabled': boolean
    'error_mapping_setting.rules': string
  }
}

const storedRuleSchema = z.object({
  enabled: z.boolean().default(true),
  status_code: z.number().int().min(0).max(599).default(0),
  match_message: z.string().optional().default(''),
  target_status: z.number().int().min(0).max(599).optional().default(0),
  target_message: z.string().default(''),
})

function parseOptionalStatusCode(raw: string): number | null {
  const trimmed = raw.trim()
  if (!trimmed) return 0
  if (!/^\d+$/.test(trimmed)) return null
  const num = Number(trimmed)
  if (!Number.isInteger(num) || num < 100 || num > 599 || num === 200) {
    return null
  }
  return num
}

function parseStoredRules(raw: string): {
  valid: boolean
  rules: ErrorMappingRuleFormItem[]
} {
  const trimmed = (raw ?? '').trim()
  if (!trimmed) {
    return { valid: true, rules: [] }
  }
  try {
    const parsedJson: unknown = JSON.parse(trimmed)
    const parsed = z.array(storedRuleSchema).safeParse(parsedJson)
    if (!parsed.success) {
      return { valid: false, rules: [] }
    }
    return {
      valid: true,
      rules: parsed.data.map((rule) => ({
        enabled: rule.enabled,
        status_code: rule.status_code > 0 ? String(rule.status_code) : '',
        match_message: rule.match_message ?? '',
        target_status: rule.target_status > 0 ? String(rule.target_status) : '',
        target_message: rule.target_message ?? '',
      })),
    }
  } catch {
    return { valid: false, rules: [] }
  }
}

function serializeRules(rules: ErrorMappingRuleFormItem[]): string {
  const payload: ErrorMappingRulePayload[] = rules.map((rule) => {
    const statusCode = parseOptionalStatusCode(rule.status_code) ?? 0
    const targetStatus = parseOptionalStatusCode(rule.target_status) ?? 0
    const matchMessage = rule.match_message.trim()
    const targetMessage = rule.target_message.trim()
    const item: ErrorMappingRulePayload = {
      enabled: rule.enabled,
      status_code: statusCode,
      target_message: targetMessage,
    }
    if (matchMessage) {
      item.match_message = matchMessage
    }
    if (targetStatus > 0) {
      item.target_status = targetStatus
    }
    return item
  })
  return JSON.stringify(payload)
}

export function ErrorMappingSection(props: ErrorMappingSectionProps) {
  const { t } = useTranslation()
  const { saveOptions, isSaving } = useSaveOptions()
  const defaultEnabled = props.defaultValues['error_mapping_setting.enabled']
  const defaultRulesRaw = props.defaultValues['error_mapping_setting.rules']

  const parsedStored = useMemo(
    () => parseStoredRules(defaultRulesRaw),
    [defaultRulesRaw]
  )
  const invalidConfig = !parsedStored.valid

  const formSchema = useMemo(
    () =>
      z.object({
        enabled: z.boolean(),
        rules: z
          .array(
            z
              .object({
                enabled: z.boolean(),
                status_code: z.string(),
                match_message: z.string(),
                target_status: z.string(),
                target_message: z.string(),
              })
              .superRefine((rule, ctx) => {
                const statusCode = parseOptionalStatusCode(rule.status_code)
                if (statusCode === null) {
                  ctx.addIssue({
                    code: 'custom',
                    path: ['status_code'],
                    message: t(
                      'Enter an HTTP status code between 100 and 599 (excluding 200), or leave empty'
                    ),
                  })
                }
                const targetStatus = parseOptionalStatusCode(rule.target_status)
                if (targetStatus === null) {
                  ctx.addIssue({
                    code: 'custom',
                    path: ['target_status'],
                    message: t(
                      'Enter an HTTP status code between 100 and 599 (excluding 200), or leave empty'
                    ),
                  })
                }
                if (
                  statusCode === 0 &&
                  rule.match_message.trim().length === 0
                ) {
                  ctx.addIssue({
                    code: 'custom',
                    path: ['status_code'],
                    message: t(
                      'Specify a match status code or an upstream message keyword'
                    ),
                  })
                }
                if (rule.target_message.trim().length === 0) {
                  ctx.addIssue({
                    code: 'custom',
                    path: ['target_message'],
                    message: t('Mapped error message is required'),
                  })
                }
              })
          )
          .max(MAX_RULES),
      }),
    [t]
  )

  const defaultFormValues = useMemo<ErrorMappingFormValues>(
    () => ({
      enabled: defaultEnabled,
      rules: parsedStored.rules,
    }),
    [defaultEnabled, parsedStored.rules]
  )

  const baselineRef = useRef<{ enabled: boolean; rulesJson: string }>({
    enabled: defaultFormValues.enabled,
    rulesJson: serializeRules(defaultFormValues.rules),
  })

  const form = useForm<ErrorMappingFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: defaultFormValues,
  })

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'rules',
    keyName: 'fieldKey',
  })

  useEffect(() => {
    if (!invalidConfig) {
      form.reset(defaultFormValues)
      baselineRef.current = {
        enabled: defaultFormValues.enabled,
        rulesJson: serializeRules(defaultFormValues.rules),
      }
    }
  }, [defaultFormValues, form, invalidConfig])

  const handleAddRule = () => {
    if (fields.length >= MAX_RULES) return
    append({
      enabled: true,
      status_code: '',
      match_message: '',
      target_status: '',
      target_message: '',
    })
  }

  const handleAddExampleRule = () => {
    if (fields.length >= MAX_RULES) return
    append({
      enabled: true,
      status_code: '502',
      match_message: '',
      target_status: '',
      target_message: t('Upstream service unavailable, please contact administrator'),
    })
  }

  const onSubmit = async (values: ErrorMappingFormValues) => {
    if (invalidConfig || isSaving) return
    const nextRulesJson = serializeRules(values.rules)
    const entries: UpdateOptionRequest[] = []

    if (values.enabled !== baselineRef.current.enabled) {
      entries.push({
        key: 'error_mapping_setting.enabled',
        value: values.enabled,
      })
    }
    if (nextRulesJson !== baselineRef.current.rulesJson) {
      entries.push({
        key: 'error_mapping_setting.rules',
        value: nextRulesJson,
      })
    }

    const saved = await saveOptions(entries)
    if (saved) {
      baselineRef.current = {
        enabled: values.enabled,
        rulesJson: nextRulesJson,
      }
      form.reset(values)
    }
  }

  return (
    <SettingsSection title={t('Error Response Mapping')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={() => {
              if (!invalidConfig) {
                form.reset(defaultFormValues)
              }
            }}
            isSaving={isSaving}
            isSaveDisabled={invalidConfig}
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Error Response Mapping')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Rewrite client-facing HTTP status codes and error messages across OpenAI, Claude, Gemini, and other relay protocols so raw upstream errors are hidden from users. Real upstream errors are still preserved in server and admin logs.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={invalidConfig}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {invalidConfig && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Invalid error mapping configuration')}</AlertTitle>
              <AlertDescription>
                {t(
                  'The saved error_mapping_setting.rules value could not be parsed. Fix it in the database before editing here.'
                )}
              </AlertDescription>
            </Alert>
          )}

          <Alert>
            <AlertTitle>{t('Rule Matching Priority')}</AlertTitle>
            <AlertDescription>
              {t(
                'Rules are matched in priority order: (1) Exact HTTP status code + message keyword, (2) Exact HTTP status code for all messages, (3) Message keyword across any error status code. Keyword matching is case-insensitive and never rewrites HTTP 200 responses.'
              )}
            </AlertDescription>
          </Alert>

          <Separator />

          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='flex items-center gap-2'>
              <h3 className='text-sm font-medium'>{t('Mapping Rules')}</h3>
              <Badge variant='secondary'>{fields.length}</Badge>
            </div>
            <div className='flex items-center gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={handleAddExampleRule}
                disabled={invalidConfig || fields.length >= MAX_RULES}
              >
                {t('Add 502 Example')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={handleAddRule}
                disabled={invalidConfig || fields.length >= MAX_RULES}
              >
                <Plus className='mr-1.5 size-4' />
                {t('Add Rule')}
              </Button>
            </div>
          </div>

          {fields.length === 0 && !invalidConfig ? (
            <Empty className='border'>
              <EmptyHeader>
                <EmptyTitle>{t('No error mapping rules configured')}</EmptyTitle>
                <EmptyDescription>
                  {t(
                    'Add a rule to mask upstream error reasons (for example, map HTTP 502 to a friendly message) before returning responses to clients.'
                  )}
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <div className='flex flex-wrap justify-center gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={handleAddExampleRule}
                  >
                    {t('Add 502 Example')}
                  </Button>
                  <Button type='button' size='sm' onClick={handleAddRule}>
                    <Plus className='mr-1.5 size-4' />
                    {t('Add Rule')}
                  </Button>
                </div>
              </EmptyContent>
            </Empty>
          ) : (
            <div className='flex flex-col gap-4'>
              {fields.map((fieldItem, index) => (
                <div
                  key={fieldItem.fieldKey}
                  className='rounded-lg border p-4 flex flex-col gap-4'
                >
                  <div className='flex items-center justify-between gap-2'>
                    <div className='flex items-center gap-3'>
                      <Badge variant='outline'>#{index + 1}</Badge>
                      <FormField
                        control={form.control}
                        name={`rules.${index}.enabled`}
                        render={({ field }) => (
                          <div className='flex items-center gap-2'>
                            <Switch
                              checked={field.value}
                              onCheckedChange={field.onChange}
                              aria-label={t('Enable rule')}
                            />
                            <span className='text-muted-foreground text-xs'>
                              {field.value ? t('Enabled') : t('Disabled')}
                            </span>
                          </div>
                        )}
                      />
                    </div>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      onClick={() => remove(index)}
                      aria-label={t('Delete rule')}
                    >
                      <Trash2 className='size-4 text-destructive' />
                    </Button>
                  </div>

                  <div className='grid gap-4 lg:grid-cols-2'>
                    <FormField
                      control={form.control}
                      name={`rules.${index}.status_code`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Match HTTP Status Code')}</FormLabel>
                          <FormControl>
                            <Input
                              {...field}
                              inputMode='numeric'
                              placeholder={t('e.g. 502 (leave empty for any)')}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Upstream HTTP error status code to match, such as 502, 503, or 429.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name={`rules.${index}.match_message`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Match Upstream Message Keyword')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              {...field}
                              placeholder={t(
                                'Optional keyword (leave empty to match all reasons)'
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Case-insensitive substring match against the upstream error reason.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name={`rules.${index}.target_status`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Mapped HTTP Status Code')}</FormLabel>
                          <FormControl>
                            <Input
                              {...field}
                              inputMode='numeric'
                              placeholder={t(
                                'Optional (leave empty to keep original)'
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'HTTP status code returned to clients. Leave empty to keep the original status code.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name={`rules.${index}.target_message`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Mapped Error Message')}</FormLabel>
                          <FormControl>
                            <Input
                              {...field}
                              placeholder={t(
                                'e.g. Upstream service unavailable, please contact administrator'
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Error reason returned to clients and shown in user logs instead of the real upstream error.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
