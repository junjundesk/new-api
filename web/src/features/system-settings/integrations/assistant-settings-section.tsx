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
import type { TFunction } from 'i18next'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useSaveOptions } from '../hooks/use-save-options'
import type { UpdateOptionRequest } from '../types'
import { safeNumberFieldProps } from '../utils/numeric-field'
import {
  AssistantProfileCard,
  type AssistantProfileFormValues,
  type AssistantSettingsFormValues,
} from './assistant-profile-card'
import {
  listAssistantChannels,
  type AssistantProfileName,
  type AssistantSource,
} from './assistant-settings-api'

export type AssistantProfileDefaults = {
  enabled: boolean
  source: AssistantSource
  baseUrl: string
  model: string
  channelId: number
  channelModel: string
  apiKeyConfigured: boolean
}

export type AssistantSettingsDefaults = {
  user: AssistantProfileDefaults
  admin: AssistantProfileDefaults
  systemPrompt: string
  maxToolRounds: number
  requestTimeoutSeconds: number
  maxOutputTokens: number
  dailyUserMessageLimit: number
  allowTokenActions: boolean
}

type AssistantSettingsSectionProps = {
  defaultValues: AssistantSettingsDefaults
}

const OPTION_KEYS = {
  systemPrompt: 'assistant_setting.system_prompt',
  maxToolRounds: 'assistant_setting.max_tool_rounds',
  requestTimeoutSeconds: 'assistant_setting.request_timeout_seconds',
  maxOutputTokens: 'assistant_setting.max_output_tokens',
  dailyUserMessageLimit: 'assistant_setting.daily_user_message_limit',
  allowTokenActions: 'assistant_setting.allow_token_actions',
  user: {
    enabled: 'assistant_setting.user_enabled',
    source: 'assistant_setting.user_source',
    baseUrl: 'assistant_setting.user_base_url',
    apiKey: 'assistant_setting.user_api_key',
    model: 'assistant_setting.user_model',
    channelId: 'assistant_setting.user_channel_id',
    channelModel: 'assistant_setting.user_channel_model',
  },
  admin: {
    enabled: 'assistant_setting.admin_enabled',
    source: 'assistant_setting.admin_source',
    baseUrl: 'assistant_setting.admin_base_url',
    apiKey: 'assistant_setting.admin_api_key',
    model: 'assistant_setting.admin_model',
    channelId: 'assistant_setting.admin_channel_id',
    channelModel: 'assistant_setting.admin_channel_model',
  },
} as const

/**
 * One profile is only usable when its selected source is fully configured:
 * a direct profile needs an API root and model, a channel profile needs the
 * channel and one of its models. The API key stays optional because an empty
 * field means "keep the stored key" (direct) or "use the channel key" (channel).
 */
const createAssistantSettingsSchema = (t: TFunction) => {
  const profileSchema = z
    .object({
      enabled: z.boolean(),
      source: z.enum(['direct', 'channel']),
      baseUrl: z.string(),
      model: z.string(),
      channelId: z.number().int(),
      channelModel: z.string(),
      apiKey: z.string(),
    })
    .superRefine((profile, ctx) => {
      if (!profile.enabled) return

      if (profile.source === 'direct') {
        const baseUrl = profile.baseUrl.trim()
        if (baseUrl === '') {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['baseUrl'],
            message: t('Base URL is required'),
          })
        } else if (!z.string().url().safeParse(baseUrl).success) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['baseUrl'],
            message: t('Must be a valid URL'),
          })
        }
        if (profile.model.trim() === '') {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['model'],
            message: t('Model is required'),
          })
        }
        return
      }

      if (profile.channelId <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['channelId'],
          message: t('Channel is required'),
        })
      }
      if (profile.channelModel.trim() === '') {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['channelModel'],
          message: t('Channel model is required'),
        })
      }
    })

  return z.object({
    user: profileSchema,
    admin: profileSchema,
    systemPrompt: z.string(),
    maxToolRounds: z
      .number()
      .int(t('Value is out of range'))
      .min(1, t('Value is out of range'))
      .max(10, t('Value is out of range')),
    requestTimeoutSeconds: z
      .number()
      .int(t('Value is out of range'))
      .min(5, t('Value is out of range'))
      .max(300, t('Value is out of range')),
    maxOutputTokens: z
      .number()
      .int(t('Value is out of range'))
      .min(64, t('Value is out of range'))
      .max(8192, t('Value is out of range')),
    dailyUserMessageLimit: z
      .number()
      .int(t('Value is out of range'))
      .min(0, t('Value is out of range'))
      .max(10000, t('Value is out of range')),
    allowTokenActions: z.boolean(),
  })
}

