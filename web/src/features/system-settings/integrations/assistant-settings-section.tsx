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
  parseTokenAmount,
  type AssistantProfileName,
  type AssistantProtocol,
  type AssistantSource,
} from './assistant-settings-api'

export type AssistantProfileDefaults = {
  enabled: boolean
  source: AssistantSource
  protocol: AssistantProtocol
  baseUrl: string
  model: string
  channelId: number
  channelModel: string
  apiKeyConfigured: boolean
  contextWindow: number
  maxOutputTokens: number
}

export type AssistantSettingsDefaults = {
  enabled: boolean
  user: AssistantProfileDefaults
  admin: AssistantProfileDefaults
  systemPrompt: string
  requestTimeoutSeconds: number
  confirmWrites: boolean
  adminDbToolsEnabled: boolean
}

type AssistantSettingsSectionProps = {
  defaultValues: AssistantSettingsDefaults
}

/**
 * The shared form carries the master and safety switches in addition to the
 * profile cards rendered by AssistantProfileCard.
 */
type AssistantSettingsFormValuesWithMaster = AssistantSettingsFormValues & {
  enabled: boolean
  confirmWrites: boolean
  adminDbToolsEnabled: boolean
}

const OPTION_KEYS = {
  enabled: 'assistant_setting.enabled',
  systemPrompt: 'assistant_setting.system_prompt',
  requestTimeoutSeconds: 'assistant_setting.request_timeout_seconds',
  confirmWrites: 'assistant_setting.confirm_writes',
  adminDbToolsEnabled: 'assistant_setting.admin_db_tools_enabled',
  user: {
    enabled: 'assistant_setting.user_enabled',
    source: 'assistant_setting.user_source',
    protocol: 'assistant_setting.user_protocol',
    baseUrl: 'assistant_setting.user_base_url',
    apiKey: 'assistant_setting.user_api_key',
    model: 'assistant_setting.user_model',
    channelId: 'assistant_setting.user_channel_id',
    channelModel: 'assistant_setting.user_channel_model',
    contextWindow: 'assistant_setting.user_context_window',
    maxOutputTokens: 'assistant_setting.user_max_output_tokens',
  },
  admin: {
    enabled: 'assistant_setting.admin_enabled',
    source: 'assistant_setting.admin_source',
    protocol: 'assistant_setting.admin_protocol',
    baseUrl: 'assistant_setting.admin_base_url',
    apiKey: 'assistant_setting.admin_api_key',
    model: 'assistant_setting.admin_model',
    channelId: 'assistant_setting.admin_channel_id',
    channelModel: 'assistant_setting.admin_channel_model',
    contextWindow: 'assistant_setting.admin_context_window',
    maxOutputTokens: 'assistant_setting.admin_max_output_tokens',
  },
} as const

/**
 * One profile is only usable when its selected source is fully configured and
 * its token limits are consistent: a direct profile needs an API root and
 * model, a channel profile needs the channel and one of its models, and the
 * output limit must stay strictly below the context window. The API key stays
 * optional because an empty field means "keep the stored key" (direct) or
 * "use the channel key" (channel).
 *
 * All of those checks are skipped while the master switch is off: turning the
 * assistant off must always be saveable, whatever state the profiles are in.
 */
