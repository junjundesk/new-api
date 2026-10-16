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
import i18next from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'

import type { UserProfile } from '../../../types'
import { NotificationTab } from '../notification-tab'

const OPERATIONAL_SWITCH = 'Operational Notifications'

function buildProfile(setting?: string): UserProfile {
  return {
    id: 1,
    username: 'alice',
    display_name: 'Alice',
    role: ROLE.USER,
    group: 'default',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    status: 1,
    aff_count: 0,
    aff_quota: 0,
    aff_history_quota: 0,
    created_time: 0,
    setting,
  }
}

async function renderNotificationTab(setting?: string) {
  await i18next.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
  render(
    <I18nextProvider i18n={i18next}>
      <NotificationTab profile={buildProfile(setting)} onUpdate={() => {}} />
    </I18nextProvider>
  )
  return screen.getByRole('switch', { name: OPERATIONAL_SWITCH })
}

beforeEach(() => {
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('NotificationTab operational notifications', () => {
  test('an absent flag keeps operational notifications on by default', async () => {
    const toggle = await renderNotificationTab('{}')
    expect(toggle).toBeChecked()
  })

  test('a stored false flag renders the switch off', async () => {
    const toggle = await renderNotificationTab(
      '{"operational_notify_enabled":false}'
    )
    expect(toggle).not.toBeChecked()
  })

  test('turning the switch off saves the opt-out with the other settings', async () => {
    const user = userEvent.setup()
    const toggle = await renderNotificationTab(
      '{"operational_notify_enabled":true}'
    )
    await user.click(toggle)
    expect(toggle).not.toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Save Settings' }))

    expect(api.put).toHaveBeenCalledExactlyOnceWith(
      '/api/user/setting',
      expect.objectContaining({ operational_notify_enabled: false })
    )
  })
})
