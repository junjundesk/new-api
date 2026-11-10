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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { updateApiKeyGroup } from '../../api'
import { ApiKeyGroupCell } from '../api-key-group-cell'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: { count?: number }) =>
      key.replace('{{count}}', String(options?.count ?? '')),
  }),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('../../api', () => ({ updateApiKeyGroup: vi.fn() }))

beforeEach(() => {
  vi.mocked(updateApiKeyGroup).mockResolvedValue({ success: true })
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('API key model-chain group switching', () => {
  it.each([
    { fallbackEnabled: true, chainType: 'models' as const, blocked: true },
    { fallbackEnabled: false, chainType: 'models' as const, blocked: false },
    { fallbackEnabled: true, chainType: 'groups' as const, blocked: false },
  ])(
    'checks $chainType chains with fallback $fallbackEnabled before saving',
    async ({ fallbackEnabled, chainType, blocked }) => {
      const user = userEvent.setup()
      const onGroupChanged = vi.fn()
      render(
        <ApiKeyGroupCell
          apiKeyId={42}
          group='default'
          crossGroupRetry={false}
          fallbackEnabled={fallbackEnabled}
          shouldReduceMotion
          onGroupChanged={onGroupChanged}
          options={[
            { value: 'default', label: 'default' },
            {
              value: 'chain:routing',
              label: 'Routing chain',
              chainType,
              chainModelCount: 2,
              chainGroupCount: 1,
            },
          ]}
        />
      )

      await user.click(screen.getByRole('combobox'))
      await user.click(
        await screen.findByRole('option', { name: /Routing chain/ })
      )

      if (blocked) {
        await waitFor(() => {
          expect(toast.error).toHaveBeenCalledWith(
            'Model-based group chains do not support cross-model fallback. Disable fallback before selecting this chain; your draft will be kept.'
          )
        })
        expect(updateApiKeyGroup).not.toHaveBeenCalled()
        expect(onGroupChanged).not.toHaveBeenCalled()
        expect(screen.getByRole('combobox').textContent).toContain('default')
        return
      }

      await waitFor(() => {
        expect(updateApiKeyGroup).toHaveBeenCalledWith(
          42,
          'chain:routing',
          false
        )
        expect(onGroupChanged).toHaveBeenCalledTimes(1)
      })
      expect(toast.error).not.toHaveBeenCalled()
    }
  )
})
