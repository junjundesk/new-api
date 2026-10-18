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
  type AssistantProfileDefaults,
  type AssistantSettingsDefaults,
} from '../assistant-settings-section'

const i18n = i18next.createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Connected in {{latency}} ms': 'Connected in {{latency}} ms',
        'Resolved value: {{tokens}} tokens':
          'Resolved value: {{tokens}} tokens',
      },
    },
  },
})

const baseProfileDefaults: AssistantProfileDefaults = {
  enabled: false,
  source: 'direct',
  protocol: 'responses',
  baseUrl: 'https://api.openai.com/v1',
  model: 'gpt-4.1-mini',
  channelId: 0,
  channelModel: '',
  apiKeyConfigured: false,
  contextWindow: 1000000,
  maxOutputTokens: 128000,
}

const baseDefaults: AssistantSettingsDefaults = {
  enabled: true,
  user: { ...baseProfileDefaults },
  admin: { ...baseProfileDefaults },
  systemPrompt: '',
  requestTimeoutSeconds: 60,
  confirmWrites: true,
  adminDbToolsEnabled: true,
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

  it('saves master first, then shared fields, then each profile with its enabled flag last', async () => {
    const user = userEvent.setup()
    renderSection()

    const userCard = getProfileCard('User assistant')
    const adminCard = getProfileCard('Admin assistant')

    fireEvent.click(screen.getByRole('switch', { name: 'Enable AI Assistant' }))

    fireEvent.change(screen.getByLabelText('System Prompt'), {
      target: { value: 'Be concise.' },
    })
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Request timeout (seconds)' }),
      { target: { value: '90' } }
    )
    fireEvent.click(
      screen.getByRole('switch', {
        name: 'Require confirmation for AI write operations',
      })
    )
    fireEvent.click(
      screen.getByRole('switch', { name: 'Allow root database tools' })
    )

    await selectComboboxOption(
      user,
      within(userCard).getByRole('combobox', { name: 'Protocol' }),
      'OpenAI Chat'
    )
    fireEvent.change(
      within(userCard).getByRole('textbox', { name: 'Context window' }),
      { target: { value: '256k' } }
    )
    fireEvent.change(
      within(userCard).getByRole('textbox', { name: 'Max output tokens' }),
      { target: { value: '64k' } }
    )
    expect(
      within(userCard).getByText('Resolved value: 256000 tokens')
    ).toBeVisible()
    expect(
      within(userCard).getByText('Resolved value: 64000 tokens')
    ).toBeVisible()
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

    fireEvent.change(
      within(adminCard).getByRole('textbox', { name: 'Context window' }),
      { target: { value: '500000' } }
    )
    fireEvent.change(
      within(adminCard).getByRole('textbox', { name: 'Max output tokens' }),
      { target: { value: '8192' } }
    )
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

    await waitFor(() => expect(putSpy).toHaveBeenCalledTimes(18))
    expect(putSpy.mock.calls).toEqual([
      ['/api/option/', { key: 'assistant_setting.enabled', value: false }],
      [
        '/api/option/',
        { key: 'assistant_setting.system_prompt', value: 'Be concise.' },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.request_timeout_seconds', value: 90 },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.confirm_writes', value: false },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.admin_db_tools_enabled', value: false },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.user_protocol', value: 'chat' },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.user_context_window', value: 256000 },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.user_max_output_tokens', value: 64000 },
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
        { key: 'assistant_setting.admin_context_window', value: 500000 },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.admin_max_output_tokens', value: 8192 },
      ],
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

  it('shows the master and safety switches on by default', () => {
    renderSection()

    expect(screen.getAllByRole('switch')).toHaveLength(5)
    expect(
      screen.getByRole('switch', { name: 'Enable AI Assistant' })
    ).toBeChecked()
    expect(
      screen.getByRole('switch', {
        name: 'Require confirmation for AI write operations',
      })
    ).toBeChecked()
    expect(
      screen.getByRole('switch', { name: 'Allow root database tools' })
    ).toBeChecked()
    expect(
      within(getProfileCard('User assistant')).getByRole('switch', {
        name: 'Enabled',
      })
    ).not.toBeChecked()
    expect(
      within(getProfileCard('Admin assistant')).getByRole('switch', {
        name: 'Enabled',
      })
    ).not.toBeChecked()
  })

  it('saves the master switch off as false before any other option', async () => {
    renderSection()

    fireEvent.click(screen.getByRole('switch', { name: 'Enable AI Assistant' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(putSpy).toHaveBeenCalledTimes(1))
    expect(putSpy.mock.calls).toEqual([
      ['/api/option/', { key: 'assistant_setting.enabled', value: false }],
    ])
    await waitFor(() => expect(successSpy).toHaveBeenCalledTimes(1))
  })

  it('saves the master switch off without validating incomplete enabled profiles', async () => {
    renderSection({
      user: {
        ...baseProfileDefaults,
        enabled: true,
        baseUrl: '',
        model: '',
      },
    })

    fireEvent.change(screen.getByLabelText('System Prompt'), {
      target: { value: 'Paused.' },
    })
    fireEvent.click(screen.getByRole('switch', { name: 'Enable AI Assistant' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(putSpy).toHaveBeenCalledTimes(2))
    expect(putSpy.mock.calls).toEqual([
      ['/api/option/', { key: 'assistant_setting.enabled', value: false }],
      [
        '/api/option/',
        { key: 'assistant_setting.system_prompt', value: 'Paused.' },
      ],
    ])
    await waitFor(() => expect(successSpy).toHaveBeenCalledTimes(1))
    expect(screen.queryByText('Base URL is required')).toBeNull()
  })

  it('saves safety switches with the master switch off while profiles are invalid', async () => {
    renderSection({
      user: {
        ...baseProfileDefaults,
        enabled: true,
        baseUrl: '',
        model: '',
      },
    })

    fireEvent.click(screen.getByRole('switch', { name: 'Enable AI Assistant' }))
    fireEvent.click(
      screen.getByRole('switch', {
        name: 'Require confirmation for AI write operations',
      })
    )
    fireEvent.click(
      screen.getByRole('switch', { name: 'Allow root database tools' })
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(putSpy).toHaveBeenCalledTimes(3))
    expect(putSpy.mock.calls).toEqual([
      ['/api/option/', { key: 'assistant_setting.enabled', value: false }],
      [
        '/api/option/',
        { key: 'assistant_setting.confirm_writes', value: false },
      ],
      [
        '/api/option/',
        { key: 'assistant_setting.admin_db_tools_enabled', value: false },
      ],
    ])
    await waitFor(() => expect(successSpy).toHaveBeenCalledTimes(1))
    expect(screen.queryByText('Base URL is required')).toBeNull()
  })

  it('blocks saving an incomplete enabled profile while the master switch is on', async () => {
    renderSection({
      user: {
        ...baseProfileDefaults,
        enabled: true,
        baseUrl: '',
        model: '',
      },
    })

    fireEvent.change(screen.getByLabelText('System Prompt'), {
      target: { value: 'Be concise.' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(await screen.findByText('Base URL is required')).toBeVisible()
    expect(screen.getByText('Model is required')).toBeVisible()
    expect(putSpy).not.toHaveBeenCalled()
  })

  it('skips an empty API key when other profile fields change', async () => {
    renderSection({ user: { ...baseProfileDefaults, apiKeyConfigured: true } })

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
      user: { ...baseProfileDefaults, apiKeyConfigured: true },
      admin: { ...baseProfileDefaults, apiKeyConfigured: true },
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
        protocol: 'responses',
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
    renderSection({ user: { ...baseProfileDefaults, apiKeyConfigured: true } })

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
        protocol: 'responses',
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
    expect(
      screen.getByRole('switch', { name: 'Enable AI Assistant' })
    ).toBeChecked()
    expect(putSpy).not.toHaveBeenCalled()
  })

  it('fetches models for the direct profile and offers them without selecting one', async () => {
    postSpy.mockResolvedValue({
      data: {
        success: true,
        message: '',
        data: { models: ['gpt-4o', 'o3'] },
      },
    })
    renderSection()

    const userCard = getProfileCard('User assistant')
    const modelInput = within(userCard).getByRole('combobox', {
      name: 'Model',
    })
    fireEvent.click(
      within(userCard).getByRole('button', { name: 'Fetch models' })
    )

    await waitFor(() =>
      expect(getModelSuggestions(modelInput)).toEqual(['gpt-4o', 'o3'])
    )
    expect(postSpy).toHaveBeenCalledWith(
      '/api/assistant/admin/models',
      {
        profile: 'user',
        source: 'direct',
        protocol: 'responses',
        base_url: 'https://api.openai.com/v1',
        model: 'gpt-4.1-mini',
        context_window: 1000000,
        max_output_tokens: 128000,
      },
      testRequestConfig
    )
    expect((modelInput as HTMLInputElement).value).toBe('gpt-4.1-mini')
  })

  it('merges fetched models with the selected channel models', async () => {
    const user = userEvent.setup()
    postSpy.mockResolvedValue({
      data: {
        success: true,
        message: '',
        data: { models: ['gpt-4o', 'extra-model'] },
      },
    })
    renderSection()

    const channelSelect = await switchUserProfileToChannel(user)
    await selectComboboxOption(user, channelSelect, /Primary/)

    const userCard = getProfileCard('User assistant')
    const modelInput = within(userCard).getByRole('combobox', {
      name: 'Channel model',
    })
    fireEvent.click(
      within(userCard).getByRole('button', { name: 'Fetch models' })
    )

    await waitFor(() =>
      expect(getModelSuggestions(modelInput)).toEqual([
        'gpt-4.1-mini',
        'gpt-4o',
        'extra-model',
      ])
    )
    expect(postSpy).toHaveBeenCalledWith(
      '/api/assistant/admin/models',
      {
        profile: 'user',
        source: 'channel',
        protocol: 'responses',
        channel_id: 1,
        channel_model: '',
        context_window: 1000000,
        max_output_tokens: 128000,
      },
      testRequestConfig
    )
  })

  it('shows a failed model fetch inline without a toast or suggestions', async () => {
    postSpy.mockResolvedValue({
      data: { success: false, message: 'No API key configured' },
    })
    renderSection()

    const userCard = getProfileCard('User assistant')
    const modelInput = within(userCard).getByRole('combobox', {
      name: 'Model',
    })
    fireEvent.click(
      within(userCard).getByRole('button', { name: 'Fetch models' })
    )

    expect(await screen.findByText('No API key configured')).toBeVisible()
    expect(errorSpy).not.toHaveBeenCalled()
    expect(getModelSuggestions(modelInput)).toEqual([])
  })

  it('blocks saving when max output tokens are not below the context window', async () => {
    renderSection()

    const userCard = getProfileCard('User assistant')
    fireEvent.change(
      within(userCard).getByRole('textbox', { name: 'Context window' }),
      { target: { value: '200000' } }
    )
    fireEvent.change(
      within(userCard).getByRole('textbox', { name: 'Max output tokens' }),
      { target: { value: '200000' } }
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => {
      const messages = within(userCard).getAllByText(
        'Max output tokens must be less than the context window'
      )
      expect(messages.length).toBeGreaterThan(0)
    })
    expect(putSpy).not.toHaveBeenCalled()
  })

  it('parses human-readable token amounts and shows the resolved count', () => {
    renderSection()

    const userCard = getProfileCard('User assistant')
    const contextInput = within(userCard).getByRole('textbox', {
      name: 'Context window',
    })
    fireEvent.change(contextInput, { target: { value: '1M' } })
    expect(
      within(userCard).getByText('Resolved value: 1000000 tokens')
    ).toBeVisible()

    fireEvent.change(contextInput, { target: { value: '1000000' } })
    expect(
      within(userCard).getByText('Resolved value: 1000000 tokens')
    ).toBeVisible()

    const maxOutputInput = within(userCard).getByRole('textbox', {
      name: 'Max output tokens',
    })
    fireEvent.change(maxOutputInput, { target: { value: '128k' } })
    expect(
      within(userCard).getByText('Resolved value: 128000 tokens')
    ).toBeVisible()

    fireEvent.change(maxOutputInput, { target: { value: '256k' } })
    expect(
      within(userCard).getByText('Resolved value: 256000 tokens')
    ).toBeVisible()
  })

  it('blocks saving an unparseable token amount', async () => {
    renderSection()

    const userCard = getProfileCard('User assistant')
    fireEvent.change(
      within(userCard).getByRole('textbox', { name: 'Context window' }),
      { target: { value: 'lots' } }
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() =>
      expect(within(userCard).getByText('Value is out of range')).toBeVisible()
    )
    expect(putSpy).not.toHaveBeenCalled()
  })
})
