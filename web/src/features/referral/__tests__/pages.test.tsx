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
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { Referral } from '../index'
import { ReferralManagement } from '../management'

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  Link: (props: { to: string; children?: React.ReactNode }) => (
    <a href={props.to}>{props.children}</a>
  ),
}))
vi.mock('../api', () => ({
  getMyInvitees: vi.fn(),
  getInviters: vi.fn(),
  getInviterInvitees: vi.fn(),
}))
// The wallet and users API modules are imported by unrelated code pulled in
// through the page, so only the calls these pages make are replaced.
vi.mock('@/features/wallet/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/wallet/api')>()),
  getAffiliateCode: vi.fn(),
  getReferralSummary: vi.fn(),
  getReferralCharges: vi.fn(),
  transferAffiliateQuota: vi.fn(),
}))
vi.mock('@/features/users/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/users/api')>()),
  bindUserInviter: vi.fn(),
  searchUsers: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<unknown[]> }
}

const referralApi = await import('../api')
const walletApi = await import('@/features/wallet/api')
const mockedMyInvitees = referralApi.getMyInvitees as unknown as AsyncMock
const mockedInviters = referralApi.getInviters as unknown as AsyncMock
const mockedInviterInvitees =
  referralApi.getInviterInvitees as unknown as AsyncMock
const mockedAffCode = walletApi.getAffiliateCode as unknown as AsyncMock
const mockedSummary = walletApi.getReferralSummary as unknown as AsyncMock
const mockedCharges = walletApi.getReferralCharges as unknown as AsyncMock

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

function renderPage(node: React.ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>{node}</I18nextProvider>
    </QueryClientProvider>
  )
}

function page<T>(items: T[]) {
  return { items, total: items.length, page: 1, page_size: 10 }
}

function statValue(label: string): string {
  const card = screen
    .getAllByText(label)
    .map((node) => node.closest('[data-slot=card]'))
    .find((node): node is HTMLElement => node !== null)
  return card?.textContent ?? ''
}

beforeEach(() => {
  if (!originalGetAnimations) {
    Object.defineProperty(Element.prototype, 'getAnimations', {
      configurable: true,
      value: (): Animation[] => [],
    })
  }
  for (const mock of [
    mockedMyInvitees,
    mockedInviters,
    mockedInviterInvitees,
    mockedAffCode,
    mockedSummary,
    mockedCharges,
  ]) {
    mock.mockReset()
  }
  mockedAffCode.mockResolvedValue({ success: true, data: 'CODE' })
  mockedCharges.mockResolvedValue({ success: true, data: page([]) })
  mockedMyInvitees.mockResolvedValue({
    success: true,
    data: page([
      {
        id: 3,
        username: 'bo***nd',
        created_at: 1,
        valid: true,
        reward_quota: 0,
      },
      {
        id: 4,
        username: 'ca***nd',
        created_at: 1,
        valid: false,
        reward_quota: 0,
      },
    ]),
  })
})

afterEach(() => {
  cleanup()
  useAuthStore.getState().auth.reset()
  if (!originalGetAnimations) {
    Reflect.deleteProperty(Element.prototype, 'getAnimations')
  }
})

