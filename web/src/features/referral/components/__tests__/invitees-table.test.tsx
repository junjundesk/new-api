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
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { ReferralInvitee } from '../../types'
import { InviteesTable } from '../invitees-table'
import { ListPagination } from '../list-pagination'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

function renderWithI18n(node: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{node}</I18nextProvider>)
}

function buildInvitee(overrides: Partial<ReferralInvitee>): ReferralInvitee {
  return {
    id: 1,
    username: 'li***88',
    created_at: 0,
    valid: false,
    reward_quota: 0,
    ...overrides,
  }
}

afterEach(cleanup)

describe('invitees table', () => {
  it('marks paying invitees as valid and the rest as registered only', () => {
    renderWithI18n(
      <InviteesTable
        invitees={[
          buildInvitee({ id: 1, username: 'pa***er', valid: true }),
          buildInvitee({ id: 2, username: 'id***le' }),
        ]}
      />
    )

    const payer = screen.getByText('pa***er').closest('tr') as HTMLElement
    const idle = screen.getByText('id***le').closest('tr') as HTMLElement
    expect(within(payer).getByText('Valid invite')).toBeVisible()
    expect(within(idle).getByText('Registered only')).toBeVisible()
  })

  it('shows the empty state when nobody was invited', () => {
    renderWithI18n(<InviteesTable invitees={[]} />)

    expect(screen.getByText('No invited users yet')).toBeVisible()
    expect(screen.queryByRole('table')).toBeNull()
  })

  it('shows the user ID only in the admin view', () => {
    const invitees = [buildInvitee({ id: 42, username: 'full-name' })]

    const userView = renderWithI18n(<InviteesTable invitees={invitees} />)
    expect(screen.queryByText('#42')).toBeNull()
    userView.unmount()

    renderWithI18n(<InviteesTable invitees={invitees} showId />)
    expect(screen.getByText('#42')).toBeVisible()
  })
})

describe('list pagination', () => {
  it('is hidden when everything fits on one page', () => {
    renderWithI18n(
      <ListPagination
        page={1}
        pageSize={10}
        total={10}
        onPageChange={vi.fn()}
      />
    )

    expect(screen.queryByRole('button')).toBeNull()
  })

  it('disables the previous button on the first page and moves forward', async () => {
    const onPageChange = vi.fn()
    renderWithI18n(
      <ListPagination
        page={1}
        pageSize={10}
        total={25}
        onPageChange={onPageChange}
      />
    )

    expect(screen.getByText('Page 1 of 3')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    expect(onPageChange).toHaveBeenCalledWith(2)
  })

  it('disables the next button on the last page', () => {
    renderWithI18n(
      <ListPagination
        page={3}
        pageSize={10}
        total={25}
        onPageChange={vi.fn()}
      />
    )

    expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Previous page' })).toBeEnabled()
  })
})
