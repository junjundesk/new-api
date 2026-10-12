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
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { NotificationSettings } from '../notification-settings'
import { renderSettingsForm } from './settings-test-harness'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('NotificationSettings', () => {
  test.each(['250', '0', ''])(
    'changing the threshold to "%s" saves only the global threshold as a string',
    async (value) => {
      const put = vi
        .spyOn(api, 'put')
        .mockResolvedValue({ data: { success: true } })
      const user = userEvent.setup()
      await renderSettingsForm(
        <NotificationSettings
          defaultValues={{ QuotaRemindThreshold: '100' }}
        />,
        'notifications'
      )
      const threshold = await screen.findByRole('spinbutton', {
        name: 'Quota reminder (tokens)',
      })

      await user.clear(threshold)
      if (value) await user.type(threshold, value)
      await user.click(screen.getByRole('button', { name: 'Save Changes' }))

      await waitFor(() =>
        expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
      )
      expect(put.mock.calls).toEqual([
        ['/api/option/', { key: 'QuotaRemindThreshold', value }],
      ])
      expect(
        screen.queryByRole('switch', {
          name: 'Enable model performance metrics',
        })
      ).not.toBeInTheDocument()
    }
  )

  test('a negative threshold blocks saving and exposes the validation error', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    await renderSettingsForm(
      <NotificationSettings defaultValues={{ QuotaRemindThreshold: '100' }} />,
      'notifications'
    )
    const threshold = await screen.findByRole('spinbutton', {
      name: 'Quota reminder (tokens)',
    })

    await user.clear(threshold)
    await user.type(threshold, '-1')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(
      await screen.findByText('Enter a non-negative number or leave empty')
    ).toBeVisible()
    expect(threshold).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText('Unsaved changes')).toBeVisible()
    expect(put).not.toHaveBeenCalled()
  })

  test('whitespace-only changes against the normalized saved threshold do not send an update', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    await renderSettingsForm(
      <NotificationSettings
        defaultValues={{ QuotaRemindThreshold: ' 100 ' }}
      />,
      'notifications'
    )
    const threshold = await screen.findByRole('spinbutton', {
      name: 'Quota reminder (tokens)',
    })

    await user.clear(threshold)
    await user.type(threshold, '100')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(await screen.findByText('No changes to save')).toBeVisible()
    expect(put).not.toHaveBeenCalled()
  })

  test.each(['response', 'network'] as const)(
    'a failed %s save retains the threshold and blocks changing tabs until retry succeeds',
    async (failure) => {
      const put = vi.spyOn(api, 'put')
      if (failure === 'response') {
        put.mockResolvedValueOnce({
          data: { success: false, message: 'Threshold rejected' },
        })
      } else {
        put.mockRejectedValueOnce(new Error('Threshold rejected'))
      }
      put.mockResolvedValue({ data: { success: true } })
      const user = userEvent.setup()
      const { router } = await renderSettingsForm(
        <NotificationSettings
          defaultValues={{ QuotaRemindThreshold: '100' }}
        />,
        'notifications'
      )
      const threshold = await screen.findByRole('spinbutton', {
        name: 'Quota reminder (tokens)',
      })

      await user.clear(threshold)
      await user.type(threshold, '250')
      await user.click(screen.getByRole('button', { name: 'Save Changes' }))

      expect(await screen.findByText('Threshold rejected')).toBeVisible()
      expect(threshold).toHaveValue(250)
      expect(screen.getByText('Unsaved changes')).toBeVisible()
      await user.click(screen.getByRole('link', { name: 'Send email' }))
      expect(
        await screen.findByRole('alertdialog', { name: 'Unsaved changes' })
      ).toBeVisible()
      expect(router.state.location.search).toEqual({ tab: 'notifications' })
      await user.click(screen.getByRole('button', { name: 'Stay' }))
      await waitFor(() =>
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
      )
      await user.click(screen.getByRole('button', { name: 'Save Changes' }))

      await waitFor(() =>
        expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
      )
      expect(put.mock.calls).toEqual([
        ['/api/option/', { key: 'QuotaRemindThreshold', value: '250' }],
        ['/api/option/', { key: 'QuotaRemindThreshold', value: '250' }],
      ])
      await user.click(screen.getByRole('link', { name: 'Send email' }))
      await waitFor(() =>
        expect(router.state.location.search).toEqual({ tab: 'send' })
      )
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    }
  )

  test('while a threshold save is pending the toolbar prevents duplicate submissions', async () => {
    let completeSave: (value: { data: { success: boolean } }) => void = () =>
      undefined
    const response = new Promise<{ data: { success: boolean } }>((resolve) => {
      completeSave = resolve
    })
    const put = vi.spyOn(api, 'put').mockReturnValue(response)
    const user = userEvent.setup()
    await renderSettingsForm(
      <NotificationSettings defaultValues={{ QuotaRemindThreshold: '100' }} />,
      'notifications'
    )
    const threshold = await screen.findByRole('spinbutton', {
      name: 'Quota reminder (tokens)',
    })

    await user.clear(threshold)
    await user.type(threshold, '250')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    const saving = await screen.findByRole('button', { name: 'Saving...' })
    expect(saving).toBeDisabled()
    await user.click(saving)
    expect(put).toHaveBeenCalledTimes(1)

    await act(async () => {
      completeSave({ data: { success: true } })
    })

    expect(
      await screen.findByRole('button', { name: 'Save Changes' })
    ).toBeEnabled()
    expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
  })
})