function buildProfileFormDefaults(
  defaults: AssistantProfileDefaults
): AssistantProfileFormValues {
  return {
    enabled: defaults.enabled,
    source: defaults.source,
    baseUrl: defaults.baseUrl,
    model: defaults.model,
    channelId: defaults.channelId,
    channelModel: defaults.channelModel,
    apiKey: '',
  }
}

function buildFormDefaults(
  defaults: AssistantSettingsDefaults
): AssistantSettingsFormValues {
  return {
    user: buildProfileFormDefaults(defaults.user),
    admin: buildProfileFormDefaults(defaults.admin),
    systemPrompt: defaults.systemPrompt,
    maxToolRounds: defaults.maxToolRounds,
    requestTimeoutSeconds: defaults.requestTimeoutSeconds,
    maxOutputTokens: defaults.maxOutputTokens,
    dailyUserMessageLimit: defaults.dailyUserMessageLimit,
    allowTokenActions: defaults.allowTokenActions,
  }
}

/**
 * Builds the option writes for one profile. The profile configuration is
 * written before its enabled flag so the backend never switches a profile on
 * before the fields it needs are stored.
 */
function buildProfileOptionEntries(
  profile: AssistantProfileName,
  values: AssistantProfileFormValues,
  defaults: AssistantProfileDefaults
): UpdateOptionRequest[] {
  const keys = OPTION_KEYS[profile]
  const entries: UpdateOptionRequest[] = []

  const apiKey = values.apiKey.trim()
  if (apiKey !== '') {
    entries.push({ key: keys.apiKey, value: apiKey })
  }
  if (values.source !== defaults.source) {
    entries.push({ key: keys.source, value: values.source })
  }
  const baseUrl = values.baseUrl.trim()
  if (baseUrl !== defaults.baseUrl) {
    entries.push({ key: keys.baseUrl, value: baseUrl })
  }
  const model = values.model.trim()
  if (model !== defaults.model) {
    entries.push({ key: keys.model, value: model })
  }
  if (values.channelId !== defaults.channelId) {
    entries.push({ key: keys.channelId, value: values.channelId })
  }
  const channelModel = values.channelModel.trim()
  if (channelModel !== defaults.channelModel) {
    entries.push({ key: keys.channelModel, value: channelModel })
  }
  if (values.enabled !== defaults.enabled) {
    entries.push({ key: keys.enabled, value: values.enabled })
  }

  return entries
}

function buildOptionEntries(
  values: AssistantSettingsFormValues,
  defaults: AssistantSettingsDefaults
): UpdateOptionRequest[] {
  const entries: UpdateOptionRequest[] = []

  if (values.systemPrompt !== defaults.systemPrompt) {
    entries.push({ key: OPTION_KEYS.systemPrompt, value: values.systemPrompt })
  }

  const numericEntries: Array<[string, number, number]> = [
    [OPTION_KEYS.maxToolRounds, values.maxToolRounds, defaults.maxToolRounds],
    [
      OPTION_KEYS.requestTimeoutSeconds,
      values.requestTimeoutSeconds,
      defaults.requestTimeoutSeconds,
    ],
    [
      OPTION_KEYS.maxOutputTokens,
      values.maxOutputTokens,
      defaults.maxOutputTokens,
    ],
    [
      OPTION_KEYS.dailyUserMessageLimit,
      values.dailyUserMessageLimit,
      defaults.dailyUserMessageLimit,
    ],
  ]
  for (const [key, value, defaultValue] of numericEntries) {
    if (value !== defaultValue) entries.push({ key, value })
  }

  if (values.allowTokenActions !== defaults.allowTokenActions) {
    entries.push({
      key: OPTION_KEYS.allowTokenActions,
      value: values.allowTokenActions,
    })
  }

  entries.push(...buildProfileOptionEntries('user', values.user, defaults.user))
  entries.push(
    ...buildProfileOptionEntries('admin', values.admin, defaults.admin)
  )

  return entries
}

