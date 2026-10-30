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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  ACTIVITY_BENEFIT_SCOPE,
  ACTIVITY_BENEFIT_TOKEN_MODE,
  ACTIVITY_CODE_TYPE,
} from '../../constants'
import {
  type ActivityBenefitRow,
  createActivityBenefitRow,
} from '../../lib/activity-benefit'
import {
  ACTIVITY_CODE_FORM_DEFAULT_VALUES,
  type ActivityCodeFormValues,
  getActivityCodeFormSchema,
  transformFormValuesToPayload,
} from '../../lib/activity-form'
import { ActivityBenefitsEditor } from '../activity-benefits-editor'

vi.mock('../../api', () => ({
  getGroupModels: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<unknown[]> }
}

const api = await import('../../api')
const mockedGroupModels = api.getGroupModels as unknown as AsyncMock

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

type FixtureProps = {
  disabled?: boolean
  initialRows?: ActivityBenefitRow[]
  onSubmit: (value: unknown) => void
}

function Fixture(props: FixtureProps) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { retry: false } },
      })
  )
  const form = useForm<ActivityCodeFormValues>({
    resolver: zodResolver(getActivityCodeFormSchema(i18n.t.bind(i18n))),
    defaultValues: {
      ...ACTIVITY_CODE_FORM_DEFAULT_VALUES,
      code: 'benefit-code',
      name: 'Benefit activity',
      group_name: 'vip',
      type: ACTIVITY_CODE_TYPE.BENEFIT,
      benefits: props.initialRows ?? [],
    },
  })
  return (
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <form
          onSubmit={form.handleSubmit((values) =>
            props.onSubmit(transformFormValuesToPayload(values))
          )}
        >
          <ActivityBenefitsEditor
            form={form}
            groupNames={['vip', 'default']}
            disabled={props.disabled}
          />
          <button type='submit'>Save</button>
        </form>
      </I18nextProvider>
    </QueryClientProvider>
  )
}

async function addBenefitRow(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: 'Add benefit' }))
}

async function selectOption(
  user: ReturnType<typeof userEvent.setup>,
  comboboxName: string,
  optionName: string
) {
  await user.click(screen.getByRole('combobox', { name: comboboxName }))
  await user.click(await screen.findByRole('option', { name: optionName }))
}

beforeEach(() => {
  mockedGroupModels.mockReset()
  mockedGroupModels.mockResolvedValue({
    success: true,
    data: [{ model_name: 'gpt-5' }, { model_name: 'o3' }],
  })
})

afterEach(cleanup)

describe('activity benefits editor', () => {
  it('starts with an empty state and appends a numbered benefit row', async () => {
    const user = userEvent.setup()
    render(<Fixture onSubmit={vi.fn()} />)

    expect(screen.getByText('No benefits yet.')).toBeVisible()
    await addBenefitRow(user)

    expect(
      screen.getByRole('combobox', { name: 'Group for benefit 1' })
    ).toBeVisible()
    expect(screen.queryByText('No benefits yet.')).not.toBeInTheDocument()
  })

  it('parses a token package and previews the normalized amount on save', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)

    await addBenefitRow(user)
    await selectOption(user, 'Group for benefit 1', 'vip')
    await selectOption(user, 'Quota mode for benefit 1', 'Token package')
    await user.type(
      screen.getByRole('textbox', { name: 'Token amount for benefit 1' }),
      '10万'
    )

    expect(await screen.findByText('Normalized amount: 100K')).toBeVisible()

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          type: ACTIVITY_CODE_TYPE.BENEFIT,
          benefits: [
            {
              key: '',
              group_name: 'vip',
              scope: ACTIVITY_BENEFIT_SCOPE.GROUP,
              model_name: '',
              rpm_limit: 0,
              token_total: 100_000,
            },
          ],
        })
      )
    )
  })

  it('loads group models and saves a single-model benefit', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)

    await addBenefitRow(user)
    await selectOption(user, 'Group for benefit 1', 'vip')
    await selectOption(user, 'Scope for benefit 1', 'Single model')
    await waitFor(() =>
      expect(mockedGroupModels.mock.calls).toEqual([['vip']])
    )

    await user.click(
      screen.getByRole('combobox', { name: 'Model for benefit 1' })
    )
    expect(await screen.findByRole('option', { name: 'gpt-5' })).toBeVisible()
    await user.click(screen.getByRole('option', { name: 'gpt-5' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          benefits: [
            expect.objectContaining({
              scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
              model_name: 'gpt-5',
            }),
          ],
        })
      )
    )
  })

  it('blocks saving when the token input cannot be parsed', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)

    await addBenefitRow(user)
    await selectOption(user, 'Group for benefit 1', 'vip')
    await selectOption(user, 'Quota mode for benefit 1', 'Token package')
    await user.type(
      screen.getByRole('textbox', { name: 'Token amount for benefit 1' }),
      'abc'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Enter a valid token amount, for example 1000000, 1M or 100万'
    )
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('blocks saving without any benefit', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Fixture onSubmit={onSubmit} />)

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Add at least one benefit'
    )
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('blocks saving the same group and scope twice', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <Fixture
        onSubmit={onSubmit}
        initialRows={[
          createActivityBenefitRow({ group_name: 'vip' }),
          createActivityBenefitRow({ group_name: 'vip' }),
        ]}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'This benefit is already configured'
    )
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('removes only the selected benefit row', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <Fixture
        onSubmit={onSubmit}
        initialRows={[
          createActivityBenefitRow({ group_name: 'vip' }),
          createActivityBenefitRow({ group_name: 'default' }),
        ]}
      />
    )

    await user.click(
      screen.getByRole('button', { name: 'Remove benefit 1' })
    )
    expect(
      screen.queryByRole('combobox', { name: 'Group for benefit 2' })
    ).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          benefits: [expect.objectContaining({ group_name: 'default' })],
        })
      )
    )
  })

  it('disables adding more than 20 benefits', () => {
    const rows = Array.from({ length: 20 }, () =>
      createActivityBenefitRow({ group_name: 'vip' })
    )
    render(<Fixture onSubmit={vi.fn()} initialRows={rows} />)

    expect(screen.getByRole('button', { name: 'Add benefit' })).toBeDisabled()
  })

  it('disables every control when the editor is read only', () => {
    render(
      <Fixture
        disabled
        onSubmit={vi.fn()}
        initialRows={[
          createActivityBenefitRow({
            group_name: 'vip',
            token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE,
            token_total_input: '1M',
          }),
        ]}
      />
    )

    expect(screen.getByRole('button', { name: 'Add benefit' })).toBeDisabled()
    expect(
      screen.getByRole('combobox', { name: 'Group for benefit 1' })
    ).toBeDisabled()
    expect(
      screen.getByRole('textbox', { name: 'Token amount for benefit 1' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Remove benefit 1' })
    ).toBeDisabled()
  })
})
