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
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  CHANNEL_STATUS,
  CHANNEL_STATUS_LABELS,
} from '@/features/channels/constants'

import {
  clearAssistantApiKey,
  listAssistantModels,
  parseTokenAmount,
  testAssistantConnection,
  type AssistantChannel,
  type AssistantModelsRequest,
  type AssistantProfileName,
  type AssistantProfileRequest,
  type AssistantProtocol,
  type AssistantSource,
} from './assistant-settings-api'

export type AssistantProfileFormValues = {
  enabled: boolean
  source: AssistantSource
  protocol: AssistantProtocol
  baseUrl: string
  model: string
  channelId: number
  channelModel: string
  apiKey: string
  contextWindow: string
  maxOutputTokens: string
}

export type AssistantSettingsFormValues = {
  user: AssistantProfileFormValues
  admin: AssistantProfileFormValues
  systemPrompt: string
  requestTimeoutSeconds: number
}

type ConnectionTestState =
  | { status: 'idle' }
  | { status: 'testing' }
  | { status: 'success'; latencyMs?: number }
  | { status: 'error'; message: string }

type ModelFetchState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'error'; message: string }

type AssistantProfileCardProps = {
  profile: AssistantProfileName
  title: string
  description: string
  channels: AssistantChannel[]
  isBusy: boolean
  apiKeyConfigured: boolean
  onApiKeyCleared: (profile: AssistantProfileName) => void
}

type FetchModelsButtonProps = {
  isFetching: boolean
  disabled: boolean
  onClick: () => void
}

/**
 * Path of one profile field inside the shared settings form. The returned
 * template literal keeps the field value type precise for the form helpers.
 */
function profileFieldPath<F extends keyof AssistantProfileFormValues>(
  profile: AssistantProfileName,
  field: F
): `${AssistantProfileName}.${F}` {
  return `${profile}.${field}` as `${AssistantProfileName}.${F}`
}

/**
 * Builds the connection fields shared by the connection test and the model-list
 * request. An empty API key is omitted so the backend keeps the stored key.
 */
function buildProfileRequest(
  profile: AssistantProfileName,
  values: AssistantProfileFormValues
): AssistantProfileRequest {
  const request: AssistantProfileRequest = {
    profile,
    source: values.source,
    protocol: values.protocol,
  }
  const apiKey = values.apiKey.trim()
  if (apiKey !== '') request.api_key = apiKey
  if (values.source === 'direct') {
    request.base_url = values.baseUrl.trim()
    request.model = values.model.trim()
  } else {
    request.channel_id = values.channelId
    request.channel_model = values.channelModel.trim()
  }
  return request
}

function FetchModelsButton(props: FetchModelsButtonProps) {
  const { t } = useTranslation()

  return (
    <Button
      type='button'
      variant='secondary'
      className='shrink-0'
      onClick={props.onClick}
      disabled={props.disabled || props.isFetching}
    >
      {props.isFetching ? (
        <Loader2 className='me-2 size-4 animate-spin' />
      ) : null}
      {t('Fetch models')}
    </Button>
  )
}