export function AssistantSettingsSection(props: AssistantSettingsSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { saveOptions, isSaving } = useSaveOptions()

  const { data: channelsData } = useQuery({
    queryKey: ['assistant-channels'],
    queryFn: listAssistantChannels,
  })
  const channels = channelsData?.data ?? []

  const schema = useMemo(() => createAssistantSettingsSchema(t), [t])
  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<AssistantSettingsFormValues>({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const [apiKeyConfigured, setApiKeyConfigured] = useState({
    user: props.defaultValues.user.apiKeyConfigured,
    admin: props.defaultValues.admin.apiKeyConfigured,
  })

  useEffect(() => {
    setApiKeyConfigured({
      user: props.defaultValues.user.apiKeyConfigured,
      admin: props.defaultValues.admin.apiKeyConfigured,
    })
  }, [
    props.defaultValues.user.apiKeyConfigured,
    props.defaultValues.admin.apiKeyConfigured,
  ])

  const isBusy = isSaving || form.formState.isSubmitting

  const handleApiKeyCleared = (profile: AssistantProfileName) => {
    setApiKeyConfigured((current) => ({ ...current, [profile]: false }))
    queryClient.invalidateQueries({ queryKey: ['system-options'] })
    queryClient.invalidateQueries({ queryKey: ['status'] })
  }

  const onSubmit = async (values: AssistantSettingsFormValues) => {
    const entries = buildOptionEntries(values, props.defaultValues)
    const saved = await saveOptions(entries)
    if (!saved) return

    setApiKeyConfigured((current) => ({
      user: current.user || values.user.apiKey.trim() !== '',
      admin: current.admin || values.admin.apiKey.trim() !== '',
    }))
    form.reset({
      ...values,
      user: { ...values.user, apiKey: '' },
      admin: { ...values.admin, apiKey: '' },
    })
  }

  return (
    <SettingsSection title={t('AI Assistant')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={isBusy}
            isSaveDisabled={!form.formState.isDirty}
          />

          <AssistantProfileCard
            profile='user'
            title={t('User assistant')}
            description={t('Used by signed-in users in the console assistant.')}
            channels={channels}
            isBusy={isBusy}
            apiKeyConfigured={apiKeyConfigured.user}
            onApiKeyCleared={handleApiKeyCleared}
          />

          <AssistantProfileCard
            profile='admin'
            title={t('Admin assistant')}
            description={t(
              'Used by root administrators in the console assistant.'
            )}
            channels={channels}
            isBusy={isBusy}
            apiKeyConfigured={apiKeyConfigured.admin}
            onApiKeyCleared={handleApiKeyCleared}
          />

          <div data-settings-form-span='full' className='min-w-0'>
            <h4 className='text-sm font-semibold'>{t('Shared behavior')}</h4>
          </div>

          <FormField
            control={form.control}
            name='systemPrompt'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('System Prompt')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={4}
                    placeholder={t(
                      'Optional instructions applied to every assistant conversation'
                    )}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Optional instructions applied to every assistant conversation'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='maxToolRounds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max tool rounds')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={10}
                    step={1}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('From {{min}} to {{max}}', { min: 1, max: 10 })}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='requestTimeoutSeconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Request timeout (seconds)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={5}
                    max={300}
                    step={1}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('From {{min}} to {{max}}', { min: 5, max: 300 })}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='maxOutputTokens'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max output tokens')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={64}
                    max={8192}
                    step={1}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('From {{min}} to {{max}}', { min: 64, max: 8192 })}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='dailyUserMessageLimit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Daily message limit per user')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    max={10000}
                    step={1}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('From {{min}} to {{max}}', { min: 0, max: 10000 })}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='allowTokenActions'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Allow token actions')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow the assistant to create and manage API tokens through tools'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={isBusy}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <Alert variant='default'>
            <AlertTitle>{t('Provider & data privacy')}</AlertTitle>
            <AlertDescription>
              {t(
                'Conversation content and tool results are sent to the configured upstream model provider. Review the provider data policy before enabling.'
              )}
            </AlertDescription>
          </Alert>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
