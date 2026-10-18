import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { useState } from 'react'
import { I18nextProvider } from 'react-i18next'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SettingsPageProvider } from '@/features/system-settings/components/settings-page-context'
import { api } from '@/lib/api'

import type { AssistantChannel } from '../assistant-settings-api'
import {
  AssistantSettingsSection,
  type AssistantSettingsDefaults,
} from '../assistant-settings-section'

const i18n = i18next.createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Connected in {{latency}} ms': 'Connected in {{latency}} ms',
      },
    },
  },
})

const baseDefaults: AssistantSettingsDefaults = {
  user: {
    enabled: false,
    source: 'direct',
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-4.1-mini',
    channelId: 0,
    channelModel: '',
    apiKeyConfigured: false,
  },
  admin: {
    enabled: false,
    source: 'direct',
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-4.1-mini',
    channelId: 0,
    channelModel: '',
    apiKeyConfigured: false,
  },
  systemPrompt: '',
  maxToolRounds: 5,
  requestTimeoutSeconds: 60,
  maxOutputTokens: 1500,
  dailyUserMessageLimit: 50,
  allowTokenActions: true,
}

const channels: AssistantChannel[] = [
  {
    id: 1,
    name: 'Primary',
    type: 1,
    type_name: 'OpenAI',
    status: 1,
    base_url: 'https://api.openai.com',
    models: ['gpt-4.1-mini', 'gpt-4o'],
  },
  {
    id: 2,
    name: 'Backup',
    type: 3,
    type_name: 'Azure',
    status: 2,
    base_url: 'https://azure.example.com',
    models: ['azure-gpt-4o'],
  },
]

const testRequestConfig = {
  skipErrorHandler: true,
  skipBusinessError: true,
}

function SettingsHarness(props: { defaultValues: AssistantSettingsDefaults }) {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)

  return (
    <>
      <div ref={setActionsContainer} />
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <AssistantSettingsSection defaultValues={props.defaultValues} />
      </SettingsPageProvider>
    </>
  )
}

function renderSection(overrides: Partial<AssistantSettingsDefaults> = {}) {
  const defaultValues: AssistantSettingsDefaults = {
    ...baseDefaults,
    ...overrides,
    user: { ...baseDefaults.user, ...overrides.user },
    admin: { ...baseDefaults.admin, ...overrides.admin },
  }
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <SettingsHarness defaultValues={defaultValues} />
      </I18nextProvider>
    </QueryClientProvider>
  )

  return { queryClient, invalidateSpy }
}

function getProfileCard(name: 'User assistant' | 'Admin assistant') {
  return screen.getByRole('region', { name })
}

function getModelSuggestions(input: HTMLElement): string[] {
  const listId = input.getAttribute('list')
  if (!listId) return []
  const datalist = document.querySelector<HTMLDataListElement>(
    `datalist#${listId}`
  )
  if (!datalist) return []
  return [...datalist.querySelectorAll('option')].map((option) => option.value)
}

type TestUser = ReturnType<typeof userEvent.setup>

async function selectComboboxOption(
  user: TestUser,
  combobox: HTMLElement,
  optionName: string | RegExp
) {
  await user.click(combobox)
  await user.click(await screen.findByRole('option', { name: optionName }))
}

async function switchUserProfileToChannel(user: TestUser) {
  const userCard = getProfileCard('User assistant')
  await selectComboboxOption(
    user,
    within(userCard).getByRole('combobox', { name: 'Source' }),
    'Channel'
  )
  return within(userCard).getByRole('combobox', { name: 'Channel' })
}

