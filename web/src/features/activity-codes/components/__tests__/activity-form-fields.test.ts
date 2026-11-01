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
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'

import { ACTIVITY_CODE_TYPE } from '../../constants'
import { createActivityBenefitRow } from '../../lib/activity-benefit'
import {
  ACTIVITY_CODE_FORM_DEFAULT_VALUES,
  type ActivityCodeFormValues,
  getActivityCodeFormSchema,
  transformActivityCodeToFormValues,
  transformFormValuesToPayload,
} from '../../lib/activity-form'
import type { ActivityCode } from '../../types'

const t = ((key: string) => key) as unknown as TFunction
const schema = getActivityCodeFormSchema(t)

function validValues(
  overrides: Partial<ActivityCodeFormValues> = {}
): ActivityCodeFormValues {
  return {
    ...ACTIVITY_CODE_FORM_DEFAULT_VALUES,
    code: 'summer-2026',
    name: 'Summer activity',
    group_name: 'vip',
    ...overrides,
  }
}

function activityCode(overrides: Partial<ActivityCode> = {}): ActivityCode {
  return {
    id: 1,
    code: 'summer-2026',
    name: 'Summer activity',
    description: '',
    status: 1,
    group_name: 'vip',
    duration_seconds: 86_400,
    rpm_limit: 0,
    price_quota: 0,
    ratio_mode: 'inherit',
    ratio_override: 0,
    ratio_scope: 'group',
    model_ratio_overrides: null,
    start_time: 0,
    end_time: 0,
    active_end_time: 0,
    sort_weight: 0,
    image_url: '',
    total_limit: 0,
    used_count: 0,
    per_user_limit: 1,
    public_visible: 0,
    type: ACTIVITY_CODE_TYPE.RATIO,
    benefits: null,
    created_by: 1,
    created_time: 0,
    updated_time: 0,
    ...overrides,
  }
}

function issueMessages(values: ActivityCodeFormValues): string[] {
  const parsed = schema.safeParse(values)
  return parsed.success ? [] : parsed.error.issues.map((issue) => issue.message)
}

describe('activity display and expiry defaults', () => {
  it('keeps the sort weight at 0, the background image empty and the end time unset', () => {
    expect(ACTIVITY_CODE_FORM_DEFAULT_VALUES.sort_weight).toBe(0)
    expect(ACTIVITY_CODE_FORM_DEFAULT_VALUES.image_url).toBe('')
    expect(ACTIVITY_CODE_FORM_DEFAULT_VALUES.active_end_time).toBeUndefined()
  })

  it('accepts the default display and expiry values', () => {
    expect(schema.safeParse(validValues()).success).toBe(true)
  })
})

describe('activity sort weight validation', () => {
  it('accepts the whole 0-1000000 range', () => {
    for (const sortWeight of [0, 1, 1_000_000]) {
      expect(
        schema.safeParse(validValues({ sort_weight: sortWeight })).success
      ).toBe(true)
    }
  })

  it('rejects weights outside 0-1000000', () => {
    for (const sortWeight of [-1, 1_000_001]) {
      expect(issueMessages(validValues({ sort_weight: sortWeight }))).toContain(
        'Sort weight must be between 0 and 1000000'
      )
    }
  })
})

describe('activity background image validation', () => {
  it('accepts an empty value, an http(s) URL and a root-relative path', () => {
    for (const imageUrl of [
      '',
      'https://cdn.example.com/summer.png',
      'http://cdn.example.com/summer.png',
      '/static/banner.png',
    ]) {
      expect(
        schema.safeParse(validValues({ image_url: imageUrl })).success,
        imageUrl
      ).toBe(true)
    }
  })

  it('rejects a value that is neither http(s) nor root-relative', () => {
    for (const imageUrl of [
      'cdn.example.com/summer.png',
      'ftp://cdn.example.com/summer.png',
    ]) {
      expect(issueMessages(validValues({ image_url: imageUrl }))).toContain(
        'Enter an http(s) URL or a path starting with /.'
      )
    }
  })

  it('rejects a value longer than 512 characters', () => {
    expect(
      schema.safeParse(validValues({ image_url: `/${'a'.repeat(511)}` }))
        .success
    ).toBe(true)
    expect(
      issueMessages(validValues({ image_url: `/${'a'.repeat(512)}` }))
    ).toContain('Background image URL must be at most 512 characters')
  })
})

describe('activity display and expiry payload', () => {
  it('sends the activity end time in unix seconds', () => {
    const payload = transformFormValuesToPayload(
      validValues({ active_end_time: new Date(1_900_000_000_000) })
    )
    expect(payload.active_end_time).toBe(1_900_000_000)
  })

  it('sends 0 when the activity end time is left empty', () => {
    const payload = transformFormValuesToPayload(validValues())
    expect(payload.active_end_time).toBe(0)
  })

  it('sends the sort weight and a trimmed background image', () => {
    const payload = transformFormValuesToPayload(
      validValues({
        sort_weight: 42,
        image_url: '  https://cdn.example.com/summer.png  ',
      })
    )
    expect(payload.sort_weight).toBe(42)
    expect(payload.image_url).toBe('https://cdn.example.com/summer.png')
  })

  it('keeps the display and expiry fields on a benefit code', () => {
    const payload = transformFormValuesToPayload(
      validValues({
        type: ACTIVITY_CODE_TYPE.BENEFIT,
        benefits: [createActivityBenefitRow({ group_name: 'vip' })],
        active_end_time: new Date(1_900_000_000_000),
        sort_weight: 7,
        image_url: '/static/banner.png',
      })
    )
    expect(payload.active_end_time).toBe(1_900_000_000)
    expect(payload.sort_weight).toBe(7)
    expect(payload.image_url).toBe('/static/banner.png')
  })
})

describe('activity display and expiry backfill', () => {
  it('backfills the stored sort weight and background image', () => {
    const values = transformActivityCodeToFormValues(
      activityCode({
        sort_weight: 120,
        image_url: 'https://cdn.example.com/summer.png',
      })
    )
    expect(values.sort_weight).toBe(120)
    expect(values.image_url).toBe('https://cdn.example.com/summer.png')
  })

  it('backfills the activity end time as a Date', () => {
    const values = transformActivityCodeToFormValues(
      activityCode({ active_end_time: 1_900_000_000 })
    )
    expect(values.active_end_time).toBeInstanceOf(Date)
    expect(values.active_end_time?.getTime()).toBe(1_900_000_000_000)
  })

  it('leaves the activity end time unset when it is 0', () => {
    const values = transformActivityCodeToFormValues(
      activityCode({ active_end_time: 0 })
    )
    expect(values.active_end_time).toBeUndefined()
  })
})
