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
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, expect, test, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import { CompactDateTimeRangePicker } from '../compact-date-time-range-picker'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

test('selecting Yesterday on March 1 includes the whole previous local leap day', async () => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2024, 2, 1, 0, 15))
  const i18n = createInstance()
  await i18n.init({ lng: 'zhCN', resources: { zhCN: zh } })
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(
    <I18nextProvider i18n={i18n}>
      <CompactDateTimeRangePicker onChange={onChange} />
    </I18nextProvider>
  )

  await user.click(screen.getByRole('button', { name: i18n.t('Date Range') }))
  await user.click(screen.getByRole('button', { name: '昨日' }))

  expect(onChange).toHaveBeenCalledWith({
    start: new Date(2024, 1, 29, 0, 0, 0, 0),
    end: new Date(2024, 1, 29, 23, 59, 59, 999),
  })
})
