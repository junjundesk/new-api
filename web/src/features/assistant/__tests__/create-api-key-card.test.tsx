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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { fetchTokenKey } from '@/features/keys/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import { CreateApiKeyCard } from '../components/create-api-key-card'
import { parseCreateApiKeyResult } from '../lib/tool-data'
import type { AssistantCreateApiKeyData } from '../types'

vi.mock('@/features/keys/api', () => ({ fetchTokenKey: vi.fn() }))
vi.mock('@/lib/copy-to-clipboard', () => ({ copyToClipboard: vi.fn() }))

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

const baseData: AssistantCreateApiKeyData = {
  id: 7,
  name: 'my-script',
  maskedKey: 'sk-a**********wxyz',
  statusText: 'enabled',
  unlimitedQuota: true,
  expiredTime: -1,
  neverExpires: true,
}

function renderCard(data: AssistantCreateApiKeyData) {
  return render(
    <I18nextProvider i18n={i18n}>
      <CreateApiKeyCard data={data} />
    </I18nextProvider>
  )
}

beforeEach(() => {
  vi.mocked(fetchTokenKey).mockReset()
  vi.mocked(copyToClipboard).mockReset()
  vi.mocked(copyToClipboard).mockResolvedValue(true)
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('created API key card', () => {
  it('renders the backend-masked key without loading the full secret', () => {
    renderCard(baseData)

    expect(screen.getByText('my-script')).toBeVisible()
    expect(screen.getByText('sk-a**********wxyz')).toBeVisible()
    expect(screen.getByText('Unlimited quota')).toBeVisible()
    expect(screen.getByText('Never expires')).toBeVisible()
    expect(fetchTokenKey).not.toHaveBeenCalled()
  })

  it('reveals the full key only after an explicit click', async () => {
    const user = userEvent.setup()
    vi.mocked(fetchTokenKey).mockResolvedValue({
      success: true,
      data: { key: 'secret-full-value' },
    })
    renderCard(baseData)

    expect(screen.queryByDisplayValue('sk-secret-full-value')).toBeNull()

    await user.click(screen.getByRole('button', { name: 'View full key' }))

    expect(
      await screen.findByDisplayValue('sk-secret-full-value')
    ).toBeVisible()
    expect(fetchTokenKey).toHaveBeenCalledWith(7)
  })

  it('copies the revealed key to the clipboard', async () => {
    const user = userEvent.setup()
    vi.mocked(fetchTokenKey).mockResolvedValue({
      success: true,
      data: { key: 'secret-full-value' },
    })
    renderCard(baseData)

    await user.click(screen.getByRole('button', { name: 'View full key' }))
    await screen.findByDisplayValue('sk-secret-full-value')
    await user.click(screen.getByRole('button', { name: 'Copy' }))

    expect(copyToClipboard).toHaveBeenCalledWith('sk-secret-full-value')
    expect(await screen.findByText('Copied!')).toBeVisible()
  })

  it('shows an error when the full key cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(fetchTokenKey).mockResolvedValue({
      success: false,
      message: 'Token not found',
    })
    renderCard(baseData)

    await user.click(screen.getByRole('button', { name: 'View full key' }))

    expect(await screen.findByText('Token not found')).toBeVisible()
  })

  it('disables the reveal button when the token id is missing', () => {
    renderCard({ ...baseData, id: undefined })

    expect(screen.getByRole('button', { name: 'View full key' })).toBeDisabled()
  })

  it('adds the sk- prefix to a backend masked key that omits it', () => {
    const parsed = parseCreateApiKeyResult({
      id: 3,
      name: 'masked',
      masked_key: 'abcd**********wxyz',
      unlimited_quota: true,
      expired_time: -1,
      never_expires: true,
    })

    expect(parsed?.maskedKey).toBe('sk-abcd**********wxyz')
  })

  it('rejects a create result without a name or masked key', () => {
    expect(parseCreateApiKeyResult({ id: 3 })).toBeNull()
    expect(parseCreateApiKeyResult('not-an-object')).toBeNull()
  })

  it('shows the formatted expiry date when the key expires', () => {
    renderCard({
      ...baseData,
      neverExpires: false,
      expiredTime: 1730000000,
    })

    expect(screen.getByText(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}/)).toBeVisible()
  })
})
