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
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ModelMappingPopover } from '../model-mapping-popover'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function renderPopover(modelMapping: string) {
  const onApply = vi.fn()
  render(
    <I18nextProvider i18n={i18n}>
      <ModelMappingPopover
        model='gpt-4o'
        modelMapping={modelMapping}
        onApply={onApply}
      />
    </I18nextProvider>
  )
  return onApply
}

async function openEditor() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Edit model mapping' }))
  const input = await screen.findByRole('textbox', {
    name: 'Public model name',
  })
  return { user, input }
}

afterEach(cleanup)

describe('model mapping popover', () => {
  it('prefills the current public name and applies a rename', async () => {
    const onApply = renderPopover(JSON.stringify({ 'public-a': 'gpt-4o' }))
    const { user, input } = await openEditor()

    expect(input).toHaveValue('public-a')
    await user.clear(input)
    await user.type(input, 'public-b')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(onApply).toHaveBeenCalledTimes(1)
    expect(JSON.parse(onApply.mock.calls[0][0])).toEqual({
      'public-b': 'gpt-4o',
    })
    expect(onApply.mock.calls[0][1]).toBe('public-b')
  })

  it('creates a public name for an unmapped upstream model', async () => {
    const onApply = renderPopover('{}')
    const { user, input } = await openEditor()

    expect(input).toHaveValue('')
    await user.type(input, 'public-a')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(JSON.parse(onApply.mock.calls[0][0])).toEqual({
      'public-a': 'gpt-4o',
    })
    expect(onApply.mock.calls[0][1]).toBe('public-a')
  })

  it('removes the mapping when the input is cleared', async () => {
    const onApply = renderPopover(JSON.stringify({ 'public-a': 'gpt-4o' }))
    const { user, input } = await openEditor()

    await user.clear(input)
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(onApply).toHaveBeenCalledWith('', '')
  })

  it('rejects a public name already used by another upstream model', async () => {
    const onApply = renderPopover(
      JSON.stringify({ 'public-a': 'gpt-4o', taken: 'other-model' })
    )
    const { user, input } = await openEditor()

    await user.clear(input)
    await user.type(input, 'taken')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      screen.getByText('This name is already mapped to another upstream model')
    ).toBeVisible()
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(onApply).not.toHaveBeenCalled()
  })

  it('rejects public names that contain spaces', async () => {
    const onApply = renderPopover('{}')
    const { user, input } = await openEditor()

    await user.type(input, 'bad name')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      screen.getByText(
        'Model names must not contain spaces or exceed 255 characters'
      )
    ).toBeVisible()
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(onApply).not.toHaveBeenCalled()
  })

  it('blocks editing when the stored mapping is invalid JSON', async () => {
    renderPopover('not-json')
    const { input } = await openEditor()

    expect(input).toBeDisabled()
    expect(
      screen.getByText(
        'Fix the existing model mapping JSON in the channel form first'
      )
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })
})