export function AssistantProfileCard(props: AssistantProfileCardProps) {
  const { t } = useTranslation()
  const form = useFormContext<AssistantSettingsFormValues>()

  const [testState, setTestState] = useState<ConnectionTestState>({
    status: 'idle',
  })
  const [fetchState, setFetchState] = useState<ModelFetchState>({
    status: 'idle',
  })
  const [fetchedModels, setFetchedModels] = useState<string[]>([])
  const [clearDialogOpen, setClearDialogOpen] = useState(false)
  const [isClearing, setIsClearing] = useState(false)

  const source = form.watch(profileFieldPath(props.profile, 'source'))
  const channelId = form.watch(profileFieldPath(props.profile, 'channelId'))
  const selectedChannel = props.channels.find(
    (channel) => channel.id === channelId
  )

  const sourceItems = useMemo(
    () => [
      { value: 'direct' as const, label: t('Direct') },
      { value: 'channel' as const, label: t('Channel') },
    ],
    [t]
  )

  const protocolItems = useMemo(
    () => [
      { value: 'responses' as const, label: t('OpenAI Responses') },
      { value: 'chat' as const, label: t('OpenAI Chat') },
    ],
    [t]
  )

  const channelItems = useMemo(
    () =>
      props.channels.map((channel) => {
        const statusLabel =
          CHANNEL_STATUS_LABELS[
            channel.status as keyof typeof CHANNEL_STATUS_LABELS
          ] ?? 'Unknown'
        const statusSuffix =
          channel.status === CHANNEL_STATUS.ENABLED
            ? ''
            : ` - ${t(statusLabel)}`
        return {
          value: channel.id,
          label: `${channel.name} (#${channel.id}, ${channel.type_name})${statusSuffix}`,
        }
      }),
    [props.channels, t]
  )

  const modelSuggestions = useMemo(() => {
    const merged = new Set<string>()
    if (source === 'channel') {
      for (const modelName of selectedChannel?.models ?? []) {
        merged.add(modelName)
      }
    }
    for (const modelName of fetchedModels) {
      merged.add(modelName)
    }
    return [...merged]
  }, [source, selectedChannel, fetchedModels])

  const modelListId = `assistant-${props.profile}-model-options`
  const isFetching = fetchState.status === 'loading'

  const handleTestConnection = async () => {
    setTestState({ status: 'testing' })

    const profileValues = form.getValues()[props.profile]
    try {
      const response = await testAssistantConnection(
        buildProfileRequest(props.profile, profileValues)
      )
      if (!response.success) {
        setTestState({
          status: 'error',
          message: response.message || t('Connection failed'),
        })
        return
      }
      setTestState({
        status: 'success',
        latencyMs: response.data?.latency_ms,
      })
    } catch (error) {
      setTestState({
        status: 'error',
        message:
          error instanceof Error ? error.message : t('Connection failed'),
      })
    }
  }

  const handleFetchModels = async () => {
    setFetchState({ status: 'loading' })

    const profileValues = form.getValues()[props.profile]
    const contextWindow = parseTokenAmount(profileValues.contextWindow)
    const maxOutputTokens = parseTokenAmount(profileValues.maxOutputTokens)
    const request: AssistantModelsRequest = {
      ...buildProfileRequest(props.profile, profileValues),
    }
    if (contextWindow !== null) request.context_window = contextWindow
    if (maxOutputTokens !== null) request.max_output_tokens = maxOutputTokens
    try {
      const response = await listAssistantModels(request)
      if (!response.success) {
        setFetchState({
          status: 'error',
          message: response.message || t('Failed to fetch models'),
        })
        return
      }
      setFetchedModels(response.data?.models ?? [])
      setFetchState({ status: 'idle' })
    } catch (error) {
      setFetchState({
        status: 'error',
        message:
          error instanceof Error ? error.message : t('Failed to fetch models'),
      })
    }
  }

  const handleClearKey = async () => {
    setIsClearing(true)
    try {
      const response = await clearAssistantApiKey(props.profile)
      if (!response.success) {
        toast.error(response.message || t('Failed to clear API key'))
        return
      }

      form.setValue(profileFieldPath(props.profile, 'apiKey'), '')
      setClearDialogOpen(false)
      toast.success(t('API key cleared'))
      props.onApiKeyCleared(props.profile)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to clear API key')
      )
    } finally {
      setIsClearing(false)
    }
  }

  const apiKeyDescription =
    source === 'direct'
      ? t('Leave blank to keep the existing API key')
      : t('Optional override; leave blank to use the selected channel API key')

  return (
    <section
      role='region'
      aria-label={props.title}
      data-settings-form-span='full'
      className='min-w-0 space-y-4 rounded-xl border p-4'
    >
      <header className='flex flex-wrap items-start justify-between gap-4'>
        <div className='min-w-0 space-y-1'>
          <h4 className='text-sm font-semibold'>{props.title}</h4>
          <p className='text-muted-foreground text-sm'>{props.description}</p>
        </div>
        <FormField
          control={form.control}
          name={profileFieldPath(props.profile, 'enabled')}
          render={({ field }) => (
            <FormItem className='flex flex-row items-center gap-2 space-y-0'>
              <FormLabel className='text-sm font-medium'>
                {t('Enabled')}
              </FormLabel>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  disabled={props.isBusy}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </header>

      <div className='grid gap-4 sm:grid-cols-2'>
        <FormField
          control={form.control}
          name={profileFieldPath(props.profile, 'source')}
          render={({ field }) => (
            <FormItem className='sm:col-span-2'>
              <FormLabel>{t('Source')}</FormLabel>
              <FormControl>
                <Select
                  items={sourceItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value !== 'direct' && value !== 'channel') return
                    if (value === field.value) return
                    field.onChange(value)
                    form.setValue(
                      profileFieldPath(props.profile, 'channelModel'),
                      '',
                      { shouldDirty: true }
                    )
                    setFetchedModels([])
                    setFetchState({ status: 'idle' })
                  }}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {sourceItems.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </FormControl>
              <FormDescription>
                {t(
                  'Direct calls an OpenAI-compatible endpoint. Channel reuses a configured upstream channel.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        {source === 'direct' ? (
          <>
            <FormField
              control={form.control}
              name={profileFieldPath(props.profile, 'baseUrl')}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Base URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://api.openai.com/v1'
                      autoComplete='off'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'OpenAI-compatible API root, for example https://api.openai.com/v1'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name={profileFieldPath(props.profile, 'model')}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Model')}</FormLabel>
                  <div className='flex flex-wrap items-center gap-2'>
                    <FormControl>
                      <Input
                        placeholder='gpt-4.1-mini'
                        autoComplete='off'
                        className='min-w-40 flex-1'
                        list={modelListId}
                        {...field}
                      />
                    </FormControl>
                    <FetchModelsButton
                      isFetching={isFetching}
                      disabled={props.isBusy}
                      onClick={() => {
                        void handleFetchModels()
                      }}
                    />
                  </div>
                  <datalist id={modelListId}>
                    {modelSuggestions.map((modelName) => (
                      <option key={modelName} value={modelName} />
                    ))}
                  </datalist>
                  <FormDescription>
                    {t('Model name used for assistant conversations')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </>
        ) : (
          <>
            <FormField
              control={form.control}
              name={profileFieldPath(props.profile, 'channelId')}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Channel')}</FormLabel>
                  <FormControl>
                    <Select
                      items={channelItems}
                      value={field.value > 0 ? field.value : null}
                      onValueChange={(value) => {
                        const nextChannelId =
                          typeof value === 'number' ? value : 0
                        field.onChange(nextChannelId)
                        if (nextChannelId !== field.value) {
                          form.setValue(
                            profileFieldPath(props.profile, 'channelModel'),
                            '',
                            { shouldDirty: true }
                          )
                          setFetchedModels([])
                          setFetchState({ status: 'idle' })
                        }
                      }}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue placeholder={t('Select a channel')} />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {props.channels.map((channel) => {
                            const statusLabel =
                              CHANNEL_STATUS_LABELS[
                                channel.status as keyof typeof CHANNEL_STATUS_LABELS
                              ] ?? 'Unknown'
                            return (
                              <SelectItem key={channel.id} value={channel.id}>
                                <span className='min-w-0 flex-1 truncate'>
                                  {channel.name}
                                </span>
                                <span className='text-muted-foreground text-xs'>
                                  #{channel.id}
                                </span>
                                <span className='text-muted-foreground text-xs'>
                                  {channel.type_name}
                                </span>
                                {channel.status ===
                                CHANNEL_STATUS.ENABLED ? null : (
                                  <Badge variant='outline'>
                                    {t(statusLabel)}
                                  </Badge>
                                )}
                              </SelectItem>
                            )
                          })}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Bind an OpenAI-compatible channel that supports the Responses API. Disabled channels are allowed; use Test connection to verify.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name={profileFieldPath(props.profile, 'channelModel')}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Channel model')}</FormLabel>
                  <div className='flex flex-wrap items-center gap-2'>
                    <FormControl>
                      <Input
                        placeholder='gpt-4.1-mini'
                        autoComplete='off'
                        className='min-w-40 flex-1'
                        list={modelListId}
                        {...field}
                      />
                    </FormControl>
                    <FetchModelsButton
                      isFetching={isFetching}
                      disabled={props.isBusy}
                      onClick={() => {
                        void handleFetchModels()
                      }}
                    />
                  </div>
                  <datalist id={modelListId}>
                    {modelSuggestions.map((modelName) => (
                      <option key={modelName} value={modelName} />
                    ))}
                  </datalist>
                  <FormDescription>
                    {t('Model name served by the selected channel')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </>
        )}

        <FormField
          control={form.control}
          name={profileFieldPath(props.profile, 'protocol')}
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Protocol')}</FormLabel>
              <FormControl>
                <Select
                  items={protocolItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value !== 'responses' && value !== 'chat') return
                    field.onChange(value)
                  }}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {protocolItems.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </FormControl>
              <FormDescription>
                {t('API protocol used to call the upstream endpoint.')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name={profileFieldPath(props.profile, 'contextWindow')}
          render={({ field }) => {
            const resolved = parseTokenAmount(field.value)
            return (
              <FormItem>
                <FormLabel>{t('Context window')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='1000000, 1M, 256k'
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                {resolved === null ? null : (
                  <p className='text-muted-foreground text-xs'>
                    {t('Resolved value: {{tokens}} tokens', {
                      tokens: resolved,
                    })}
                  </p>
                )}
                <FormDescription>
                  {t(
                    'Maximum context window of the model used by this profile.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )
          }}
        />

        <FormField
          control={form.control}
          name={profileFieldPath(props.profile, 'maxOutputTokens')}
          render={({ field }) => {
            const resolved = parseTokenAmount(field.value)
            return (
              <FormItem>
                <FormLabel>{t('Max output tokens')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='128000, 128k'
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                {resolved === null ? null : (
                  <p className='text-muted-foreground text-xs'>
                    {t('Resolved value: {{tokens}} tokens', {
                      tokens: resolved,
                    })}
                  </p>
                )}
                <FormDescription>
                  {t('Must be less than the context window')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )
          }}
        />

        <FormField
          control={form.control}
          name={profileFieldPath(props.profile, 'apiKey')}
          render={({ field }) => (
            <FormItem className='sm:col-span-2'>
              <FormLabel className='flex items-center gap-2'>
                <span>{t('API Key')}</span>
                {props.apiKeyConfigured ? (
                  <Badge variant='secondary'>{t('Configured')}</Badge>
                ) : null}
              </FormLabel>
              <div className='flex flex-wrap items-center gap-2'>
                <FormControl>
                  <Input
                    type='password'
                    placeholder={t('Enter API Key')}
                    autoComplete='off'
                    className='min-w-40 flex-1'
                    {...field}
                  />
                </FormControl>
                <Button
                  type='button'
                  variant='secondary'
                  onClick={() => {
                    void handleTestConnection()
                  }}
                  disabled={testState.status === 'testing' || props.isBusy}
                  className='shrink-0'
                >
                  {testState.status === 'testing' ? (
                    <Loader2 className='me-2 size-4 animate-spin' />
                  ) : null}
                  {t('Test Connection')}
                </Button>
                {props.apiKeyConfigured ? (
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => setClearDialogOpen(true)}
                    disabled={props.isBusy || isClearing}
                    className='shrink-0'
                  >
                    {t('Clear API Key')}
                  </Button>
                ) : null}
              </div>
              <FormDescription>{apiKeyDescription}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      {fetchState.status === 'error' ? (
        <p role='alert' className='text-destructive text-sm'>
          {fetchState.message}
        </p>
      ) : null}

      {testState.status === 'success' ? (
        <Alert variant='default' className='flex items-center gap-2'>
          <CheckCircle2 className='size-4 text-green-600' />
          <div className='min-w-0'>
            <AlertTitle>{t('Connection successful')}</AlertTitle>
            {testState.latencyMs === undefined ? null : (
              <AlertDescription>
                {t('Connected in {{latency}} ms', {
                  latency: testState.latencyMs,
                })}
              </AlertDescription>
            )}
          </div>
        </Alert>
      ) : null}

      {testState.status === 'error' ? (
        <Alert variant='destructive' className='flex items-center gap-2'>
          <XCircle className='size-4' />
          <div className='min-w-0'>
            <AlertTitle>{t('Connection failed')}</AlertTitle>
            <AlertDescription>{testState.message}</AlertDescription>
          </div>
        </Alert>
      ) : null}

      <AlertDialog open={clearDialogOpen} onOpenChange={setClearDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Clear API Key?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This removes the stored API key and disables the assistant. This action cannot be undone.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isClearing}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() => {
                void handleClearKey()
              }}
              disabled={isClearing}
            >
              {isClearing ? (
                <Loader2 className='me-2 size-4 animate-spin' />
              ) : null}
              {t('Clear')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  )
}
