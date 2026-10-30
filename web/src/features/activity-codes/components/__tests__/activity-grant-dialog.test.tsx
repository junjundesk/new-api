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
import { useEffect } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ActivityCode } from '../../types'
import {
  ActivityCodesProvider,
  useActivityCodes,
} from '../activity-codes-provider'
import { ActivityGrantDialog } from '../activity-grant-dialog'

vi.mock('../../api', () => ({
  grantActivityCode: vi.fn(),
}))

vi.mock('@/features/users/api', () => ({
  searchUsers: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<unknown[]> }
}

const activityApi = await import('../../api')
const usersApi = await import('@/features/users/api')
const mockedGrant = activityApi.grantActivityCode as unknown as AsyncMock
const mockedSearchUsers = usersApi.searchUsers as unknown as AsyncMock

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

const originalGetAnimations = Object.getOwnPropertyDescriptor(
  Element.prototype,
  'getAnimations'
)

const activityCode: ActivityCode = {
  id: 12,
  code: 'summer-gift',
  name: 'Summer gift',
  description: '',
  status: 1,
  group_name: 'vip',
  duration_seconds: 24 * 3600,
  rpm_limit: 0,
  price_quota: 0,
  ratio_mode: 'inherit',
  ratio_override: 0,
  ratio_scope: 'group',
  model_ratio_overrides: null,
  start_time: 0,
  end_time: 0,
  total_limit: 0,
  used_count: 0,
  per_user_limit: 1,
  public_visible: 0,
  type: 'ratio',
  benefits: null,
  created_by: 1,
  created_time: 0,
  updated_time: 0,
}

function DialogHarness(props: { code: ActivityCode }) {
  const { setOpen, setCurrentRow } = useActivityCodes()

  useEffect(() => {
    setCurrentRow(props.code)
    setOpen('grant')
  }, [props.code, setCurrentRow, setOpen])

  return <ActivityGrantDialog />
}

function renderDialog(code: ActivityCode = activityCode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ActivityCodesProvider>
          <DialogHarness code={code} />
        </ActivityCodesProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
}

function searchPayload(items: Array<{ id: number; username: string }>) {
  return {
    success: true,
    data: {
      items: items.map((item) => ({
        ...item,
        display_name: item.username,
        quota: 0,
        used_quota: 0,
        request_count: 0,
        group: 'default',
        status: 1,
        role: 1,
      })),
      total: items.length,
      page: 1,
      page_size: 20,
    },
  }
}

async function searchAndSelect(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('Users'), 'alice')
  await user.click(screen.getByRole('button', { name: 'Search' }))
  await user.click(await screen.findByText('alice'))
}

beforeEach(() => {
  // happy-dom lacks the Web Animations API used by the real ScrollArea.
  if (!originalGetAnimations) {
    Object.defineProperty(Element.prototype, 'getAnimations', {
      configurable: true,
      value: (): Animation[] => [],
    })
  }
  mockedGrant.mockReset()
  mockedSearchUsers.mockReset()
  mockedSearchUsers.mockResolvedValue(
    searchPayload([{ id: 3, username: 'alice' }])
  )
})

afterEach(() => {
  cleanup()
  if (!originalGetAnimations) {
    Reflect.deleteProperty(Element.prototype, 'getAnimations')
  }
})

const benefitCode: ActivityCode = {
  ...activityCode,
  type: 'benefit',
  benefits: [
    {
      key: 'benefit-1',
      group_name: 'vip',
      scope: 'group',
      model_name: '',
      rpm_limit: 0,
      token_total: 0,
    },
    {
      key: 'benefit-2',
      group_name: 'default',
      scope: 'model',
      model_name: 'gpt-5',
      rpm_limit: 60,
      token_total: 2_000_000,
    },
  ],
}

describe('activity grant dialog', () => {
  it('lists every benefit group and token package for a benefit code', async () => {
    renderDialog(benefitCode)

    expect(await screen.findByText(/vip, default/)).toBeVisible()
    expect(screen.getByText('vip · Whole group · Unlimited tokens')).toBeVisible()
    expect(
      screen.getByText(/default · gpt-5 · 2M tokens/)
    ).toBeVisible()
    expect(screen.getByText(/default · gpt-5 · 2M tokens · RPM: 60/)).toBeVisible()
  })

  it('keeps the grant action disabled until at least one user is selected', async () => {
    renderDialog()

    expect(
      await screen.findByRole('button', { name: 'Grant to 0 users' })
    ).toBeDisabled()
  })

  it('grants the selected users to the current activity code', async () => {
    mockedGrant.mockResolvedValue({
      success: true,
      data: { granted: 1, failed: 0, total: 1, results: [] },
    })
    const user = userEvent.setup()
    renderDialog()

    await searchAndSelect(user)

    const grantButton = screen.getByRole('button', { name: 'Grant to 1 users' })
    await waitFor(() => expect(grantButton).toBeEnabled())
    await user.click(grantButton)

    await waitFor(() => expect(mockedGrant.mock.calls.length).toBe(1))
    expect(mockedGrant.mock.calls[0][0]).toBe(activityCode.id)
    expect(mockedGrant.mock.calls[0][1]).toEqual({
      user_ids: [3],
      remark: '',
    })
  })

  it('keeps the dialog open and the failed user selected for retry', async () => {
    mockedGrant.mockResolvedValue({
      success: true,
      data: {
        granted: 0,
        failed: 1,
        total: 1,
        results: [
          { user_id: 3, success: false, message: 'Activity code disabled' },
        ],
      },
    })
    const user = userEvent.setup()
    renderDialog()

    await searchAndSelect(user)
    await user.click(screen.getByRole('button', { name: 'Grant to 1 users' }))

    await waitFor(() => expect(mockedGrant.mock.calls.length).toBe(1))
    expect(await screen.findByText('Grant Activity Code')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Remove alice' })).toBeVisible()
  })
})
