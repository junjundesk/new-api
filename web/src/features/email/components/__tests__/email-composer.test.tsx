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
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { EmailComposer } from '../email-composer'
import { renderSettingsForm } from './settings-test-harness'

const originalGetAnimations = Object.getOwnPropertyDescriptor(
  Element.prototype,
  'getAnimations'
)

beforeEach(() => {
  // happy-dom lacks the Web Animations API used by the real ScrollArea.
  if (!originalGetAnimations) {
    Object.defineProperty(Element.prototype, 'getAnimations', {
      configurable: true,
      value: (): Animation[] => [],
    })
  }
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        items: [
          {
            id: 41,
            username: 'alice',
            display_name: 'Alice',
            email: 'alice@example.com',
          },
          {
            id: 42,
            username: 'missing',
            display_name: 'No Address',
            email: '   ',
          },
          {
            id: 43,
            username: 'bob',
            display_name: 'Bob',
            email: 'bob@example.com',
          },
          {
            id: 44,
            username: 'carol',
            display_name: 'Carol',
            email: 'carol@example.com',
          },
        ],
      },
    },
  })
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: { queued: 2, sent: 2, failed: 0, skipped: 0 },
    },
  })
})

afterEach(() => {
  vi.restoreAllMocks()
  if (!originalGetAnimations) {
    Reflect.deleteProperty(Element.prototype, 'getAnimations')
  }
})

describe('EmailComposer', () => {
  test('searching recipients excludes users without email and sends only the selected user IDs', async () => {
    const user = userEvent.setup()
    await renderSettingsForm(<EmailComposer />, 'send')
    const search = await screen.findByPlaceholderText(
      'Search by username, email or user ID'
    )
    const subject = screen.getByRole('textbox', { name: 'Subject' })
    const content = screen.getByRole('textbox', { name: 'Content' })

    await user.type(subject, '  Service notice  ')
    await user.type(content, ' <p>Hello team</p>\nPlease review. ')
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    await user.type(search, '  example.com  ')
    await user.keyboard('{Enter}')

    const noEmail = await screen.findByRole('checkbox', {
      name: 'No Address No email',
    })
    expect(api.get).toHaveBeenCalledExactlyOnceWith(
      '/api/user/search?keyword=example.com&group=&p=1&page_size=20'
    )
    expect(noEmail).toHaveAttribute('aria-disabled', 'true')
    await user.click(noEmail)
    expect(noEmail).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    await user.click(
      screen.getByRole('checkbox', { name: 'Alice alice@example.com' })
    )
    await user.click(
      screen.getByRole('checkbox', { name: 'Bob bob@example.com' })
    )
    expect(
      screen.getByRole('checkbox', { name: 'Carol carol@example.com' })
    ).not.toBeChecked()
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(
      await screen.findByText('Email sent to 2 recipients (0 failed)')
    ).toBeVisible()
    expect(api.post).toHaveBeenCalledExactlyOnceWith('/api/user/email/send', {
      all_users: false,
      user_ids: [41, 43],
      subject: 'Service notice',
      content: ' <p>Hello team</p>\nPlease review. ',
    })
    expect(subject).toHaveValue('')
    expect(content).toHaveValue('')
    expect(search).toHaveValue('')
    expect(
      screen.queryByRole('button', { name: 'Remove' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  })

  test('switching from selected recipients to broadcast sends no user IDs and reports async queueing', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, data: { async: true, queued: 8, skipped: 1 } },
    })
    const user = userEvent.setup()
    await renderSettingsForm(<EmailComposer />, 'send')
    await user.type(
      await screen.findByPlaceholderText(
        'Search by username, email or user ID'
      ),
      'alice'
    )
    await user.click(screen.getByRole('button', { name: 'Search' }))
    await user.click(
      await screen.findByRole('checkbox', { name: 'Alice alice@example.com' })
    )
    await user.click(screen.getByRole('checkbox', { name: 'All users' }))
    await user.type(
      screen.getByRole('textbox', { name: 'Subject' }),
      '  Maintenance  '
    )
    await user.type(
      screen.getByRole('textbox', { name: 'Content' }),
      '<p>Scheduled maintenance</p>'
    )
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(
      await screen.findByText('Broadcast queued for 8 recipients')
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        '1 users were skipped because they have no email address'
      )
    ).toBeVisible()
    expect(api.post).toHaveBeenCalledExactlyOnceWith('/api/user/email/send', {
      all_users: true,
      user_ids: undefined,
      subject: 'Maintenance',
      content: '<p>Scheduled maintenance</p>',
    })
    expect(screen.queryByText(/Email sent to/)).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Subject' })).toHaveValue('')
    expect(screen.getByRole('textbox', { name: 'Content' })).toHaveValue('')
    await user.click(screen.getByRole('checkbox', { name: 'Selected users' }))
    expect(
      screen.queryByRole('button', { name: 'Remove' })
    ).not.toBeInTheDocument()
  })

  test.each(['response', 'network'] as const)(
    'a failed %s send preserves the draft and selected recipient for retry',
    async (failure) => {
      if (failure === 'response') {
        vi.mocked(api.post).mockResolvedValueOnce({
          data: { success: false, message: 'Delivery unavailable' },
        })
      } else {
        vi.mocked(api.post).mockRejectedValueOnce(
          new Error('Delivery unavailable')
        )
      }
      vi.mocked(api.post).mockResolvedValue({
        data: {
          success: true,
          data: { queued: 1, sent: 1, failed: 0, skipped: 0 },
        },
      })
      const user = userEvent.setup()
      await renderSettingsForm(<EmailComposer />, 'send')
      const search = await screen.findByPlaceholderText(
        'Search by username, email or user ID'
      )
      await user.type(search, 'alice')
      await user.click(screen.getByRole('button', { name: 'Search' }))
      const recipient = await screen.findByRole('checkbox', {
        name: 'Alice alice@example.com',
      })
      await user.click(recipient)
      const subject = screen.getByRole('textbox', { name: 'Subject' })
      const content = screen.getByRole('textbox', { name: 'Content' })
      await user.type(subject, '  Important notice  ')
      await user.type(content, '<p>Keep this draft</p>\nMore details.')
      await user.click(screen.getByRole('button', { name: 'Send' }))

      expect(await screen.findByText('Delivery unavailable')).toBeVisible()
      expect(subject).toHaveValue('  Important notice  ')
      expect(content).toHaveValue('<p>Keep this draft</p>\nMore details.')
      expect(search).toHaveValue('alice')
      expect(recipient).toBeChecked()
      expect(screen.getByRole('button', { name: 'Remove' })).toBeVisible()
      expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
      await user.click(screen.getByRole('button', { name: 'Send' }))

      expect(
        await screen.findByText('Email sent to 1 recipients (0 failed)')
      ).toBeVisible()
      expect(vi.mocked(api.post).mock.calls).toEqual([
        [
          '/api/user/email/send',
          {
            all_users: false,
            user_ids: [41],
            subject: 'Important notice',
            content: '<p>Keep this draft</p>\nMore details.',
          },
        ],
        [
          '/api/user/email/send',
          {
            all_users: false,
            user_ids: [41],
            subject: 'Important notice',
            content: '<p>Keep this draft</p>\nMore details.',
          },
        ],
      ])
      expect(subject).toHaveValue('')
      expect(content).toHaveValue('')
    }
  )
})
