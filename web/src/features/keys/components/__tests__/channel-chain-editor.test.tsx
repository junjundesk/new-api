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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createChannelChain,
  getGroupModelNames,
  updateChannelChain,
} from '../../api'
import type { ChannelChain } from '../../types'
import type { ApiKeyGroupOption } from '../api-key-group-combobox'
import {
  ChannelChainEditor,
  type ChannelChainEditorHandle,
} from '../channel-chain-editor'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      let text = key
      for (const [name, value] of Object.entries(options ?? {})) {
        text = text.replaceAll(`{{${name}}}`, String(value))
      }
      return text
    },
  }),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('../../api', () => ({
  createChannelChain: vi.fn(),
  updateChannelChain: vi.fn(),
  getGroupModelNames: vi.fn(),
}))

const groupOptions: ApiKeyGroupOption[] = [
  { value: 'vip', label: 'vip (2x)', ratio: 2 },
  { value: 'default', label: 'default (1x)', ratio: 1 },
]

const modelsChain: ChannelChain = {
  id: 'chain_1',
  name: 'Routing',
  type: 'models',
  groups: ['vip'],
  items: [{ group: 'vip', models: ['gpt-4o'] }],
}

function renderEditor(overrides: {
  mode?: 'create' | 'edit'
  chain?: ChannelChain
  groupOptionsReady?: boolean
}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const ref = createRef<ChannelChainEditorHandle>()
  const onSaved = vi.fn()
  const view = render(
    <QueryClientProvider client={client}>
      <ChannelChainEditor
        ref={ref}
        mode={overrides.mode ?? 'create'}
        chain={overrides.chain}
        open
        groupOptions={groupOptions}
        groupOptionsReady={overrides.groupOptionsReady}
        onSaved={onSaved}
        onCancel={vi.fn()}
        onSubmittingChange={vi.fn()}
      />
    </QueryClientProvider>
  )
  return { ...view, ref, onSaved }
}

async function selectMultiSelectOption(inputLabel: string, optionText: string) {
  const input = screen.getByLabelText(inputLabel)
  fireEvent.mouseDown(input)
  const option = await screen.findByText(optionText)
  fireEvent.click(option)
}

async function selectComboboxOption(comboboxName: string, optionText: string) {
  fireEvent.click(screen.getByRole('combobox', { name: comboboxName }))
  const items = await waitFor(() => {
    const found = [
      ...document.querySelectorAll<HTMLElement>('[data-slot="command-item"]'),
    ].filter((item) => item.textContent?.includes(optionText))
    if (found.length === 0) throw new Error('option not rendered yet')
    return found
  })
  fireEvent.click(items[0])
}

