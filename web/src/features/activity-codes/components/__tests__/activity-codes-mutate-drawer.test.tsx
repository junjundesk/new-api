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

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function renderDrawer() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ActivityCodesProvider>
          <ActivityCodesMutateDrawer open onOpenChange={vi.fn()} />
        </ActivityCodesProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
}

async function selectTargetGroup(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('combobox', { name: 'Target Group' }))
  await user.click(await screen.findByRole('option', { name: 'vip' }))
}

async function selectModelScope(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('combobox', { name: 'Ratio scope' }))
  await user.click(await screen.findByRole('option', { name: 'Selected models' }))
}

beforeEach(() => {
  mockedGroupNames.mockReset()
  mockedGroupModels.mockReset()
  mockedGroupNames.mockResolvedValue({ success: true, data: ['vip', 'default'] })
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
    await waitFor(() =>
      expect(mockedGroupModels.mock.calls).toEqual([['vip']])
    )

    await selectModelScope(user)
    await user.click(
      screen.getByRole('button', { name: 'Add model ratio override' })
    )
    await user.click(screen.getByRole('combobox', { name: 'Model' }))

    expect(await screen.findByRole('option', { name: 'gpt-5' })).toBeVisible()
    expect(screen.getByRole('option', { name: 'o3' })).toBeVisible()
  })
})
