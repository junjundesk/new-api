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

import { LOG_TYPE_FILTERS } from '@/features/usage-logs/constants'
import { renderAuditContent } from '@/features/usage-logs/lib/format'

import { USER_LOG_TYPE_VALUES } from '../constants'
import { buildUserLogsRequest } from '../lib/request'

// 用最简 t 保留键本身并替换占位符：验证的是「动作是否命中模板」这个契约，
// 而不是某种语言的具体译文。
const fakeT = (key: string, options?: Record<string, unknown>) =>
  options
    ? key.replaceAll(/\{\{(\w+)\}\}/g, (_match, name: string) =>
        String(options[name] ?? '')
      )
    : key

describe('user logs feature contracts', () => {
  test('account activity is queried through the account log group', () => {
    const admin = buildUserLogsRequest({ p: 1, page_size: 20 }, true)
    assert.equal(admin.path, '/api/log')
    assert.equal(admin.params.log_group, 'account')

    const self = buildUserLogsRequest({ p: 1, page_size: 20 }, false)
    assert.equal(self.path, '/api/log/self')
    assert.equal(self.params.log_group, 'account')
  })

  test('usage log type filters no longer offer account activity types', () => {
    const offered = LOG_TYPE_FILTERS.map((filter) => String(filter.value))
      .filter((value) => value !== '0')
      .sort()
    assert.deepEqual(offered, ['2', '5', '6'])
    for (const value of USER_LOG_TYPE_VALUES) {
      assert.equal(offered.includes(String(value)), false)
    }
  })

  test('key and registration actions render template content instead of raw text', () => {
    const cases: Array<[string, Record<string, string | number>, string]> = [
      ['token.create', { name: 'prod' }, 'Created API key prod'],
      ['token.update', { name: 'prod' }, 'Updated API key prod'],
      ['token.delete', { name: 'prod', id: 7 }, 'Deleted API key prod (ID: 7)'],
      ['token.delete_batch', { count: 3 }, 'Deleted 3 API keys'],
      ['token.enable', { name: 'prod' }, 'Enabled API key prod'],
      ['token.disable', { name: 'prod' }, 'Disabled API key prod'],
      ['user.register', { username: 'alice' }, 'Registered account alice'],
    ]

    for (const [action, params, expected] of cases) {
      assert.equal(
        renderAuditContent({ op: { action, params } }, fakeT),
        expected,
        action
      )
    }
    assert.equal(renderAuditContent({ op: { action: 'unknown.action' } }, fakeT), null)
  })
})
