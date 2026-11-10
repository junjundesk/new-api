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
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { getGroupModelNames } from '../../api'
import type { UserChannelChainItemRow } from '../../lib/user-channel-chain'
import type { ApiKeyGroupOption } from '../api-key-group-combobox'
import { ChannelChainModelsEditor } from '../channel-chain-models-editor'

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

vi.mock('../../api', () => ({
  getGroupModelNames: vi.fn(),
}))

const groupOptions: ApiKeyGroupOption[] = [
  { value: 'vip', label: 'vip (2x)', ratio: 2 },
  { value: 'default', label: 'default (1x)', ratio: 1 },
]

type HarnessProps = {
  initialItems: UserChannelChainItemRow[]
  groupOptions?: ApiKeyGroupOption[]
  maxGroupsPerChain?: number
  rootError?: string
}

function Harness(props: HarnessProps) {
  const [items, setItems] = useState(props.initialItems)
  return (
    <ChannelChainModelsEditor
      items={items}
      groupOptions={props.groupOptions ?? groupOptions}
      maxModelsPerItem={100}
      maxGroupsPerChain={props.maxGroupsPerChain ?? 10}
      maxItemsBytes={60 * 1024}
      open
      rootError={props.rootError}
      rowErrors={[]}
      onChange={setItems}
    />
  )
}

function renderEditor(props: HarnessProps) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <Harness {...props} />
    </QueryClientProvider>
  )
}

function getRows(container: HTMLElement) {
  return [
    ...container.querySelectorAll<HTMLElement>(
      '[data-channel-chain-model-row]'
    ),
  ]
}

async function selectGroup(row: HTMLElement, optionText: string) {
  const trigger = within(row).getByRole('combobox', {
    name: 'Group for model row',
  })
  fireEvent.click(trigger)
  const items = await waitFor(() => {
    const found = [
      ...document.querySelectorAll<HTMLElement>('[data-slot="command-item"]'),
    ].filter((item) => item.textContent?.includes(optionText))
    if (found.length === 0) throw new Error('group option not rendered yet')
    return found
  })
  fireEvent.click(items[0])
  await waitFor(() => {
    if (!trigger.textContent?.includes(optionText)) {
      throw new Error(
        `group selection did not apply, trigger shows: ${trigger.textContent}`
      )
    }
  })
}

async function openModelsPicker(row: HTMLElement) {
  const input = await within(row).findByLabelText('Models for this row')
  fireEvent.mouseDown(input)
}

beforeEach(() => {
  vi.mocked(getGroupModelNames).mockResolvedValue(['gpt-4o', 'claude-sonnet-4'])
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('ChannelChainModelsEditor', () => {
  it('adds, reorders, and removes model rows', async () => {
    const { container } = renderEditor({ initialItems: [] })

    expect(
      screen.getByText('Add at least one model row to the chain')
    ).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Add model row' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add model row' }))
    expect(getRows(container)).toHaveLength(2)

    await selectGroup(getRows(container)[0], 'vip')
    await selectGroup(getRows(container)[1], 'default')
    fireEvent.click(
      within(getRows(container)[1]).getByRole('button', {
        name: 'Move model row up',
      })
    )

    const rows = getRows(container)
    expect(within(rows[0]).getByText(/default/)).toBeTruthy()
    expect(within(rows[1]).getByText(/vip/)).toBeTruthy()

    fireEvent.click(
      within(rows[0]).getByRole('button', { name: 'Remove model row' })
    )
    expect(getRows(container)).toHaveLength(1)
  })

  it('clears the selected models when the row group changes', async () => {
    const { container } = renderEditor({
      initialItems: [{ id: 'row-1', group: 'vip', models: ['gpt-4o'] }],
    })
    const row = getRows(container)[0]
    expect(within(row).getByText('Models: 1/100')).toBeTruthy()

    await selectGroup(row, 'default')

    await waitFor(() => {
      expect(
        within(getRows(container)[0]).getByText('Models: 0/100')
      ).toBeTruthy()
    })
    expect(screen.queryByText('gpt-4o')).toBeNull()
  })

  it('keeps saved models that are no longer offered by the group', async () => {
    const { container } = renderEditor({
      initialItems: [{ id: 'row-1', group: 'vip', models: ['retired-model'] }],
    })
    const row = getRows(container)[0]

    await openModelsPicker(row)

    const savedLabels = await screen.findAllByText(
      'retired-model (Unavailable)'
    )
    expect(savedLabels.length).toBeGreaterThan(0)
  })

  it('offers a retry when the group models fail to load', async () => {
    vi.mocked(getGroupModelNames)
      .mockRejectedValueOnce(new Error('network down'))
      .mockResolvedValueOnce(['gpt-4o'])
    const { container } = renderEditor({
      initialItems: [{ id: 'row-1', group: 'vip', models: [] }],
    })
    const row = getRows(container)[0]

    expect(await within(row).findByText('Could not load models')).toBeTruthy()
    fireEvent.click(within(row).getByRole('button', { name: 'Retry' }))

    await openModelsPicker(row)
    expect(await screen.findByText('gpt-4o')).toBeTruthy()
  })

  it('stops adding rows when every group is already used', () => {
    const { container } = renderEditor({
      initialItems: [
        { id: 'row-1', group: 'vip', models: ['gpt-4o'] },
        { id: 'row-2', group: 'default', models: ['gpt-4o'] },
      ],
    })

    expect(getRows(container)).toHaveLength(2)
    expect(
      (
        screen.getByRole('button', {
          name: 'Add model row',
        }) as HTMLButtonElement
      ).disabled
    ).toBe(true)
    expect(
      screen.getByText('No more selectable groups are available.')
    ).toBeTruthy()
  })

  it('shows the root error announced to assistive technology', () => {
    renderEditor({
      initialItems: [],
      rootError: 'The model list is too large.',
    })

    expect(screen.getByRole('alert').textContent).toBe(
      'The model list is too large.'
    )
  })
})