beforeEach(() => {
  vi.mocked(createChannelChain).mockResolvedValue({
    success: true,
  } as never)
  vi.mocked(updateChannelChain).mockResolvedValue({
    success: true,
  } as never)
  vi.mocked(getGroupModelNames).mockResolvedValue(['gpt-4o', 'claude-sonnet-4'])
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('ChannelChainEditor', () => {
  it('submits the legacy group payload for a new group chain', async () => {
    const user = userEvent.setup()
    const { ref, onSaved } = renderEditor({})

    await user.type(screen.getByLabelText('Chain name'), 'Primary')
    await selectMultiSelectOption('Add a group to the chain', 'vip (2x)')
    await act(async () => ref.current?.submit())

    await waitFor(() => {
      expect(createChannelChain).toHaveBeenCalledWith({
        name: 'Primary',
        groups: ['vip'],
      })
    })
    expect(onSaved).toHaveBeenCalledTimes(1)
  })

  it('submits model rows after switching the chain type to models', async () => {
    const user = userEvent.setup()
    const { ref } = renderEditor({})

    await user.type(screen.getByLabelText('Chain name'), 'Routing')
    fireEvent.click(screen.getByRole('radio', { name: 'Models' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add model row' }))

    await selectComboboxOption('Group for model row', 'vip')
    const modelsInput = await screen.findByLabelText('Models for this row')
    fireEvent.mouseDown(modelsInput)
    fireEvent.click(await screen.findByText('gpt-4o'))
    // The accessible name stays stable after chips replace the placeholder.
    expect(screen.getByLabelText('Models for this row')).toBeTruthy()
    await act(async () => ref.current?.submit())

    await waitFor(() => {
      expect(createChannelChain).toHaveBeenCalledWith({
        name: 'Routing',
        type: 'models',
        items: [{ group: 'vip', models: ['gpt-4o'] }],
      })
    })
  })

  it('locks the chain type while editing a saved chain', async () => {
    const { ref } = renderEditor({ mode: 'edit', chain: modelsChain })

    expect(
      screen.getByText('The chain type cannot be changed after creation.')
    ).toBeTruthy()
    fireEvent.click(screen.getByRole('radio', { name: 'Groups' }))
    expect(screen.queryByRole('button', { name: 'Add model row' })).toBeTruthy()
    expect(
      screen.queryByRole('button', { name: 'Add a group to the chain' })
    ).toBeNull()

    await act(async () => ref.current?.submit())

    await waitFor(() => {
      expect(updateChannelChain).toHaveBeenCalledWith('chain_1', {
        name: 'Routing',
        type: 'models',
        items: [{ group: 'vip', models: ['gpt-4o'] }],
      })
    })
  })

  it('reports missing required fields instead of submitting', async () => {
    const { toast } = await import('sonner')
    const { ref } = renderEditor({})

    await act(async () => ref.current?.submit())

    expect(await screen.findByText('Name is required')).toBeTruthy()
    expect(createChannelChain).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith(
      'Please review the highlighted fields.'
    )
  })

  it('does not save a groups chain with a group that is no longer selectable', async () => {
    const { ref } = renderEditor({
      mode: 'edit',
      chain: {
        id: 'chain_groups',
        name: 'Legacy',
        type: 'groups',
        groups: ['vip', 'legacy'],
        items: [],
      },
    })

    await act(async () => ref.current?.submit())

    expect(
      await screen.findByText(
        'This group is no longer available for chains. Review the chain before saving.'
      )
    ).toBeTruthy()
    expect(updateChannelChain).not.toHaveBeenCalled()
  })

  it('does not save a model row whose group is no longer selectable', async () => {
    const { ref } = renderEditor({
      mode: 'edit',
      chain: {
        id: 'chain_models_group',
        name: 'Routing',
        type: 'models',
        groups: [],
        items: [{ group: 'legacy', models: ['gpt-4o'] }],
      },
    })

    await act(async () => ref.current?.submit())

    expect(
      await screen.findByText(
        'This group is no longer available for chains. Review the chain before saving.'
      )
    ).toBeTruthy()
    expect(updateChannelChain).not.toHaveBeenCalled()
  })

  it('keeps saved models that are no longer available and blocks saving', async () => {
    vi.mocked(getGroupModelNames).mockResolvedValue(['gpt-4o'])
    const { ref } = renderEditor({
      mode: 'edit',
      chain: {
        id: 'chain_models',
        name: 'Routing',
        type: 'models',
        groups: [],
        items: [{ group: 'vip', models: ['gpt-4o', 'retired-model'] }],
      },
    })

    await act(async () => ref.current?.submit())

    expect(
      await screen.findByText(
        'Some models are no longer available. Remove or replace them before saving.'
      )
    ).toBeTruthy()
    expect(updateChannelChain).not.toHaveBeenCalled()
    // The stale draft is kept visible instead of being silently cleared.
    expect(
      (await screen.findAllByText('retired-model (Unavailable)')).length
    ).toBeGreaterThan(0)
  })

  it('shows catalog errors alongside stale row errors without losing the draft', async () => {
    vi.mocked(getGroupModelNames).mockRejectedValue(new Error('offline'))
    const { ref } = renderEditor({
      mode: 'edit',
      chain: {
        id: 'chain_mixed_errors',
        name: 'Routing',
        type: 'models',
        groups: [],
        items: [
          { group: 'legacy', models: ['retired-model'] },
          { group: 'vip', models: ['gpt-4o'] },
        ],
      },
    })

    await act(async () => ref.current?.submit())

    expect(
      await screen.findByText(
        'This group is no longer available for chains. Review the chain before saving.'
      )
    ).toBeTruthy()
    expect(
      await screen.findByText(
        'Could not verify the selected options. Check your connection and try again.'
      )
    ).toBeTruthy()
    expect(updateChannelChain).not.toHaveBeenCalled()
    expect(
      (await screen.findAllByText(/retired-model/)).length
    ).toBeGreaterThan(0)
  })

  it('retries catalog verification on the next save after a load failure', async () => {
    let catalogOnline = false
    vi.mocked(getGroupModelNames).mockImplementation(async () => {
      if (!catalogOnline) throw new Error('offline')
      return ['gpt-4o']
    })
    const { ref } = renderEditor({
      mode: 'edit',
      chain: {
        id: 'chain_retry',
        name: 'Routing',
        type: 'models',
        groups: [],
        items: [{ group: 'vip', models: ['gpt-4o'] }],
      },
    })

    await act(async () => ref.current?.submit())

    expect(
      await screen.findByText(
        'Could not verify the selected options. Check your connection and try again.'
      )
    ).toBeTruthy()
    expect(updateChannelChain).not.toHaveBeenCalled()

    catalogOnline = true
    await act(async () => ref.current?.submit())

    await waitFor(() => {
      expect(updateChannelChain).toHaveBeenCalledWith('chain_retry', {
        name: 'Routing',
        type: 'models',
        items: [{ group: 'vip', models: ['gpt-4o'] }],
      })
    })
  })

  it('does not treat an unready group list as stale selections', async () => {
    const { ref } = renderEditor({ groupOptionsReady: false })
    const user = userEvent.setup()

    await user.type(screen.getByLabelText('Chain name'), 'Primary')
    await selectMultiSelectOption('Add a group to the chain', 'vip (2x)')
    await act(async () => ref.current?.submit())

    expect(
      await screen.findByText(
        'Could not verify the selected options. Check your connection and try again.'
      )
    ).toBeTruthy()
    expect(createChannelChain).not.toHaveBeenCalled()
  })
})
