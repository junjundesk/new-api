import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import i18next from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { GroupModelDescriptionsDialog } from '../group-model-descriptions-dialog'

const i18n = i18next.createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })

const members = [
  { model_name: 'claude', description: '', group_description: 'claude 描述' },
  { model_name: 'gpt-4o', description: '全局描述', group_description: '' },
]

function renderDialog(props: {
  value?: string
  onApply?: (group: string, descriptions: Record<string, string>) => void
}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onApply = props.onApply ?? vi.fn()
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <GroupModelDescriptionsDialog
          groupName='vip'
          value={props.value ?? '{"vip":{"claude":"claude 描述"}}'}
          onOpenChange={() => {}}
          onApply={onApply}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return { onApply }
}

describe('group model descriptions dialog', () => {
  let getSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    getSpy = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: members } })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('shows the saved group description and applies the edited one', async () => {
    const { onApply } = renderDialog({})

    const input = (await screen.findByLabelText('gpt-4o')) as HTMLInputElement
    expect(
      (screen.getByLabelText('claude') as HTMLInputElement).value
    ).toBe('claude 描述')

    fireEvent.change(input, { target: { value: 'vip 专享' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    expect(getSpy).toHaveBeenCalledWith('/api/models/group_members', {
      params: { group: 'vip' },
    })
    expect(onApply).toHaveBeenCalledWith('vip', {
      claude: 'claude 描述',
      'gpt-4o': 'vip 专享',
    })
  })

  it('drops a description cleared by the admin', async () => {
    const { onApply } = renderDialog({})

    const input = (await screen.findByLabelText('claude')) as HTMLInputElement
    fireEvent.change(input, { target: { value: '   ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    expect(onApply).toHaveBeenCalledWith('vip', {})
  })

  it('filters models by the search term', async () => {
    renderDialog({})

    await screen.findByLabelText('gpt-4o')
    fireEvent.change(screen.getByLabelText('Search model'), {
      target: { value: 'claude' },
    })

    await waitFor(() => expect(screen.queryByLabelText('gpt-4o')).toBeNull())
    expect(screen.getByLabelText('claude')).toBeTruthy()
  })

  it('shows the empty state when the group has no models', async () => {
    getSpy.mockResolvedValue({ data: { success: true, data: [] } })

    renderDialog({})

    await waitFor(() =>
      expect(screen.getByText('No models in this group yet')).toBeTruthy()
    )
  })
})
