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

import { UserInviterDialog } from '../user-inviter-dialog'

vi.mock('../../api', () => ({
  bindUserInviter: vi.fn(),
  searchUsers: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<unknown[]> }
}

const usersApi = await import('../../api')
const mockedBind = usersApi.bindUserInviter as unknown as AsyncMock
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

function buildUserItem(
  id: number,
  username: string,
  overrides: Record<string, unknown> = {}
) {
  return {
    id,
    username,
    display_name: username,
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
    ...overrides,
  }
}

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onOpenChange = vi.fn()
  const onSuccess = vi.fn()
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <UserInviterDialog
          open
          onOpenChange={onOpenChange}
          userIds={[1, 2]}
          onSuccess={onSuccess}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return { onOpenChange, onSuccess }
}

async function searchAndPickInviter(
  user: ReturnType<typeof userEvent.setup>,
  username = 'alice'
) {
  await user.type(screen.getByLabelText('Search users'), 'ali')
  await waitFor(() => expect(mockedSearchUsers.mock.calls.length).toBe(1))
  await user.click(await screen.findByText(username))
}

beforeEach(() => {
  if (!originalGetAnimations) {
    Object.defineProperty(Element.prototype, 'getAnimations', {
      configurable: true,
      value: (): Animation[] => [],
    })
  }
  mockedBind.mockReset()
  mockedSearchUsers.mockReset()
  mockedSearchUsers.mockResolvedValue({
    success: true,
    data: {
      items: [buildUserItem(3, 'alice')],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
})

afterEach(() => {
  cleanup()
  if (!originalGetAnimations) {
    Reflect.deleteProperty(Element.prototype, 'getAnimations')
  }
})

describe('user inviter dialog', () => {
  it('keeps the bind action disabled until an inviter is picked', async () => {
    renderDialog()

    expect(
      await screen.findByRole('button', { name: 'Bind Inviter' })
    ).toBeDisabled()
  })

  it('binds every selected user to the picked inviter', async () => {
    mockedBind.mockResolvedValue({ success: true, data: { updated: 2 } })
    const user = userEvent.setup()
    const callbacks = renderDialog()

    await searchAndPickInviter(user)
    const bindButton = screen.getByRole('button', { name: 'Bind Inviter' })
    await waitFor(() => expect(bindButton).toBeEnabled())
    await user.click(bindButton)

    await waitFor(() => expect(mockedBind.mock.calls.length).toBe(1))
    expect(mockedBind.mock.calls[0][0]).toEqual({
      user_ids: [1, 2],
      inviter_id: 3,
    })
    expect(callbacks.onSuccess).toHaveBeenCalledTimes(1)
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false)
  })

  it('does not offer selected users or deleted accounts as inviters', async () => {
    mockedSearchUsers.mockResolvedValue({
      success: true,
      data: {
        items: [
          buildUserItem(1, 'selected-target'),
          buildUserItem(4, 'deleted-account', { DeletedAt: '2026-01-01' }),
          buildUserItem(3, 'alice'),
        ],
        total: 3,
        page: 1,
        page_size: 20,
      },
    })
    const user = userEvent.setup()
    renderDialog()

    await user.type(screen.getByLabelText('Search users'), 'ali')

    expect(await screen.findByText('alice')).toBeVisible()
    expect(screen.queryByText('selected-target')).toBeNull()
    expect(screen.queryByText('deleted-account')).toBeNull()
  })

  it('keeps the dialog open when the server rejects the binding', async () => {
    mockedBind.mockResolvedValue({
      success: false,
      message: 'This binding would create a circular inviter chain',
    })
    const user = userEvent.setup()
    const callbacks = renderDialog()

    await searchAndPickInviter(user)
    await user.click(screen.getByRole('button', { name: 'Bind Inviter' }))

    await waitFor(() => expect(mockedBind.mock.calls.length).toBe(1))
    expect(callbacks.onSuccess).not.toHaveBeenCalled()
    expect(callbacks.onOpenChange).not.toHaveBeenCalledWith(false)
  })
})
