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
import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { FetchModelsDialog } from '../fetch-models-dialog'

vi.mock('../../channels-provider', () => ({
  useChannels: () => ({ currentRow: null }),
}))

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

const writeText = vi.fn()

beforeEach(() => {
  writeText.mockReset()
  writeText.mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText },
    configurable: true,
  })
})

afterEach(cleanup)

function renderDialog() {
  const onModelMappingChange = vi.fn()
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <FetchModelsDialog
          open
          onOpenChange={() => {}}
          onModelsSelected={() => {}}
          existingModelsOverride={['gpt-4o']}
          customFetcher={async () => ['gpt-4o']}
          modelMapping={JSON.stringify({ 'public-a': 'gpt-4o' })}
          onModelMappingChange={onModelMappingChange}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return onModelMappingChange
}

describe('fetch models dialog model actions', () => {
  it('shows the public name mapped to the upstream model', async () => {
    renderDialog()

    expect(await screen.findByText('gpt-4o')).toBeVisible()
    expect(screen.getByText('→ public-a')).toBeVisible()
  })

  it('copies the model name from the row action', async () => {
    renderDialog()

    fireEvent.click(
      await screen.findByRole('button', { name: 'Copy model name' })
    )

    await waitFor(() => expect(writeText).toHaveBeenCalledWith('gpt-4o'))
  })

  it('applies an edited mapping through the channel form callback', async () => {
    const onModelMappingChange = renderDialog()
    const user = userEvent.setup()

    await user.click(
      await screen.findByRole('button', { name: 'Edit model mapping' })
    )
    const input = await screen.findByRole('textbox', {
      name: 'Public model name',
    })
    expect(input).toHaveValue('public-a')

    await user.clear(input)
    await user.type(input, 'public-b')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(onModelMappingChange).toHaveBeenCalledTimes(1)
    expect(JSON.parse(onModelMappingChange.mock.calls[0][0])).toEqual({
      'public-b': 'gpt-4o',
    })
  })
})
