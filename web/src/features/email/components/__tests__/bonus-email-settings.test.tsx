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

import { api } from '@/lib/api'

import { BonusEmailSettings } from '../bonus-email-settings'
import { renderSettingsForm } from './settings-test-harness'

const defaultValues = {
  bonus_setting: {
    bonus_email_subject: 'Existing bonus subject',
    bonus_email_content: '<p>Existing bonus body</p>',
  },
}

const defaultTemplate = `<p>您好 $display_name，</p>
<p>您已收到一笔赠金：<b>$amount</b>。</p>
<p>有效期：$expire_text</p>
$remark_block`

afterEach(() => {
  vi.restoreAllMocks()
})

describe('BonusEmailSettings', () => {
  test('editing the subject saves only its option and leaves financial settings outside the form', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    await renderSettingsForm(
      <BonusEmailSettings defaultValues={defaultValues} />,
      'templates'
    )
    const subject = await screen.findByRole('textbox', {
      name: 'Bonus Email Subject',
    })

    await user.clear(subject)
    await user.type(subject, 'Bonus for $display_name')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() =>
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
    )
    expect(put.mock.calls).toEqual([
      [
        '/api/option/',
        {
          key: 'bonus_setting.bonus_email_subject',
          value: 'Bonus for $display_name',
        },
      ],
    ])
    expect(
      screen.getByRole('textbox', { name: 'Bonus Email Template' })
    ).toHaveValue(defaultValues.bonus_setting.bonus_email_content)
    expect(
      screen.queryByRole('spinbutton', { name: 'Signup Bonus Amount' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('switch', { name: 'Signup Bonus' })
    ).not.toBeInTheDocument()
  })

  test('clearing customized subject and body saves empty strings and displays built-in fallback hints', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    await renderSettingsForm(
      <BonusEmailSettings defaultValues={defaultValues} />,
      'templates'
    )
    const subject = await screen.findByRole('textbox', {
      name: 'Bonus Email Subject',
    })
    const template = screen.getByRole('textbox', {
      name: 'Bonus Email Template',
    })

    await user.clear(subject)
    await user.clear(template)
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() =>
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
    )
    expect(put.mock.calls).toEqual([
      ['/api/option/', { key: 'bonus_setting.bonus_email_subject', value: '' }],
      ['/api/option/', { key: 'bonus_setting.bonus_email_content', value: '' }],
    ])
    expect(subject).toHaveValue('')
    expect(subject).toHaveAttribute('placeholder', 'Bonus arrival notification')
    expect(subject).toHaveAccessibleDescription(
      'Leave empty to use the built-in default subject'
    )
    expect(template).toHaveValue('')
    expect(template).toHaveAttribute('placeholder', defaultTemplate)
    expect(template).toHaveAccessibleDescription(
      /Leave empty to use the built-in template/
    )
    for (const token of [
      '$username',
      '$display_name',
      '$amount',
      '$expire_time',
      '$expire_text',
      '$remark',
      '$remark_block',
      '$site_name',
      '$base_url',
    ]) {
      expect(screen.getByText(token, { selector: 'code' })).toBeVisible()
    }
  })

  test('saving unchanged blank defaults does not persist placeholder content', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    await renderSettingsForm(
      <BonusEmailSettings
        defaultValues={{
          bonus_setting: { bonus_email_subject: '', bonus_email_content: '' },
        }}
      />,
      'templates'
    )

    await user.click(
      await screen.findByRole('button', { name: 'Save Changes' })
    )

    expect(await screen.findByText('No changes to save')).toBeVisible()
    expect(put).not.toHaveBeenCalled()
    expect(
      screen.getByRole('textbox', { name: 'Bonus Email Template' })
    ).toHaveValue('')
  })

  test('a failed body save after a successful subject save retains edits and permits retry', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValueOnce({ data: { success: true } })
      .mockResolvedValueOnce({
        data: { success: false, message: 'Template rejected' },
      })
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    const { router } = await renderSettingsForm(
      <BonusEmailSettings defaultValues={defaultValues} />,
      'templates'
    )
    const subject = await screen.findByRole('textbox', {
      name: 'Bonus Email Subject',
    })
    const template = screen.getByRole('textbox', {
      name: 'Bonus Email Template',
    })

    await user.clear(subject)
    await user.type(subject, 'New bonus subject')
    await user.clear(template)
    await user.type(template, '<p>$amount</p>\n$remark_block')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(await screen.findByText('Template rejected')).toBeVisible()
    expect(screen.getByText('Unsaved changes')).toBeVisible()
    expect(subject).toHaveValue('New bonus subject')
    expect(template).toHaveValue('<p>$amount</p>\n$remark_block')
    await user.click(screen.getByRole('link', { name: 'Send email' }))
    expect(
      await screen.findByRole('alertdialog', { name: 'Unsaved changes' })
    ).toBeVisible()
    expect(router.state.location.search).toEqual({ tab: 'templates' })
    await user.click(screen.getByRole('button', { name: 'Stay' }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() =>
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
    )
    expect(put.mock.calls).toEqual([
      [
        '/api/option/',
        {
          key: 'bonus_setting.bonus_email_subject',
          value: 'New bonus subject',
        },
      ],
      [
        '/api/option/',
        {
          key: 'bonus_setting.bonus_email_content',
          value: '<p>$amount</p>\n$remark_block',
        },
      ],
      [
        '/api/option/',
        {
          key: 'bonus_setting.bonus_email_subject',
          value: 'New bonus subject',
        },
      ],
      [
        '/api/option/',
        {
          key: 'bonus_setting.bonus_email_content',
          value: '<p>$amount</p>\n$remark_block',
        },
      ],
    ])
    await user.click(screen.getByRole('link', { name: 'Send email' }))
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ tab: 'send' })
    )
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })
})
