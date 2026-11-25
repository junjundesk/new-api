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
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { useTopupInfo } from '../use-topup-info'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

test('failed catalog reads offer a retry instead of gateway presets; a successful retry restores card tiers', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  const store = {
    url: 'https://cards.test/',
    tiers: [{ id: 9, name: 'Standard', quota: 100, bonus_quota: 8 }],
  }
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({
      data: { success: false, message: 'catalog unavailable' },
    })
    .mockResolvedValue({
      data: {
        success: true,
        data: { card_store: store, amount_options: [1, 2], pay_methods: [] },
      },
    })
  const { result } = renderHook(() => useTopupInfo())
  await waitFor(() => expect(result.current.failed).toBe(true))
  expect(result.current.topupInfo).toBeNull()
  expect(result.current.presetAmounts).toEqual([])
  await act(() => result.current.refetch())
  expect(result.current.failed).toBe(false)
  expect(result.current.topupInfo?.card_store).toEqual(store)
  expect(result.current.presetAmounts).toEqual([])
})