describe('assistant settings section', () => {
  let putSpy: ReturnType<typeof vi.spyOn>
  let postSpy: ReturnType<typeof vi.spyOn>
  let deleteSpy: ReturnType<typeof vi.spyOn>
  let successSpy: ReturnType<typeof vi.spyOn>
  let errorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, message: '', data: channels },
    })
    putSpy = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true, message: '' } })
    postSpy = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { success: true, message: '' } })
    deleteSpy = vi
      .spyOn(api, 'delete')
      .mockResolvedValue({ data: { success: true, message: '' } })
    successSpy = vi.spyOn(toast, 'success').mockImplementation(() => '')
    errorSpy = vi.spyOn(toast, 'error').mockImplementation(() => '')
    vi.spyOn(toast, 'info').mockImplementation(() => '')
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('saves shared fields first, then each profile with its enabled flag last', async () => {
    renderSection()

    const userCard = getProfileCard('User assistant')
    const adminCard = getProfileCard('Admin assistant')

    fireEvent.change(screen.getByLabelText('System Prompt'), {
      target: { value: 'Be concise.' },
    })
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Max tool rounds' }),
      { target: { value: '6' } }
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Request timeout (seconds)' }),
      { target: { value: '90' } }
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Max output tokens' }),
      { target: { value: '2000' } }
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Daily message limit per user' }),
      { target: { value: '70' } }
    )
    fireEvent.click(screen.getByRole('switch', { name: 'Allow token actions' }))

    fireEvent.change(within(userCard).getByLabelText(/API Key/), {
      target: { value: ' sk-test ' },
    })
    fireEvent.change(within(userCard).getByLabelText('Base URL'), {
      target: { value: 'https://proxy.example.com/v1' },
    })
    fireEvent.change(within(userCard).getByLabelText('Model'), {
      target: { value: 'gpt-4o' },
    })
    fireEvent.click(within(userCard).getByRole('switch', { name: 'Enabled' }))

    fireEvent.change(within(adminCard).getByLabelText(/API Key/), {
      target: { value: 'sk-admin' },
    })
    fireEvent.change(within(adminCard).getByLabelText('Base URL'), {
      target: { value: 'https://admin.example.com/v1' },
    })
    fireEvent.change(within(adminCard).getByLabelText('Model'), {
      target: { value: 'gpt-4.1' },
    })
    fireEvent.click(within(adminCard).getByRole('switch', { name: 'Enabled' }))

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(putSpy).toHaveBeenCalledTimes(14))
    expect(putSpy.mock.calls).toEqual([
      [
        '/api/option/',
        { key: 'assistant_setting.system_prompt', value: 'Be concise.' },
      ],
      ['/api/option/', { key: 'assistant_setting.max_tool_rounds', value: 6 }],
      [
        '/api/option/',
        { key: 'assistant_setting.request_timeout_seconds', value: 90 },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.max_output_tokens', value: 2000 },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.daily_user_message_limit', value: 70 },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.allow_token_actions', value: false },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.user_api_key', value: 'sk-test' },
      ],
      [
        '/api/option/',
        {
          key: 'assistant_setting.user_base_url',
          value: 'https://proxy.example.com/v1',
        },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.user_model', value: 'gpt-4o' },
      ],
      ['/api/option/', { key: 'assistant_setting.user_enabled', value: true }],
      [
        '/api/option/',
        { key: 'assistant_setting.admin_api_key', value: 'sk-admin' },
      ],
      [
        '/api/option/',
        {
          key: 'assistant_setting.admin_base_url',
          value: 'https://admin.example.com/v1',
        },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.admin_model', value: 'gpt-4.1' },
      ],
      ['/api/option/', { key: 'assistant_setting.admin_enabled', value: true }],
    ])

    await waitFor(() => expect(successSpy).toHaveBeenCalledTimes(1))
    expect(
      (within(userCard).getByLabelText(/API Key/) as HTMLInputElement).value
    ).toBe('')
    expect(
      (within(adminCard).getByLabelText(/API Key/) as HTMLInputElement).value
    ).toBe('')
  })

  it('skips an empty API key when other profile fields change', async () => {
    renderSection({ user: { ...baseDefaults.user, apiKeyConfigured: true } })

    const userCard = getProfileCard('User assistant')
    fireEvent.change(within(userCard).getByLabelText('Model'), {
      target: { value: 'gpt-4o-mini' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(putSpy).toHaveBeenCalledTimes(1))
    expect(putSpy.mock.calls).toEqual([
      [
        '/api/option/',
        { key: 'assistant_setting.user_model', value: 'gpt-4o-mini' },
      ],
    ])
  })

  it('clears only the confirmed profile API key', async () => {
    const { invalidateSpy } = renderSection({
      user: { ...baseDefaults.user, apiKeyConfigured: true },
      admin: { ...baseDefaults.admin, apiKeyConfigured: true },
    })

    const userCard = getProfileCard('User assistant')
    const adminCard = getProfileCard('Admin assistant')

    fireEvent.change(within(userCard).getByLabelText(/API Key/), {
      target: { value: 'sk-unsaved' },
    })
    fireEvent.click(
      within(userCard).getByRole('button', { name: 'Clear API Key' })
    )
    expect(screen.getByText('Clear API Key?')).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByText('Clear API Key?')).toBeNull())
    expect(deleteSpy).not.toHaveBeenCalled()
    expect(within(userCard).getByText('Configured')).toBeVisible()

    fireEvent.click(
      within(userCard).getByRole('button', { name: 'Clear API Key' })
    )
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))

    await waitFor(() =>
      expect(deleteSpy).toHaveBeenCalledWith('/api/assistant/admin/key', {
        params: { profile: 'user' },
        ...testRequestConfig,
      })
    )
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: ['system-options'],
      })
    )
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['status'] })
    await waitFor(() => expect(successSpy).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(within(userCard).queryByText('Configured')).toBeNull()
    )
    expect(
      (within(userCard).getByLabelText(/API Key/) as HTMLInputElement).value
    ).toBe('')
    expect(within(adminCard).getByText('Configured')).toBeVisible()
  })

  it('shows a successful test connection inline with its latency', async () => {
    postSpy.mockResolvedValue({
      data: {
        success: true,
        message: '',
        data: { latency_ms: 123, model: 'gpt-4.1-mini', text: 'pong' },
      },
    })
    renderSection()

    const userCard = getProfileCard('User assistant')
    fireEvent.change(within(userCard).getByLabelText(/API Key/), {
      target: { value: ' sk-test ' },
    })
    fireEvent.click(
      within(userCard).getByRole('button', { name: 'Test Connection' })
    )

    expect(await screen.findByText('Connection successful')).toBeVisible()
    expect(screen.getByText('Connected in 123 ms')).toBeVisible()
    expect(postSpy).toHaveBeenCalledWith(
      '/api/assistant/admin/test',
      {
        profile: 'user',
        source: 'direct',
        base_url: 'https://api.openai.com/v1',
        model: 'gpt-4.1-mini',
        api_key: 'sk-test',
      },
      testRequestConfig
    )
    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('shows a failed test connection inline without a toast', async () => {
    postSpy.mockResolvedValue({
      data: { success: false, message: 'Invalid API key' },
    })
    renderSection({ user: { ...baseDefaults.user, apiKeyConfigured: true } })

    const userCard = getProfileCard('User assistant')
    fireEvent.click(
      within(userCard).getByRole('button', { name: 'Test Connection' })
    )

    expect(await screen.findByText('Invalid API key')).toBeVisible()
    expect(screen.getByText('Connection failed')).toBeVisible()
    expect(screen.queryByText('Connection successful')).toBeNull()
    expect(postSpy).toHaveBeenCalledWith(
      '/api/assistant/admin/test',
      {
        profile: 'user',
        source: 'direct',
        base_url: 'https://api.openai.com/v1',
        model: 'gpt-4.1-mini',
      },
      testRequestConfig
    )
    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('loads the selected channel models into the model suggestions', async () => {
    const user = userEvent.setup()
    renderSection()

    const channelSelect = await switchUserProfileToChannel(user)
    await selectComboboxOption(user, channelSelect, /Primary/)

    const userCard = getProfileCard('User assistant')
    const modelInput = within(userCard).getByRole('combobox', {
      name: 'Channel model',
    })
    await waitFor(() =>
      expect(getModelSuggestions(modelInput)).toEqual([
        'gpt-4.1-mini',
        'gpt-4o',
      ])
    )

    fireEvent.change(modelInput, { target: { value: 'custom-model' } })
    expect((modelInput as HTMLInputElement).value).toBe('custom-model')
  })

  it('allows selecting a disabled channel and offers its models', async () => {
    const user = userEvent.setup()
    renderSection()

    const channelSelect = await switchUserProfileToChannel(user)
    await selectComboboxOption(user, channelSelect, /Backup/)

    expect(channelSelect).toHaveTextContent('Backup')
    expect(channelSelect).toHaveTextContent('Disabled')

    const userCard = getProfileCard('User assistant')
    const modelInput = within(userCard).getByRole('combobox', {
      name: 'Channel model',
    })
    await waitFor(() =>
      expect(getModelSuggestions(modelInput)).toEqual(['azure-gpt-4o'])
    )
  })

  it('blocks saving an enabled channel profile without a channel and model', async () => {
    const user = userEvent.setup()
    renderSection()

    await switchUserProfileToChannel(user)
    const userCard = getProfileCard('User assistant')
    fireEvent.click(within(userCard).getByRole('switch', { name: 'Enabled' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(await screen.findByText('Channel is required')).toBeVisible()
    expect(screen.getByText('Channel model is required')).toBeVisible()
    expect(putSpy).not.toHaveBeenCalled()
  })
})
