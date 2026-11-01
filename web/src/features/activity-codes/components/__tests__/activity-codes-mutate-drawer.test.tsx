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

// 项目测试运行时只有 vi.fn()，没有 vi.mocked()，因此用窄接口访问模块 mock。
interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<unknown[]> }
}

const api = await import('../../api')
const mockedGroupNames = api.getGroupNames as unknown as AsyncMock
const mockedGroupModels = api.getGroupModels as unknown as AsyncMock
const mockedGetActivityCode = api.getActivityCode as unknown as AsyncMock
const mockedUpdateActivityCode = api.updateActivityCode as unknown as AsyncMock
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

const benefitActivityCode: ActivityCode = {
  id: 7,
  code: 'summer-benefit',
  name: 'Summer benefit',
  description: '',
  status: 1,
  group_name: 'vip',
  duration_seconds: 86400,
  rpm_limit: 0,
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
  type: 'benefit',
  benefits: [
    {
      key: 'benefit-1',
      group_name: 'vip',
      scope: 'model',
      model_name: 'gpt-5',
      rpm_limit: 60,
      token_total: 1_234_567,
    },
  ],
  created_by: 1,
  created_time: 0,
  updated_time: 0,
}

const displayActivityCode: ActivityCode = {
  ...benefitActivityCode,
  id: 9,
  code: 'summer-ratio',
  name: 'Summer ratio',
  type: 'ratio',
  benefits: null,
  sort_weight: 30,
  image_url: 'https://cdn.example.com/summer.png',
  active_end_time: 1_900_000_000,
}

async function selectTargetGroup(user: ReturnType<typeof userEvent.setup>) {
  await user.click(
    await screen.findByRole('combobox', { name: 'Target Group' })
  )
  await user.click(await screen.findByRole('option', { name: 'vip' }))
}

async function selectModelScope(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('combobox', { name: 'Ratio scope' }))
  await user.click(
    await screen.findByRole('option', { name: 'Selected models' })
  )
}

beforeEach(() => {
  mockedGroupNames.mockReset()
  mockedGroupModels.mockReset()
  mockedGetActivityCode.mockReset()
  mockedUpdateActivityCode.mockReset()
  mockedCreateActivityCode.mockReset()
  mockedCreateActivityCode.mockResolvedValue({ success: true })
  mockedGroupNames.mockResolvedValue({
    success: true,
    data: ['vip', 'default'],
  })
  mockedGroupModels.mockResolvedValue({
    success: true,
    data: [{ model_name: 'gpt-5' }, { model_name: 'o3' }],
  })
})

afterEach(cleanup)

describe('activity code mutate drawer ratio scope', () => {
  it('starts on the whole-group scope with the group ratio controls', async () => {
    renderDrawer()

    expect(
      await screen.findByRole('combobox', { name: 'Ratio scope' })
    ).toBeVisible()
    expect(screen.getByRole('combobox', { name: 'Ratio' })).toBeVisible()
    expect(screen.queryByText('Model ratio overrides')).not.toBeInTheDocument()
  })

  it('swaps the group ratio for the model editor on the selected-models scope', async () => {
    const user = userEvent.setup()
    renderDrawer()
    await selectTargetGroup(user)

    await selectModelScope(user)

    expect(
      screen.queryByRole('combobox', { name: 'Ratio' })
    ).not.toBeInTheDocument()
    expect(screen.getByText('Model ratio overrides')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Add model ratio override' })
    ).toBeVisible()
  })

  it('offers the selected group models as suggestions for the picker', async () => {
    const user = userEvent.setup()
    renderDrawer()
    await selectTargetGroup(user)
    await waitFor(() => expect(mockedGroupModels.mock.calls).toEqual([['vip']]))

    await selectModelScope(user)
    await user.click(
      screen.getByRole('button', { name: 'Add model ratio override' })
    )
    await user.click(screen.getByRole('combobox', { name: 'Model' }))

    expect(await screen.findByRole('option', { name: 'gpt-5' })).toBeVisible()
    expect(screen.getByRole('option', { name: 'o3' })).toBeVisible()
  })
})
describe('activity code mutate drawer type switch', () => {
  it('defaults to the ratio type and hides the benefit editor', async () => {
    renderDrawer()

    expect(
      await screen.findByRole('combobox', { name: 'Activity type' })
    ).toHaveTextContent('Ratio discount')
    expect(
      screen.queryByRole('button', { name: 'Add benefit' })
    ).not.toBeInTheDocument()
  })

  it('switches to free benefits and hides the ratio controls', async () => {
    const user = userEvent.setup()
    renderDrawer()

    await user.click(
      await screen.findByRole('combobox', { name: 'Activity type' })
    )
    await user.click(
      await screen.findByRole('option', { name: 'Free benefits' })
    )

    expect(screen.queryByLabelText('Ratio scope')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('RPM limit')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Target Group')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add benefit' })).toBeVisible()
    expect(screen.getByText('No benefits yet.')).toBeVisible()
  })

  it('disables the type switch and backfills benefits with raw token totals on edit', async () => {
    mockedGetActivityCode.mockResolvedValue({
      success: true,
      data: benefitActivityCode,
    })
    mockedUpdateActivityCode.mockResolvedValue({ success: true })
    const user = userEvent.setup()
    renderDrawer(benefitActivityCode)

    expect(
      await screen.findByRole('combobox', { name: 'Activity type' })
    ).toBeDisabled()
    expect(screen.getByRole('note')).toHaveTextContent(
      'Saving immediately syncs active grants.'
    )
    expect(screen.queryByLabelText('Ratio scope')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('RPM limit')).not.toBeInTheDocument()
    const tokenInput = await screen.findByRole('textbox', {
      name: 'Token amount for benefit 1',
    })
    expect(tokenInput).toHaveValue('1234567')

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(mockedUpdateActivityCode.mock.calls.length).toBe(1)
    )
    const payload = mockedUpdateActivityCode.mock.calls[0][0] as Record<
      string,
      unknown
    >
    expect(payload.type).toBe('benefit')
    expect(payload.group_name).toBe('vip')
    expect(payload.rpm_limit).toBe(0)
    expect(payload.benefits).toEqual([
      {
        key: 'benefit-1',
        group_name: 'vip',
        scope: 'model',
        model_name: 'gpt-5',
        rpm_limit: 60,
        token_total: 1_234_567,
      },
    ])
  })
})