const createAssistantSettingsSchema = (t: TFunction) => {
  const rangeMessage = t('Value is out of range')
  const profileSchema = z.object({
    enabled: z.boolean(),
    source: z.enum(['direct', 'channel']),
    protocol: z.enum(['responses', 'chat']),
    baseUrl: z.string(),
    model: z.string(),
    channelId: z.number().int(),
    channelModel: z.string(),
    apiKey: z.string(),
    contextWindow: z.string(),
    maxOutputTokens: z.string(),
  })

  return z
    .object({
      enabled: z.boolean(),
      user: profileSchema,
      admin: profileSchema,
      systemPrompt: z.string(),
      requestTimeoutSeconds: z
        .number()
        .int(rangeMessage)
        .min(5, rangeMessage)
        .max(300, rangeMessage),
      confirmWrites: z.boolean(),
      adminDbToolsEnabled: z.boolean(),
    })
    .superRefine((values, ctx) => {
      if (!values.enabled) return

      for (const profileName of ['user', 'admin'] as const) {
        const profile = values[profileName]
        const addIssue = (
          field: keyof AssistantProfileFormValues,
          message: string
        ) => {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: [profileName, field],
            message,
          })
        }

        const contextWindow = parseTokenAmount(profile.contextWindow)
        if (
          contextWindow === null ||
          contextWindow < 1024 ||
          contextWindow > 10000000
        ) {
          addIssue('contextWindow', rangeMessage)
        }
        const maxOutputTokens = parseTokenAmount(profile.maxOutputTokens)
        if (
          maxOutputTokens === null ||
          maxOutputTokens < 1 ||
          maxOutputTokens > 1000000
        ) {
          addIssue('maxOutputTokens', rangeMessage)
        }
        if (
          contextWindow !== null &&
          maxOutputTokens !== null &&
          maxOutputTokens >= contextWindow
        ) {
          addIssue(
            'maxOutputTokens',
            t('Max output tokens must be less than the context window')
          )
        }

        if (!profile.enabled) continue

        if (profile.source === 'direct') {
          const baseUrl = profile.baseUrl.trim()
          if (baseUrl === '') {
            addIssue('baseUrl', t('Base URL is required'))
          } else if (!z.string().url().safeParse(baseUrl).success) {
            addIssue('baseUrl', t('Must be a valid URL'))
          }
          if (profile.model.trim() === '') {
            addIssue('model', t('Model is required'))
          }
          continue
        }

        if (profile.channelId <= 0) {
          addIssue('channelId', t('Channel is required'))
        }
        if (profile.channelModel.trim() === '') {
          addIssue('channelModel', t('Channel model is required'))
        }
      }
    })
}
function buildProfileFormDefaults(
  defaults: AssistantProfileDefaults
): AssistantProfileFormValues {
  return {
    enabled: defaults.enabled,
    source: defaults.source,
    protocol: defaults.protocol,
    baseUrl: defaults.baseUrl,
    model: defaults.model,
    channelId: defaults.channelId,
    channelModel: defaults.channelModel,
    apiKey: '',
    contextWindow: String(defaults.contextWindow),
    maxOutputTokens: String(defaults.maxOutputTokens),
  }
}

function buildFormDefaults(
  defaults: AssistantSettingsDefaults
): AssistantSettingsFormValuesWithMaster {
  return {
    enabled: defaults.enabled,
    user: buildProfileFormDefaults(defaults.user),
    admin: buildProfileFormDefaults(defaults.admin),
    systemPrompt: defaults.systemPrompt,
    requestTimeoutSeconds: defaults.requestTimeoutSeconds,
    confirmWrites: defaults.confirmWrites,
    adminDbToolsEnabled: defaults.adminDbToolsEnabled,
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

  if (values.protocol !== defaults.protocol) {
    entries.push({ key: keys.protocol, value: values.protocol })
  }
  const contextWindow = parseTokenAmount(values.contextWindow)
  if (contextWindow !== null && contextWindow !== defaults.contextWindow) {
    entries.push({ key: keys.contextWindow, value: contextWindow })
  }
  const maxOutputTokens = parseTokenAmount(values.maxOutputTokens)
  if (
    maxOutputTokens !== null &&
    maxOutputTokens !== defaults.maxOutputTokens
  ) {
    entries.push({ key: keys.maxOutputTokens, value: maxOutputTokens })
  }
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
  values: AssistantSettingsFormValuesWithMaster,
  defaults: AssistantSettingsDefaults
): UpdateOptionRequest[] {
  const entries: UpdateOptionRequest[] = []

  if (values.enabled !== defaults.enabled) {
    entries.push({ key: OPTION_KEYS.enabled, value: values.enabled })
  }

  if (values.systemPrompt !== defaults.systemPrompt) {
    entries.push({ key: OPTION_KEYS.systemPrompt, value: values.systemPrompt })
  }
  if (values.requestTimeoutSeconds !== defaults.requestTimeoutSeconds) {
    entries.push({
      key: OPTION_KEYS.requestTimeoutSeconds,
      value: values.requestTimeoutSeconds,
    })
  }

  if (values.confirmWrites !== defaults.confirmWrites) {
    entries.push({
      key: OPTION_KEYS.confirmWrites,
      value: values.confirmWrites,
    })
  }
  if (values.adminDbToolsEnabled !== defaults.adminDbToolsEnabled) {
    entries.push({
      key: OPTION_KEYS.adminDbToolsEnabled,
      value: values.adminDbToolsEnabled,
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

  const form = useForm<AssistantSettingsFormValuesWithMaster>({
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

  const onSubmit = async (values: AssistantSettingsFormValuesWithMaster) => {
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

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable AI Assistant')}</FormLabel>
                  <FormDescription>
                    {t('Master switch for the user and admin assistants.')}
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

          <div data-settings-form-span='full' className='min-w-0'>
            <h4 className='text-sm font-semibold'>{t('Safety')}</h4>
          </div>

          <FormField
            control={form.control}
            name='confirmWrites'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Require confirmation for AI write operations')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Ask for confirmation before the assistant creates or changes data. Enabled by default.'
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

          <FormField
            control={form.control}
            name='adminDbToolsEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Allow root database tools')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow database tools in the root admin assistant only. Enabled by default.'
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