describe('invite rewards page', () => {
  it('shows the invited and valid invite counts from the summary', async () => {
    mockedSummary.mockResolvedValue({
      success: true,
      data: {
        enabled: true,
        registered_count: 12,
        valid_invitees: 4,
        claimable_quota: 0,
        tiers: [],
      },
    })
    renderPage(<Referral />)

    expect(await screen.findByText('bo***nd')).toBeVisible()
    expect(statValue('Invited')).toContain('12')
    expect(statValue('Valid invitees')).toContain('4')
  })

  // A disabled program returns no counts, but invite relationships still exist.
  it('falls back to the invitee list total while the program is disabled', async () => {
    mockedSummary.mockResolvedValue({ success: true, data: { enabled: false } })
    renderPage(<Referral />)

    expect(await screen.findByText('bo***nd')).toBeVisible()
    expect(statValue('Invited')).toContain('2')
    expect(
      screen.getByText('The referral program is currently disabled.')
    ).toBeVisible()
  })

  it('opens the transfer dialog when there is a claimable reward', async () => {
    mockedSummary.mockResolvedValue({
      success: true,
      data: {
        enabled: true,
        registered_count: 1,
        valid_invitees: 1,
        claimable_quota: 500000,
        tiers: [],
      },
    })
    renderPage(<Referral />)
    const transfer = await screen.findByRole('button', {
      name: 'Transfer to Balance',
    })
    await vi.waitFor(() => expect(transfer).toBeEnabled())

    await userEvent.click(transfer)

    expect(await screen.findByRole('dialog')).toBeVisible()
  })

  it.each([
    ['an admin', ROLE.ADMIN, true],
    ['a regular user', ROLE.USER, false],
  ])('%s sees the invite management entry: %s', async (_who, role, shown) => {
    useAuthStore.getState().auth.setUser({ id: 9, username: 'someone', role })
    mockedSummary.mockResolvedValue({
      success: true,
      data: { enabled: true, claimable_quota: 0, tiers: [] },
    })
    renderPage(<Referral />)
    expect(await screen.findByText('bo***nd')).toBeVisible()

    const entry = screen.queryByRole('link', { name: 'Invite Management' })
    if (shown) {
      expect(entry).toHaveAttribute('href', '/referral-management')
    } else {
      expect(entry).toBeNull()
    }
  })

  it('keeps the transfer action disabled when nothing is claimable', async () => {
    mockedSummary.mockResolvedValue({
      success: true,
      data: { enabled: true, claimable_quota: 0, tiers: [] },
    })
    renderPage(<Referral />)

    expect(await screen.findByText('bo***nd')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Transfer to Balance' })
    ).toBeDisabled()
  })
})

describe('invite management page', () => {
  beforeEach(() => {
    mockedInviters.mockResolvedValue({
      success: true,
      data: {
        ...page([
          {
            id: 2,
            username: 'alice',
            display_name: 'alice',
            registered_count: 3,
            valid_invitees: 2,
            history_quota: 0,
          },
        ]),
        overview: {
          inviters: 1,
          invitees: 3,
          valid_invitees: 2,
          reward_quota: 0,
        },
      },
    })
    mockedInviterInvitees.mockResolvedValue({
      success: true,
      data: page([
        {
          id: 5,
          username: 'dave_friend',
          created_at: 1,
          valid: false,
          reward_quota: 0,
        },
      ]),
    })
  })

  it('lists each inviter with the invited and valid counts', async () => {
    renderPage(<ReferralManagement />)

    const row = (await screen.findByText('alice')).closest('tr') as HTMLElement
    expect(
      within(row)
        .getAllByRole('cell')
        .map((cell) => cell.textContent)
        .slice(0, 3)
    ).toEqual(['#2alice', '3', '2'])
  })

  it('opens the invitee list of the chosen inviter', async () => {
    renderPage(<ReferralManagement />)

    await userEvent.click(
      await screen.findByRole('button', { name: 'View invited users' })
    )

    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText('dave_friend')).toBeVisible()
    expect(within(dialog).getByText('#5')).toBeVisible()
    expect(mockedInviterInvitees.mock.calls[0]).toEqual([2, 1, 10])
  })

  it('links back to the invite rewards page', async () => {
    renderPage(<ReferralManagement />)
    await screen.findByText('alice')

    expect(screen.getByRole('link', { name: 'Back' })).toHaveAttribute(
      'href',
      '/referral'
    )
  })

  it('opens the bind dialog asking for both the user and the inviter', async () => {
    renderPage(<ReferralManagement />)
    await screen.findByText('alice')

    await userEvent.click(screen.getByRole('button', { name: 'Bind Inviter' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByLabelText('Search invited user')).toBeVisible()
    expect(within(dialog).getByLabelText('Search users')).toBeVisible()
  })
})