describe('activity code mutate drawer display and expiry', () => {
  it('starts the display fields empty and every time picker unset on create', async () => {
    renderDrawer()

    expect(
      await screen.findByRole('spinbutton', { name: 'Sort weight' })
    ).toHaveValue(0)
    expect(
      screen.getByRole('textbox', { name: 'Background image URL' })
    ).toHaveValue('')
    expect(screen.getAllByRole('button', { name: 'Select date' })).toHaveLength(
      3
    )
  })

  it('blocks saving a background image that is neither a URL nor a path', async () => {
    const user = userEvent.setup()
    renderDrawer()
    await selectTargetGroup(user)

    await user.type(
      screen.getByRole('textbox', { name: 'Background image URL' }),
      'summer.png'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText('Enter an http(s) URL or a path starting with /.')
    ).toBeVisible()
    expect(mockedCreateActivityCode.mock.calls.length).toBe(0)
  })

  it('saves the sort weight and a root-relative background image with no end time', async () => {
    const user = userEvent.setup()
    renderDrawer()
    await user.type(
      screen.getByRole('textbox', { name: 'Activity Code' }),
      'summer-2026'
    )
    await user.type(screen.getByRole('textbox', { name: 'Name' }), 'Summer')
    await selectTargetGroup(user)

    const weightInput = await screen.findByRole('spinbutton', {
      name: 'Sort weight',
    })
    await user.clear(weightInput)
    await user.type(weightInput, '15')
    await user.type(
      screen.getByRole('textbox', { name: 'Background image URL' }),
      '/static/banner.png'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(mockedCreateActivityCode.mock.calls.length).toBe(1)
    )
    const payload = mockedCreateActivityCode.mock.calls[0][0] as Record<
      string,
      unknown
    >
    expect(payload.sort_weight).toBe(15)
    expect(payload.image_url).toBe('/static/banner.png')
    expect(payload.active_end_time).toBe(0)
  })

  it('backfills the display fields and sends the activity end time in seconds', async () => {
    mockedGetActivityCode.mockResolvedValue({
      success: true,
      data: displayActivityCode,
    })
    mockedUpdateActivityCode.mockResolvedValue({ success: true })
    const user = userEvent.setup()
    renderDrawer(displayActivityCode)

    expect(
      await screen.findByRole('spinbutton', { name: 'Sort weight' })
    ).toHaveValue(30)
    expect(
      screen.getByRole('textbox', { name: 'Background image URL' })
    ).toHaveValue('https://cdn.example.com/summer.png')
    // 活动到期时间已回填，三个时间选择器里只剩兑换开始/结束显示占位文案。
    expect(screen.getAllByRole('button', { name: 'Select date' })).toHaveLength(
      2
    )

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(mockedUpdateActivityCode.mock.calls.length).toBe(1)
    )
    const payload = mockedUpdateActivityCode.mock.calls[0][0] as Record<
      string,
      unknown
    >
    expect(payload.sort_weight).toBe(30)
    expect(payload.image_url).toBe('https://cdn.example.com/summer.png')
    expect(payload.active_end_time).toBe(1_900_000_000)
  })
})
