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
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { AssistantConfirmationCard } from '../components/assistant-confirmation-card'
import type { AssistantConfirmationPart } from '../types'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

const basePart: AssistantConfirmationPart = {
  id: 'confirmation-conf-1',
  type: 'confirmation',
  confirmationId: 'conf-1',
  operationId: 'DELETE /api/token/7',
  risk: 'destructive',
  summary: 'Delete the API key token-7',
  params: { path_params: { id: 7 } },
  expiresAt: Math.floor(Date.now() / 1000) + 120,
  status: 'pending',
}

function renderCard(
  part: AssistantConfirmationPart,
  onConfirm: (confirmationId: string, approve: boolean) => Promise<void> = vi
    .fn()
    .mockResolvedValue(undefined)
) {
  render(
    <I18nextProvider i18n={i18n}>
      <AssistantConfirmationCard onConfirm={onConfirm} part={part} />
    </I18nextProvider>
  )
  return onConfirm
}

afterEach(() => {
  cleanup()
})

describe('assistant confirmation card', () => {
  it('renders the operation, risk, summary and actions for a pending write', () => {
    renderCard(basePart)

    expect(screen.getByText('Confirmation required')).toBeVisible()
    expect(screen.getByText('Destructive operation')).toBeVisible()
    expect(screen.getByText('Delete the API key token-7')).toBeVisible()
    expect(screen.getByText('DELETE /api/token/7')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Approve' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Deny' })).toBeEnabled()
    expect(screen.getByText(/Expires in/)).toBeVisible()
  })

  it('submits the approval once and disables the actions while submitting', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn(() => new Promise<void>(() => undefined))
    renderCard(basePart, onConfirm)

    await user.click(screen.getByRole('button', { name: 'Approve' }))

    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(onConfirm).toHaveBeenCalledWith('conf-1', true)
    expect(screen.getByRole('button', { name: 'Approve' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Deny' })).toBeDisabled()
  })

  it('shows the server error and re-enables the actions when the request fails', async () => {
    const user = userEvent.setup()
    const onConfirm = vi
      .fn()
      .mockRejectedValue(new Error('confirmation not found or expired'))
    renderCard(basePart, onConfirm)

    await user.click(screen.getByRole('button', { name: 'Approve' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'confirmation not found or expired'
    )
    expect(screen.getByRole('button', { name: 'Approve' })).toBeEnabled()
  })

  it('falls back to the translated failure text when the request fails silently', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn().mockRejectedValue(new Error(''))
    renderCard(basePart, onConfirm)

    await user.click(screen.getByRole('button', { name: 'Deny' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Failed to send the confirmation decision'
    )
  })

  it('renders a resolved confirmation without actions', () => {
    renderCard({ ...basePart, status: 'approved' })

    expect(screen.getByText('Approved')).toBeVisible()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Deny' })).toBeNull()
  })

  it('treats an elapsed expiry as expired without a server event', () => {
    renderCard({
      ...basePart,
      expiresAt: Math.floor(Date.now() / 1000) - 1,
    })

    expect(screen.getByText('Expired')).toBeVisible()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
  })
})
