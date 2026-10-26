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
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { FallbackAttemptInfo, FallbackLogInfo } from '../../types'
import { LogFallbackBadge } from '../log-fallback-badge'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      let text = key
      for (const [name, value] of Object.entries(opts ?? {})) {
        text = text.replaceAll(`{{${name}}}`, String(value))
      }
      return text
    },
  }),
}))

afterEach(() => {
  cleanup()
})

function renderBadge(
  fallback?: FallbackLogInfo,
  options: { isAdmin?: boolean; adminAttempts?: FallbackAttemptInfo[] } = {}
) {
  return render(
    <LogFallbackBadge
      fallback={fallback}
      isAdmin={options.isAdmin}
      adminAttempts={options.adminAttempts}
    />
  )
}

describe('LogFallbackBadge', () => {
  it('renders nothing when the log has no fallback information', () => {
    renderBadge(undefined)

    expect(screen.queryByRole('button')).toBeNull()
  })

  it('renders nothing when neither model is known', () => {
    renderBadge({})

    expect(screen.queryByRole('button')).toBeNull()
  })

  it('exposes an accessible badge for a fallback request', () => {
    renderBadge({ from_model: 'gpt-5', to_model: 'backup' })

    expect(
      screen.getByRole('button', { name: 'Fallback request' })
    ).toBeTruthy()
  })

  it('shows the original and fallback model when opened', async () => {
    renderBadge({
      from_model: 'gpt-5',
      from_group: 'default',
      to_model: 'claude-sonnet-4-5',
      to_group: 'vip',
      attempt: 2,
    })

    await userEvent.click(
      screen.getByRole('button', { name: 'Fallback request' })
    )

    expect(await screen.findByText('Original model')).toBeTruthy()
    expect(screen.getByText(/gpt-5/)).toBeTruthy()
    expect(screen.getByText(/claude-sonnet-4-5/)).toBeTruthy()
    expect(screen.getByText('Attempt {{n}}'.replace('{{n}}', '2'))).toBeTruthy()
  })

  it('reports when every fallback target failed', async () => {
    renderBadge({ from_model: 'gpt-5', failed: true, attempts: 3 })

    await userEvent.click(
      screen.getByRole('button', { name: 'Fallback request' })
    )

    expect(
      await screen.findByText('All fallback models failed (3 attempts)')
    ).toBeTruthy()
  })

  it('hides the per-attempt audit trail from non-admin viewers', async () => {
    renderBadge(
      { from_model: 'gpt-5', to_model: 'backup', attempt: 1 },
      {
        isAdmin: false,
        adminAttempts: [
          { index: 1, group: 'vip', model: 'backup', status_code: 503 },
        ],
      }
    )

    await userEvent.click(
      screen.getByRole('button', { name: 'Fallback request' })
    )

    expect(await screen.findByText('Original model')).toBeTruthy()
    expect(screen.queryByText('Attempt history (admin)')).toBeNull()
  })

  it('shows the per-attempt audit trail to admins', async () => {
    renderBadge(
      { from_model: 'gpt-5', to_model: 'backup', attempt: 1 },
      {
        isAdmin: true,
        adminAttempts: [
          {
            index: 1,
            group: 'vip',
            model: 'backup',
            channel_id: 42,
            status_code: 503,
            error: 'upstream unavailable',
          },
        ],
      }
    )

    await userEvent.click(
      screen.getByRole('button', { name: 'Fallback request' })
    )

    expect(await screen.findByText('Attempt history (admin)')).toBeTruthy()
    expect(screen.getByText(/upstream unavailable/)).toBeTruthy()
    expect(screen.getByText(/503/)).toBeTruthy()
  })
})
