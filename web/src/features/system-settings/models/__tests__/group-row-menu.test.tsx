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

import { GroupRatioVisualEditor } from '../group-ratio-visual-editor'

const i18n = i18next.createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })

function renderEditor(props: {
  onChange: (field: string, value: string) => void
}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <GroupRatioVisualEditor
          groupRatio='{"vip":1}'
          topupGroupRatio='{}'
          userUsableGroups='{}'
          groupGroupRatio='{}'
          autoGroups='[]'
          maxTokenAutoGroupsField={null}
          groupSpecialUsableGroup='{}'
          groupModelDescriptions='{"vip":{"gpt-4o":"旧描述"}}'
          onChange={props.onChange}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

async function openRowMenu() {
  fireEvent.click(screen.getByRole('button', { name: 'Open menu' }))
  return screen.findByText('Model descriptions')
}

describe('group row menu', () => {
  let getSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    getSpy = vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: [
          { model_name: 'gpt-4o', description: '', group_description: '旧描述' },
        ],
      },
    })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('opens the model description dialog for the row group', async () => {
    renderEditor({ onChange: vi.fn() })

    const item = await openRowMenu()
    fireEvent.click(item)

    // The dialog loads the models of the group the menu was opened on.
    expect(await screen.findByLabelText('gpt-4o')).toBeTruthy()
    expect(getSpy).toHaveBeenCalledWith('/api/models/group_members', {
      params: { group: 'vip' },
    })
  })

  it('writes the applied descriptions into the group form value', async () => {
    const onChange = vi.fn()
    renderEditor({ onChange })

    fireEvent.click(await openRowMenu())
    const input = (await screen.findByLabelText('gpt-4o')) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'VIP 描述' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(onChange).toHaveBeenCalled())
    const call = onChange.mock.calls.find(
      (entry) => entry[0] === 'GroupModelDescriptions'
    )
    expect(call).toBeTruthy()
    expect(JSON.parse(String(call?.[1]))).toEqual({
      vip: { 'gpt-4o': 'VIP 描述' },
    })
  })
})
