import '@testing-library/jest-dom/vitest'
import { zodResolver } from '@hookform/resolvers/zod'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  normalizeBaseUrls,
  transformFormDataToCreatePayload,
  type ChannelFormValues,
} from '../../lib/channel-form'
import { BaseUrlListField } from '../base-url-list-field'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function ListFixture(props: { initial: string[] }) {
  const [rows, setRows] = useState<string[]>(props.initial)
  return (
    <I18nextProvider i18n={i18n}>
      <BaseUrlListField value={rows} onChange={setRows} />
    </I18nextProvider>
  )
}

function FormFixture(props: {
  initial: string[]
  onSubmit: (value: unknown) => void
}) {
  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(channelFormSchema),
    defaultValues: {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'upstream',
      models: 'model-a',
      base_urls: props.initial,
      base_url: normalizeBaseUrls(props.initial)[0] ?? '',
    },
  })
  return (
    <I18nextProvider i18n={i18n}>
      <form
        onSubmit={form.handleSubmit((values) =>
          props.onSubmit(transformFormDataToCreatePayload(values))
        )}
      >
        <BaseUrlListField
          value={form.watch('base_urls') ?? []}
          onChange={(rows) => {
            form.setValue('base_urls', rows)
            form.setValue('base_url', normalizeBaseUrls(rows)[0] ?? '')
          }}
        />
        <button type='submit'>Save</button>
      </form>
    </I18nextProvider>
  )
}

afterEach(cleanup)

describe('base url list field', () => {
  it('renders existing addresses and marks the first one as primary', () => {
    render(
      <ListFixture
        initial={['https://a.example.com', 'https://b.example.com']}
      />
    )
    expect(screen.getByRole('textbox', { name: 'API address 1' })).toHaveValue(
      'https://a.example.com'
    )
    expect(screen.getByRole('textbox', { name: 'API address 2' })).toHaveValue(
      'https://b.example.com'
    )
    expect(screen.getByText('Primary address')).toBeVisible()
    expect(screen.getByText('Backup address')).toBeVisible()
  })

  it('adds rows and removes rows while keeping at least one row', async () => {
    const user = userEvent.setup()
    render(<ListFixture initial={['https://a.example.com']} />)
    expect(
      screen.getByRole('button', { name: 'Remove API address 1' })
    ).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Add backup address' }))
    expect(screen.getByRole('textbox', { name: 'API address 2' })).toBeVisible()
    await user.click(
      screen.getByRole('button', { name: 'Remove API address 2' })
    )
    expect(
      screen.queryByRole('textbox', { name: 'API address 2' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'API address 1' })).toHaveValue(
      'https://a.example.com'
    )
  })

  it('shows a field error for a non-http(s) address', async () => {
    const user = userEvent.setup()
    render(<ListFixture initial={['']} />)
    const input = screen.getByRole('textbox', { name: 'API address 1' })
    await user.type(input, 'ftp://example.com')
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Please enter a valid http/https absolute URL'
    )
    expect(input).toHaveAttribute('aria-invalid', 'true')
  })

  it('serializes the deduplicated normalized list with the first address as base_url', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <FormFixture
        initial={[
          'https://a.example.com/',
          'https://a.example.com',
          'https://b.example.com',
          '',
        ]}
        onSubmit={onSubmit}
      />
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          channel: expect.objectContaining({
            base_url: 'https://a.example.com',
            base_urls: ['https://a.example.com', 'https://b.example.com'],
          }),
        })
      )
    )
  })

  it('mirrors the edited first row into base_url and filters blank rows on submit', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <FormFixture initial={['https://old.example.com']} onSubmit={onSubmit} />
    )
    const input = screen.getByRole('textbox', { name: 'API address 1' })
    await user.clear(input)
    await user.type(input, 'https://new.example.com')
    await user.click(screen.getByRole('button', { name: 'Add backup address' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          channel: expect.objectContaining({
            base_url: 'https://new.example.com',
            base_urls: ['https://new.example.com'],
          }),
        })
      )
    )
  })

  it('blocks submission when an address is invalid', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<FormFixture initial={['not-a-url']} onSubmit={onSubmit} />)
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findAllByRole('alert')).not.toHaveLength(0)
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
