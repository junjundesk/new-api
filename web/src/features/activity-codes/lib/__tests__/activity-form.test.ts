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

import {
  ACTIVITY_BENEFIT_SCOPE,
  ACTIVITY_BENEFIT_TOKEN_MODE,
  ACTIVITY_CODE_TYPE,
  ACTIVITY_RATIO_MODE,
  ACTIVITY_RATIO_SCOPE,
} from '../../constants'
import type { ActivityCode } from '../../types'
import {
  type ActivityBenefitRow,
  createActivityBenefitRow,
} from '../activity-benefit'
import {
  type ActivityCodeFormValues,
  getActivityCodeFormSchema,
  transformActivityCodeToFormValues,
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
    ratio_scope: ACTIVITY_RATIO_SCOPE.GROUP,
    model_ratio_overrides: [],
    start_time: undefined,
    end_time: undefined,
    total_limit: 0,
    per_user_limit: 1,
    public_visible: false,
    type: ACTIVITY_CODE_TYPE.RATIO,
    benefits: [],
    ...overrides,
  }
}

function modelScopeValues(
  overrides: Partial<ActivityCodeFormValues> = {}
): ActivityCodeFormValues {
  return validValues({
    ratio_scope: ACTIVITY_RATIO_SCOPE.MODELS,
    ratio_mode: ACTIVITY_RATIO_MODE.INHERIT,
    ratio_override: 0,
    model_ratio_overrides: [{ model: 'gpt-5', ratio: '0' }],
    ...overrides,
  })
}

function activityCode(overrides: Partial<ActivityCode> = {}): ActivityCode {
  return {
    id: 1,
    code: 'hdmmodels',
    name: '指定模型活动',
    description: '',
    status: 1,
    group_name: 'vip',
    duration_seconds: 86400,
    rpm_limit: 0,
    price_quota: 0,
    ratio_mode: ACTIVITY_RATIO_MODE.INHERIT,
    ratio_override: 0,
    ratio_scope: ACTIVITY_RATIO_SCOPE.MODELS,
    model_ratio_overrides: { 'gpt-5': 0, o3: 0.5 },
    start_time: 0,
    end_time: 0,
    total_limit: 0,
    used_count: 0,
    per_user_limit: 1,
    public_visible: 1,
    created_by: 1,
    created_time: 0,
    updated_time: 0,
    type: ACTIVITY_CODE_TYPE.RATIO,
    benefits: null,
    ...overrides,
  } as ActivityCode
}

function benefitRow(
  overrides: Partial<ActivityBenefitRow> = {}
): ActivityBenefitRow {
  return createActivityBenefitRow({ group_name: 'vip', ...overrides })
}

function benefitValues(
  rows: ActivityBenefitRow[],
  overrides: Partial<ActivityCodeFormValues> = {}
): ActivityCodeFormValues {
  return validValues({
    type: ACTIVITY_CODE_TYPE.BENEFIT,
    benefits: rows,
    ...overrides,
  })
}

