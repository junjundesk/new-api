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
import { zodResolver } from '@hookform/resolvers/zod'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ACTIVITY_RATIO_SCOPE } from '../../constants'
import {
  ACTIVITY_CODE_FORM_DEFAULT_VALUES,
  type ActivityCodeFormValues,
  type ActivityModelRatioRow,
  getActivityCodeFormSchema,
  transformFormValuesToPayload,
} from '../../lib/activity-form'
import { ActivityModelRatiosEditor } from '../activity-model-ratios-editor'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function Fixture(props: {
  disabled?: boolean
  initialRows?: ActivityModelRatioRow[]
  onSubmit: (value: unknown) => void
}) {
  const form = useForm<ActivityCodeFormValues>({
    resolver: zodResolver(getActivityCodeFormSchema(i18n.t.bind(i18n))),
    defaultValues: {
      ...ACTIVITY_CODE_FORM_DEFAULT_VALUES,
      code: 'hdmmodels',
      name: 'Models activity',
      group_name: 'vip',
      ratio_scope: ACTIVITY_RATIO_SCOPE.MODELS,
      model_ratio_overrides: props.initialRows ?? [],
    },
  })
  return (
    <I18nextProvider i18n={i18n}>
      <form
        onSubmit={form.handleSubmit((values) =>
          props.onSubmit(transformFormValuesToPayload(values))
        )}
      >
        <ActivityModelRatiosEditor
          form={form}
          models={['gpt-5', 'o3']}
          disabled={props.disabled}
        />
        <button type='submit'>Save</button>
      </form>
    </I18nextProvider>
  )
}

afterEach(cleanup)

describe('activity model ratios editor', () => {
  it('starts empty, accepts a suggested model and saves its own ratio', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)

    expect(screen.getByText('No model ratio override yet.')).toBeVisible()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()

    await user.click(
      screen.getByRole('button', { name: 'Add model ratio override' })
    )
    await user.type(screen.getByRole('combobox', { name: 'Model' }), 'gpt-5')
    await user.type(
      screen.getByRole('textbox', { name: 'Model ratio' }),
      '0'
    )
    expect(
      screen.getByText('0 means this model is completely free.')
    ).toBeVisible()

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          ratio_scope: ACTIVITY_RATIO_SCOPE.MODELS,
          ratio_mode: 'inherit',
          ratio_override: 0,
          model_ratio_overrides: { 'gpt-5': 0 },
        })
      )
    )
  })

  it('accepts a model name that is not enabled in the group yet', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)

    await user.click(
      screen.getByRole('button', { name: 'Add model ratio override' })
    )
    await user.type(
      screen.getByRole('combobox', { name: 'Model' }),
      'not-on-sale-yet'
    )
    await user.type(
      screen.getByRole('textbox', { name: 'Model ratio' }),
      '0.25'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          model_ratio_overrides: { 'not-on-sale-yet': 0.25 },
        })
      )
    )
  })

  it('blocks saving an empty model list and explains why', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Select at least one model'
    )
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('blocks saving until a ratio is entered and marks the field invalid', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <Fixture onSubmit={onSubmit} initialRows={[{ model: 'gpt-5', ratio: '' }]} />
    )

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Please enter a ratio'
    )
    expect(screen.getByRole('textbox', { name: 'Model ratio' })).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('deletes only the selected row and keeps the remaining ratio', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <Fixture
        onSubmit={onSubmit}
        initialRows={[
          { model: 'gpt-5', ratio: '0' },
          { model: 'o3', ratio: '0.5' },
        ]}
      />
    )

    await user.click(
      screen.getByRole('button', { name: 'Remove model ratio override 1' })
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ model_ratio_overrides: { o3: 0.5 } })
      )
    )
  })

  it('disables add, model, ratio and delete when editing is unavailable', () => {
    render(
      <Fixture
        disabled
        onSubmit={vi.fn()}
        initialRows={[{ model: 'gpt-5', ratio: '0' }]}
      />
    )

    expect(
      screen.getByRole('button', { name: 'Add model ratio override' })
    ).toBeDisabled()
    expect(screen.getByRole('combobox', { name: 'Model' })).toBeDisabled()
    expect(screen.getByRole('textbox', { name: 'Model ratio' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Remove model ratio override 1' })
    ).toBeDisabled()
  })
})
