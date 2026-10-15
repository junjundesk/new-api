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

import type { TFunction } from 'i18next'

import { ACTIVITY_RATIO_MODE } from '../../constants'
import {
  type ActivityCodeFormValues,
  getActivityCodeFormSchema,
  transformFormValuesToPayload,
} from '../activity-form'

const t = ((key: string) => key) as unknown as TFunction
const schema = getActivityCodeFormSchema(t)

function validValues(
  overrides: Partial<ActivityCodeFormValues> = {}
): ActivityCodeFormValues {
  return {
    code: '夏日免费',
    name: '中文活动',
    description: '',
    group_name: 'default',
    duration_hours: 24,
    rpm_limit: 5,
    price_dollars: 0,
    ratio_mode: ACTIVITY_RATIO_MODE.OVERRIDE,
    ratio_override: 0,
    start_time: undefined,
    end_time: undefined,
    total_limit: 0,
    per_user_limit: 1,
    public_visible: false,
    ...overrides,
  }
}

describe('activity code form validation', () => {
  test('accepts Chinese activity codes', () => {
    assert.equal(schema.safeParse(validValues()).success, true)
  })

  test('accepts mixed-script codes with digits, underscore and hyphen', () => {
    const parsed = schema.safeParse(validValues({ code: 'hdm-免费_2026' }))
    assert.equal(parsed.success, true)
  })

  test('rejects codes containing spaces or symbols', () => {
    assert.equal(schema.safeParse(validValues({ code: '夏日 免费' })).success, false)
    assert.equal(schema.safeParse(validValues({ code: '免费🎉' })).success, false)
  })

  test('limits code length by characters instead of bytes', () => {
    assert.equal(
      schema.safeParse(validValues({ code: '免'.repeat(64) })).success,
      true
    )
    assert.equal(
      schema.safeParse(validValues({ code: '免'.repeat(65) })).success,
      false
    )
  })

  test('trims surrounding whitespace before validating', () => {
    const parsed = schema.safeParse(validValues({ code: '  夏日免费  ' }))
    assert.equal(parsed.success, true)
    if (parsed.success) {
      assert.equal(parsed.data.code, '夏日免费')
    }
  })
})

describe('activity code form payload', () => {
  test('lowercases latin letters while keeping Chinese characters', () => {
    const payload = transformFormValuesToPayload(
      validValues({ code: 'HDM-免费', price_dollars: 0 })
    )
    assert.equal(payload.code, 'hdm-免费')
    assert.equal(payload.price_quota, 0)
  })
})
