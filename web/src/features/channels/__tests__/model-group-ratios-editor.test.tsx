import '@testing-library/jest-dom/vitest'
import { zodResolver } from '@hookform/resolvers/zod'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ModelGroupRatiosEditor } from '../components/model-group-ratios-editor'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
} from '../lib/channel-form'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function Fixture(props: {
  disabled?: boolean
  onSubmit: (value: unknown) => void
  initial?: ChannelFormValues['model_group_ratios']
}) {
  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(channelFormSchema),
    defaultValues: {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'upstream',
      models: 'model-a,model-b',
      model_group_ratios: props.initial ?? [],
    },
  })
  return (
    <I18nextProvider i18n={i18n}>
      <form
        onSubmit={form.handleSubmit((values) =>
          props.onSubmit(
            JSON.parse(transformFormDataToUpdatePayload(values, 1).setting!)
          )
        )}
      >
        <ModelGroupRatiosEditor
          form={form}
          models={['model-a', 'model-b']}
          disabled={props.disabled}
        />
        <button type='submit'>Save</button>
      </form>
    </I18nextProvider>
  )
}

afterEach(cleanup)

describe('sparse channel model ratios editor', () => {
  it('starts empty, selects a published model, and saves explicit free pricing', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)
    expect(
      screen.getByText('All models inherit their group pricing.')
    ).toBeVisible()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    await user.click(
      screen.getByRole('button', { name: 'Add model billing ratio' })
    )
    await user.click(screen.getByRole('combobox', { name: 'Published model' }))
    await user.click(await screen.findByRole('option', { name: 'model-a' }))
    await user.type(screen.getByRole('textbox', { name: 'Billing ratio' }), '0')
    expect(screen.getByText('Free on this channel')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ model_group_ratios: { 'model-a': 0 } })
      )
    )
  })

  it('shows invalid input and prevents saving until corrected; blank restores inheritance', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <Fixture
        onSubmit={onSubmit}
        initial={[{ model: 'model-a', ratio: '0.8' }]}
      />
    )
    const input = screen.getByRole('textbox', { name: 'Billing ratio' })
    await user.clear(input)
    await user.type(input, '1001')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Billing ratio must be a number between 0 and 1000'
    )
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(onSubmit).not.toHaveBeenCalled()
    await user.clear(input)
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(onSubmit).toHaveBeenCalled())
    expect(onSubmit.mock.calls[0][0]).not.toHaveProperty('model_group_ratios')
  })

  it('deletes only the selected override and preserves the remaining value', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <Fixture
        onSubmit={onSubmit}
        initial={[
          { model: 'model-a', ratio: '0.8' },
          { model: 'model-b', ratio: '1' },
        ]}
      />
    )
    await user.click(
      screen.getByRole('button', { name: 'Remove billing ratio 1' })
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ model_group_ratios: { 'model-b': 1 } })
      )
    )
  })

  it('disables add, select, input and delete when sensitive editing is unavailable', () => {
    render(
      <Fixture
        disabled
        onSubmit={vi.fn()}
        initial={[{ model: 'model-a', ratio: '0.8' }]}
      />
    )
    expect(
      screen.getByRole('button', { name: 'Add model billing ratio' })
    ).toBeDisabled()
    expect(screen.getByRole('combobox')).toBeDisabled()
    expect(
      screen.getByRole('textbox', { name: 'Billing ratio' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Remove billing ratio 1' })
    ).toBeDisabled()
  })
})