function benefitActivityCode(overrides: Partial<ActivityCode> = {}): ActivityCode {
  return activityCode({
    type: ACTIVITY_CODE_TYPE.BENEFIT,
    benefits: [
      {
        key: 'benefit-1',
        group_name: 'vip',
        scope: 'model',
        model_name: 'gpt-5',
        rpm_limit: 60,
        token_total: 1_000_000,
      },
    ],
    ...overrides,
  })
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

  test('the whole-group scope ignores leftover model rows', () => {
    const parsed = schema.safeParse(
      validValues({ model_ratio_overrides: [{ model: '', ratio: '' }] })
    )
    assert.equal(parsed.success, true)
  })

  test('the selected-models scope requires at least one model', () => {
    const parsed = schema.safeParse(modelScopeValues({ model_ratio_overrides: [] }))
    assert.equal(parsed.success, false)
  })

  test('the selected-models scope rejects a row without a model or ratio', () => {
    assert.equal(
      schema.safeParse(
        modelScopeValues({ model_ratio_overrides: [{ model: '  ', ratio: '0' }] })
      ).success,
      false
    )
    assert.equal(
      schema.safeParse(
        modelScopeValues({ model_ratio_overrides: [{ model: 'gpt-5', ratio: '' }] })
      ).success,
      false
    )
  })

  test('the selected-models scope rejects the same model twice', () => {
    const parsed = schema.safeParse(
      modelScopeValues({
        model_ratio_overrides: [
          { model: 'gpt-5', ratio: '0' },
          { model: 'gpt-5', ratio: '0.5' },
        ],
      })
    )
    assert.equal(parsed.success, false)
  })

  test('the selected-models scope rejects ratios outside 0-1000', () => {
    for (const ratio of ['-1', '1001', 'abc']) {
      assert.equal(
        schema.safeParse(
          modelScopeValues({ model_ratio_overrides: [{ model: 'gpt-5', ratio }] })
        ).success,
        false,
        `${ratio} must be rejected`
      )
    }
  })

  test('the selected-models scope accepts a single free model', () => {
    const parsed = schema.safeParse(
      modelScopeValues({ model_ratio_overrides: [{ model: 'gpt-5', ratio: '0' }] })
    )
    assert.equal(parsed.success, true)
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

  test('the whole-group scope keeps the group ratio and clears the model list', () => {
    const payload = transformFormValuesToPayload(
      validValues({
        ratio_mode: ACTIVITY_RATIO_MODE.OVERRIDE,
        ratio_override: 0.5,
        model_ratio_overrides: [{ model: 'gpt-5', ratio: '0' }],
      })
    )
    assert.equal(payload.ratio_scope, ACTIVITY_RATIO_SCOPE.GROUP)
    assert.equal(payload.ratio_mode, ACTIVITY_RATIO_MODE.OVERRIDE)
    assert.equal(payload.ratio_override, 0.5)
    assert.deepEqual(payload.model_ratio_overrides, {})
  })

  test('an inherited group ratio never sends a stale override value', () => {
    const payload = transformFormValuesToPayload(
      validValues({ ratio_mode: ACTIVITY_RATIO_MODE.INHERIT, ratio_override: 5 })
    )
    assert.equal(payload.ratio_override, 0)
  })

  test('the selected-models scope sends per-model ratios and inherits the group ratio', () => {
    const payload = transformFormValuesToPayload(
      modelScopeValues({
        ratio_mode: ACTIVITY_RATIO_MODE.OVERRIDE,
        ratio_override: 0.9,
        model_ratio_overrides: [
          { model: ' gpt-5 ', ratio: '0' },
          { model: 'o3', ratio: '0.5' },
        ],
      })
    )
    assert.equal(payload.ratio_scope, ACTIVITY_RATIO_SCOPE.MODELS)
    assert.equal(payload.ratio_mode, ACTIVITY_RATIO_MODE.INHERIT)
    assert.equal(payload.ratio_override, 0)
    assert.deepEqual(payload.model_ratio_overrides, { 'gpt-5': 0, o3: 0.5 })
  })
})

describe('activity code to form values', () => {
  test('maps stored model overrides to sorted rows', () => {
    const values = transformActivityCodeToFormValues(activityCode())
    assert.equal(values.ratio_scope, ACTIVITY_RATIO_SCOPE.MODELS)
    assert.deepEqual(values.model_ratio_overrides, [
      { model: 'gpt-5', ratio: '0' },
      { model: 'o3', ratio: '0.5' },
    ])
  })

  test('treats a missing scope or map as the whole group', () => {
    const values = transformActivityCodeToFormValues(
      activityCode({
        ratio_scope: '',
        model_ratio_overrides: null,
      })
    )
    assert.equal(values.ratio_scope, ACTIVITY_RATIO_SCOPE.GROUP)
    assert.deepEqual(values.model_ratio_overrides, [])
  })
})
describe('activity benefit form validation', () => {
  test('accepts a whole-group benefit with a token package', () => {
    const parsed = schema.safeParse(
      benefitValues([
        benefitRow({
          token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE,
          token_total_input: '10万',
        }),
      ])
    )
    assert.equal(parsed.success, true)
  })

  test('accepts an unlimited benefit with an empty token input', () => {
    assert.equal(schema.safeParse(benefitValues([benefitRow()])).success, true)
  })

  test('requires at least one benefit', () => {
    const parsed = schema.safeParse(benefitValues([]))
    assert.equal(parsed.success, false)
    if (!parsed.success) {
      assert.equal(
        parsed.error.issues.some(
          (issue) => issue.message === 'Add at least one benefit'
        ),
        true
      )
    }
  })

  test('requires a group on every benefit', () => {
    assert.equal(
      schema.safeParse(benefitValues([benefitRow({ group_name: '  ' })]))
        .success,
      false
    )
  })

  test('requires a model name for a single-model benefit', () => {
    assert.equal(
      schema.safeParse(
        benefitValues([benefitRow({ scope: ACTIVITY_BENEFIT_SCOPE.MODEL })])
      ).success,
      false
    )
    assert.equal(
      schema.safeParse(
        benefitValues([
          benefitRow({
            scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
            model_name: 'gpt-5',
          }),
        ])
      ).success,
      true
    )
  })

  test('rejects the same group, scope and model combination twice', () => {
    assert.equal(
      schema.safeParse(benefitValues([benefitRow(), benefitRow()])).success,
      false
    )
    assert.equal(
      schema.safeParse(
        benefitValues([
          benefitRow({
            scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
            model_name: 'gpt-5',
          }),
          benefitRow({
            scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
            model_name: 'o3',
          }),
        ])
      ).success,
      true
    )
  })

  test('rejects RPM limits outside 0-100000 or fractional limits', () => {
    for (const rpmLimit of [-1, 100_001, 1.5]) {
      assert.equal(
        schema.safeParse(benefitValues([benefitRow({ rpm_limit: rpmLimit })]))
          .success,
        false,
        `${rpmLimit} must be rejected`
      )
    }
    assert.equal(
      schema.safeParse(benefitValues([benefitRow({ rpm_limit: 100_000 })]))
        .success,
      true
    )
  })

  test('rejects an invalid token package input', () => {
    for (const input of ['', 'abc', '0', '1000000000000001']) {
      assert.equal(
        schema.safeParse(
          benefitValues([
            benefitRow({
              token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE,
              token_total_input: input,
            }),
          ])
        ).success,
        false,
        `${JSON.stringify(input)} must be rejected`
      )
    }
    assert.equal(
      schema.safeParse(
        benefitValues([
          benefitRow({
            token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE,
            token_total_input: '1.5M',
          }),
        ])
      ).success,
      true
    )
  })

  test('ignores benefit rows for the ratio type', () => {
    const parsed = schema.safeParse(
      validValues({ benefits: [benefitRow({ group_name: '' })] })
    )
    assert.equal(parsed.success, true)
  })

  test('rejects more than 20 benefits', () => {
    const rows = Array.from({ length: 21 }, () => benefitRow())
    assert.equal(schema.safeParse(benefitValues(rows)).success, false)
  })
})

describe('activity benefit form payload', () => {
  test('keeps existing benefit keys and sends an empty key for new rows', () => {
    const payload = transformFormValuesToPayload(
      benefitValues([
        benefitRow({
          key: 'benefit-1',
          group_name: 'vip',
          rpm_limit: 60,
          token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE,
          token_total_input: '10万',
        }),
        benefitRow({
          group_name: 'default',
          scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
          model_name: ' gpt-5 ',
        }),
      ])
    )
    assert.equal(payload.type, ACTIVITY_CODE_TYPE.BENEFIT)
    assert.deepEqual(payload.benefits, [
      {
        key: 'benefit-1',
        group_name: 'vip',
        scope: ACTIVITY_BENEFIT_SCOPE.GROUP,
        model_name: '',
        rpm_limit: 60,
        token_total: 100_000,
      },
      {
        key: '',
        group_name: 'default',
        scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
        model_name: 'gpt-5',
        rpm_limit: 0,
        token_total: 0,
      },
    ])
  })

  test('derives the top-level group from the first benefit and drops ratio fields', () => {
    const payload = transformFormValuesToPayload(
      benefitValues([benefitRow({ group_name: 'default' })], {
        group_name: 'vip',
        rpm_limit: 30,
        ratio_mode: ACTIVITY_RATIO_MODE.OVERRIDE,
        ratio_override: 0.5,
      })
    )
    assert.equal(payload.group_name, 'default')
    assert.equal(payload.rpm_limit, 0)
    assert.equal(payload.ratio_mode, ACTIVITY_RATIO_MODE.INHERIT)
    assert.equal(payload.ratio_override, 0)
    assert.deepEqual(payload.model_ratio_overrides, {})
  })

  test('sends an empty benefit list for the ratio type', () => {
    const payload = transformFormValuesToPayload(
      validValues({ benefits: [benefitRow()] })
    )
    assert.equal(payload.type, ACTIVITY_CODE_TYPE.RATIO)
    assert.deepEqual(payload.benefits, [])
  })
})

describe('activity code benefits to form values', () => {
  test('backfills keys and raw integer token totals', () => {
    const values = transformActivityCodeToFormValues(benefitActivityCode())
    assert.equal(values.type, ACTIVITY_CODE_TYPE.BENEFIT)
    assert.deepEqual(values.benefits, [
      {
        key: 'benefit-1',
        group_name: 'vip',
        scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
        model_name: 'gpt-5',
        rpm_limit: 60,
        token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.PACKAGE,
        token_total_input: '1000000',
      },
    ])
  })

  test('treats a zero token total as unlimited with an empty input', () => {
    const values = transformActivityCodeToFormValues(
      benefitActivityCode({
        benefits: [
          {
            key: 'benefit-2',
            group_name: 'default',
            scope: 'group',
            model_name: '',
            rpm_limit: 0,
            token_total: 0,
          },
        ],
      })
    )
    assert.deepEqual(values.benefits, [
      {
        key: 'benefit-2',
        group_name: 'default',
        scope: ACTIVITY_BENEFIT_SCOPE.GROUP,
        model_name: '',
        rpm_limit: 0,
        token_mode: ACTIVITY_BENEFIT_TOKEN_MODE.UNLIMITED,
        token_total_input: '',
      },
    ])
  })

  test('falls back to the ratio type and no benefits for old data', () => {
    const values = transformActivityCodeToFormValues(
      activityCode({ type: undefined, benefits: null })
    )
    assert.equal(values.type, ACTIVITY_CODE_TYPE.RATIO)
    assert.deepEqual(values.benefits, [])
  })
})
