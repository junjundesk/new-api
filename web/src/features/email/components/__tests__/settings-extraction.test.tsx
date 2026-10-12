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
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { QuotaSettingsSection } from '@/features/system-settings/general/quota-settings-section'
import { MonitoringSettingsSection } from '@/features/system-settings/integrations/monitoring-settings-section'
import { api } from '@/lib/api'

import { renderSettingsForm } from './settings-test-harness'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('email settings extraction', () => {
  test('quota settings link to templates while signup bonus amounts still save locally', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    await renderSettingsForm(
      <QuotaSettingsSection
        defaultValues={{
          QuotaForNewUser: 0,
          PreConsumedQuota: 0,
          QuotaForInviter: 0,
          QuotaForInvitee: 0,
          TopUpLink: '',
          general_setting: { docs_link: '' },
          quota_setting: { enable_free_model_pre_consume: false },
          bonus_setting: {
            signup_bonus_enabled: true,
            signup_bonus_amount: 100,
            signup_bonus_duration: '24',
          },
        }}
      />,
      'templates'
    )
    const amount = await screen.findByRole('spinbutton', {
      name: 'Signup Bonus Amount',
    })

    expect(
      screen.getByRole('link', {
        name: 'Manage bonus email templates in Email Center',
      })
    ).toHaveAttribute('href', '/email?tab=templates')
    expect(
      screen.queryByRole('textbox', { name: 'Bonus Email Subject' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('textbox', { name: 'Bonus Email Template' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('spinbutton', { name: 'New User Quota' })
    ).toBeVisible()
    expect(
      screen.getByRole('spinbutton', { name: 'Inviter Reward' })
    ).toBeVisible()
    expect(
      screen.getByRole('spinbutton', { name: 'Invitee Reward' })
    ).toBeVisible()
    expect(
      screen.getByRole('textbox', { name: 'Signup Bonus Validity' })
    ).toHaveValue('24')
    await user.clear(amount)
    await user.type(amount, '200')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() =>
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
    )
    expect(put.mock.calls).toEqual([
      [
        '/api/option/',
        { key: 'bonus_setting.signup_bonus_amount', value: 200 },
      ],
    ])
  })

  test('monitoring links to notifications while metric retention still saves locally', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    await renderSettingsForm(
      <MonitoringSettingsSection
        defaultValues={{
          'perf_metrics_setting.enabled': true,
          'perf_metrics_setting.flush_interval': 5,
          'perf_metrics_setting.bucket_time': 'hour',
          'perf_metrics_setting.retention_days': 0,
        }}
      />,
      'notifications'
    )
    const retention = await screen.findByRole('spinbutton', {
      name: 'Retention days',
    })

    expect(
      screen.getByRole('link', {
        name: 'Manage quota email notifications in Email Center',
      })
    ).toHaveAttribute('href', '/email?tab=notifications')
    expect(
      screen.queryByRole('spinbutton', { name: 'Quota reminder (tokens)' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('switch', { name: 'Enable model performance metrics' })
    ).toBeChecked()
    await user.clear(retention)
    await user.type(retention, '7')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(
      await screen.findByText('Setting updated successfully')
    ).toBeVisible()
    expect(put.mock.calls).toEqual([
      [
        '/api/option/',
        { key: 'perf_metrics_setting.retention_days', value: 7 },
      ],
    ])
  })
})
