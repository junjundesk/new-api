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

import type { UsageLog } from '../../data/schema'
import { formatModelName } from '../format'

function makeLog(other: Record<string, unknown>): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1700000000,
    type: 2,
    content: '',
    username: 'admin',
    token_name: 'key',
    model_name: 'gpt-4o-mini',
    quota: 1,
    prompt_tokens: 1,
    completion_tokens: 1,
    use_time: 1,
    is_stream: false,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: '',
    upstream_request_id: '',
  }
}

describe('log model mapping visibility', () => {
  const mappedOther = {
    model_ratio: 0.1,
    admin_info: {
      is_model_mapped: true,
      upstream_model_name: 'qwen-turbo',
    },
  }

  test('admins see the upstream model behind the mapping flag', () => {
    const info = formatModelName(makeLog(mappedOther), true)
    assert.equal(info.name, 'gpt-4o-mini')
    assert.equal(info.isMapped, true)
    assert.equal(info.actualModel, 'qwen-turbo')
  })

  test('non-admin viewers never receive the mapping details', () => {
    const info = formatModelName(makeLog(mappedOther), false)
    assert.equal(info.name, 'gpt-4o-mini')
    assert.equal(info.isMapped, false)
    assert.equal(info.actualModel, undefined)
  })

  test('admins still see legacy top-level mapping fields', () => {
    // Logs written before the fields moved under admin_info carry them at the
    // top level. Admin viewers keep the indicator for those historical entries.
    const legacyOther = {
      is_model_mapped: true,
      upstream_model_name: 'qwen-turbo',
    }
    const info = formatModelName(makeLog(legacyOther), true)
    assert.equal(info.isMapped, true)
    assert.equal(info.actualModel, 'qwen-turbo')
  })

  test('non-admins never see legacy top-level mapping fields either', () => {
    const legacyOther = {
      is_model_mapped: true,
      upstream_model_name: 'qwen-turbo',
    }
    const info = formatModelName(makeLog(legacyOther), false)
    assert.equal(info.isMapped, false)
    assert.equal(info.actualModel, undefined)
  })

  test('logs without a mapping stay unflagged for admins', () => {
    const info = formatModelName(makeLog({ model_ratio: 0.1 }), true)
    assert.equal(info.isMapped, false)
    assert.equal(info.actualModel, undefined)
  })
})
