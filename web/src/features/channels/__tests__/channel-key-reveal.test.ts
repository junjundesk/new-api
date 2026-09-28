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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getChannelKey } from '@/features/channels/api'
import { api } from '@/lib/api'

type CapturedRequest = {
  url?: string
  method?: string
  headers?: Record<string, unknown>
}

describe('channel key reveal', () => {
  test('loads the saved key without sending any security proof header', async () => {
    const captured: CapturedRequest[] = []
    const originalAdapter = api.defaults.adapter
    api.defaults.adapter = (async (config: unknown) => {
      const request = config as CapturedRequest
      captured.push(request)
      return {
        data: { success: true, data: { key: 'sk-saved-channel-key' } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: request,
      }
    }) as unknown as typeof api.defaults.adapter
    try {
      const response = await getChannelKey(7)

      assert.equal(response.data?.key, 'sk-saved-channel-key')
      assert.equal(captured.length, 1)
      assert.equal(captured[0].url, '/api/channel/7/key')
      assert.equal(captured[0].method, 'post')
      const headerNames = Object.keys(captured[0].headers ?? {}).map((name) =>
        name.toLowerCase()
      )
      assert.equal(headerNames.includes('x-security-proof'), false)
    } finally {
      api.defaults.adapter = originalAdapter
    }
  })
})
