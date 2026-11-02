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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ActivityCode } from '../../types'
import { ActivityCodesMutateDrawer } from '../activity-codes-mutate-drawer'
import { ActivityCodesProvider } from '../activity-codes-provider'

vi.mock('../../api', () => ({
  createActivityCode: vi.fn(),
  getActivityCode: vi.fn(),
  getGroupModels: vi.fn(),
  getGroupNames: vi.fn(),
  updateActivityCode: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<unknown[]> }
}

const api = await import('../../api')
const mockedGroupNames = api.getGroupNames as unknown as AsyncMock
const mockedGroupModels = api.getGroupModels as unknown as AsyncMock
const mockedGetActivityCode = api.getActivityCode as unknown as AsyncMock
const mockedCreateActivityCode = api.createActivityCode as unknown as AsyncMock

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function renderDrawer(currentRow?: ActivityCode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ActivityCodesProvider>
          <ActivityCodesMutateDrawer
            open
            onOpenChange={vi.fn()}
            currentRow={currentRow}
          />
        </ActivityCodesProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
}

// 模拟旧接口返回：没有 limit_mode / concurrency_limit 字段的历史活动码。
function legacyActivityCode(overrides: Record<string, unknown>): ActivityCode {
  return {
    id: 7,
    code: 'legacy',
    name: 'Legacy code',
    description: '',
    status: 1,
    group_name: 'vip',
    duration_seconds: 86_400,
    rpm_limit: 60,
    price_quota: 0,
    ratio_mode: 'inherit',
    ratio_override: 0,
    ratio_scope: 'group',
    model_ratio_overrides: null,
    start_time: 0,
    end_time: 0,
    active_end_time: 0,
    sort_weight: 0,
    image_url: '',
    total_limit: 0,
    used_count: 0,
    per_user_limit: 1,
    public_visible: 0,
    type: 'ratio',
    benefits: null,
    created_by: 1,
    created_time: 0,
    updated_time: 0,
    ...overrides,
  } as unknown as ActivityCode
}

async function selectLimitType(
  user: ReturnType<typeof userEvent.setup>,
  optionName: string
) {
  await user.click(screen.getByRole('combobox', { name: 'Limit type' }))
  await user.click(await screen.findByRole('option', { name: optionName }))
}

async function fillRequiredFields(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    screen.getByRole('textbox', { name: 'Activity Code' }),
    'limitmode'
  )
  await user.type(screen.getByRole('textbox', { name: 'Name' }), 'Limit mode')
  await user.click(screen.getByRole('combobox', { name: 'Target Group' }))
  await user.click(await screen.findByRole('option', { name: 'vip' }))
}

beforeEach(() => {
  mockedGroupNames.mockReset()
  mockedGroupModels.mockReset()
  mockedGetActivityCode.mockReset()
  mockedCreateActivityCode.mockReset()
  mockedCreateActivityCode.mockResolvedValue({ success: true })
  mockedGroupNames.mockResolvedValue({
    success: true,
    data: ['vip', 'default'],
  })
  mockedGroupModels.mockResolvedValue({
    success: true,
    data: [{ model_name: 'gpt-5' }],
  })
})

afterEach(cleanup)

describe('activity code limit type fields', () => {
  it('starts on the rpm limit type with the concurrency input disabled', async () => {
    renderDrawer()

    const limitType = await screen.findByRole('combobox', {
      name: 'Limit type',
    })
    expect(limitType).toHaveTextContent('RPM')
    expect(screen.getByRole('spinbutton', { name: 'RPM limit' })).toBeEnabled()
    expect(
      screen.getByRole('spinbutton', { name: 'Concurrency limit' })
    ).toBeDisabled()
  })

  it('switches limit types without copying the rpm number into the concurrency input', async () => {
    const user = userEvent.setup()
    renderDrawer()

    const rpmInput = await screen.findByRole('spinbutton', {
      name: 'RPM limit',
    })
    await user.type(rpmInput, '60')

    await selectLimitType(user, 'Concurrency')

    const concurrencyInput = screen.getByRole('spinbutton', {
      name: 'Concurrency limit',
    })
    expect(rpmInput).toBeDisabled()
    expect(concurrencyInput).toBeEnabled()
    expect(concurrencyInput).toHaveValue(0)

    await selectLimitType(user, 'RPM')

    expect(screen.getByRole('spinbutton', { name: 'RPM limit' })).toHaveValue(
      60
    )
    expect(
      screen.getByRole('spinbutton', { name: 'Concurrency limit' })
    ).toBeDisabled()
  })

  it('submits the concurrency limit and zeroes the unused rpm limit', async () => {
    const user = userEvent.setup()
    renderDrawer()
    await fillRequiredFields(user)

    await selectLimitType(user, 'Concurrency')
    await user.type(
      screen.getByRole('spinbutton', { name: 'Concurrency limit' }),
      '10'
    )
    await user.click(screen.getByRole('button', { name: /Save/ }))

    await waitFor(() =>
      expect(mockedCreateActivityCode.mock.calls.length).toBe(1)
    )
    const payload = mockedCreateActivityCode.mock.calls[0][0] as Record<
      string,
      unknown
    >
    expect(payload.limit_mode).toBe('concurrency')
    expect(payload.concurrency_limit).toBe(10)
    expect(payload.rpm_limit).toBe(0)
  })

  it('backfills a legacy activity without limit fields as the rpm type', async () => {
    mockedGetActivityCode.mockResolvedValue({
      success: true,
      data: legacyActivityCode({}),
    })
    renderDrawer(legacyActivityCode({}))

    const limitType = await screen.findByRole('combobox', {
      name: 'Limit type',
    })
    await waitFor(() => expect(limitType).toHaveTextContent('RPM'))
    expect(screen.getByRole('spinbutton', { name: 'RPM limit' })).toHaveValue(
      60
    )
    expect(
      screen.getByRole('spinbutton', { name: 'Concurrency limit' })
    ).toHaveValue(0)
  })

  it('blocks saving when the concurrency limit exceeds 100000', async () => {
    const user = userEvent.setup()
    renderDrawer()
    await fillRequiredFields(user)

    await selectLimitType(user, 'Concurrency')
    await user.type(
      screen.getByRole('spinbutton', { name: 'Concurrency limit' }),
      '100001'
    )
    await user.click(screen.getByRole('button', { name: /Save/ }))

    expect(
      await screen.findByText('Concurrency limit is too large')
    ).toBeVisible()
    expect(mockedCreateActivityCode.mock.calls.length).toBe(0)
  })

  it('saves after switching away from an out-of-range unused input', async () => {
    const user = userEvent.setup()
    renderDrawer()
    await fillRequiredFields(user)

    await user.type(
      screen.getByRole('spinbutton', { name: 'RPM limit' }),
      '100001'
    )
    await selectLimitType(user, 'Concurrency')
    await user.type(
      screen.getByRole('spinbutton', { name: 'Concurrency limit' }),
      '3'
    )
    await user.click(screen.getByRole('button', { name: /Save/ }))

    await waitFor(() =>
      expect(mockedCreateActivityCode.mock.calls.length).toBe(1)
    )
    const payload = mockedCreateActivityCode.mock.calls[0][0] as Record<
      string,
      unknown
    >
    expect(payload.limit_mode).toBe('concurrency')
    expect(payload.concurrency_limit).toBe(3)
    expect(payload.rpm_limit).toBe(0)
  })
})
