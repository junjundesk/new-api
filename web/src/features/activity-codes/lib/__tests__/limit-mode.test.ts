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

import {
  ACTIVITY_BENEFIT_SCOPE,
  ACTIVITY_CODE_TYPE,
  ACTIVITY_LIMIT_MODE,
  ACTIVITY_RATIO_MODE,
  ACTIVITY_RATIO_SCOPE,
} from '../../constants'
import {
  activityBenefitSchema,
  activityCodeSchema,
  activityGrantSchema,
  type ActivityBenefit,
  type ActivityCode,
} from '../../types'
import {
  createActivityBenefitRow,
  transformBenefitRowsToPayload,
  transformBenefitsToRows,
} from '../activity-benefit'
import {
  ACTIVITY_CODE_FORM_DEFAULT_VALUES,
  type ActivityCodeFormValues,
  getActivityCodeFormSchema,
  transformActivityCodeToFormValues,
  transformFormValuesToPayload,
} from '../activity-form'

const t = ((key: string) => key) as unknown as TFunction
const schema = getActivityCodeFormSchema(t)

function ratioValues(
  overrides: Partial<ActivityCodeFormValues> = {}
): ActivityCodeFormValues {
  return {
    ...ACTIVITY_CODE_FORM_DEFAULT_VALUES,
    code: 'limit-mode',
    name: 'Limit mode activity',
    group_name: 'vip',
    type: ACTIVITY_CODE_TYPE.RATIO,
    ...overrides,
  }
}

// 模拟旧接口返回的活动码：没有 limit_mode / concurrency_limit 字段。
const legacyCodeRaw = {
  id: 1,
  code: 'legacy',
  name: 'Legacy',
  description: '',
  status: 1,
  group_name: 'vip',
  duration_seconds: 86_400,
  rpm_limit: 60,
  price_quota: 0,
  ratio_mode: ACTIVITY_RATIO_MODE.INHERIT,
  ratio_override: 0,
  ratio_scope: ACTIVITY_RATIO_SCOPE.GROUP,
  model_ratio_overrides: null,
  start_time: 0,
  end_time: 0,
  total_limit: 0,
  used_count: 0,
  per_user_limit: 1,
  public_visible: 0,
  type: ACTIVITY_CODE_TYPE.RATIO,
  benefits: null,
  created_by: 1,
  created_time: 0,
  updated_time: 0,
}

function legacyActivityCode(overrides: Record<string, unknown>): ActivityCode {
  return { ...legacyCodeRaw, ...overrides } as unknown as ActivityCode
}

const legacyBenefitRaw = {
  key: 'k',
  group_name: 'vip',
  scope: ACTIVITY_BENEFIT_SCOPE.GROUP,
  model_name: '',
  rpm_limit: 60,
  token_total: 0,
}

const legacyGrantRaw = {
  id: 1,
  user_id: 3,
  code_id: 1,
  code: 'legacy',
  code_name: 'Legacy',
  group_name: 'vip',
  ratio_mode: ACTIVITY_RATIO_MODE.INHERIT,
  ratio_override: 0,
  ratio_scope: ACTIVITY_RATIO_SCOPE.GROUP,
  model_ratio_overrides: null,
  rpm_limit: 5,
  price_quota: 0,
  start_time: 0,
  expire_time: 0,
  status: 'active',
  remark: '',
  created_time: 0,
}

describe('limit mode data contract defaults', () => {
  it('reads a missing or empty limit mode as rpm and a missing concurrency limit as 0', () => {
    const code = activityCodeSchema.parse({ ...legacyCodeRaw })
    expect(code.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)
    expect(code.concurrency_limit).toBe(0)

    const emptyMode = activityCodeSchema.parse({
      ...legacyCodeRaw,
      limit_mode: '',
    })
    expect(emptyMode.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)

    const benefit = activityBenefitSchema.parse({ ...legacyBenefitRaw })
    expect(benefit.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)
    expect(benefit.concurrency_limit).toBe(0)

    const grant = activityGrantSchema.parse({ ...legacyGrantRaw })
    expect(grant.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)
    expect(grant.concurrency_limit).toBe(0)
  })

  it('keeps an explicit concurrency configuration from the API', () => {
    const code = activityCodeSchema.parse({
      ...legacyCodeRaw,
      limit_mode: 'concurrency',
      rpm_limit: 0,
      concurrency_limit: 10,
    })
    expect(code.limit_mode).toBe(ACTIVITY_LIMIT_MODE.CONCURRENCY)
    expect(code.concurrency_limit).toBe(10)
  })
})

describe('limit mode form defaults and backfill', () => {
  it('creates a new activity with the rpm limit type and zero limits', () => {
    expect(ACTIVITY_CODE_FORM_DEFAULT_VALUES.limit_mode).toBe(
      ACTIVITY_LIMIT_MODE.RPM
    )
    expect(ACTIVITY_CODE_FORM_DEFAULT_VALUES.rpm_limit).toBe(0)
    expect(ACTIVITY_CODE_FORM_DEFAULT_VALUES.concurrency_limit).toBe(0)
  })

  it('backfills a legacy activity without limit fields to the rpm limit type', () => {
    const values = transformActivityCodeToFormValues(legacyActivityCode({}))

    expect(values.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)
    expect(values.concurrency_limit).toBe(0)
    expect(values.rpm_limit).toBe(60)
  })

  it('backfills a concurrency activity with its own concurrency limit', () => {
    const values = transformActivityCodeToFormValues(
      legacyActivityCode({
        limit_mode: 'concurrency',
        rpm_limit: 0,
        concurrency_limit: 10,
      })
    )

    expect(values.limit_mode).toBe(ACTIVITY_LIMIT_MODE.CONCURRENCY)
    expect(values.concurrency_limit).toBe(10)
    expect(values.rpm_limit).toBe(0)
  })

  it('backfills benefit rows with their own limit type and legacy rows as rpm', () => {
    const rows = transformBenefitsToRows([
      {
        ...legacyBenefitRaw,
        key: 'a',
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        rpm_limit: 0,
        concurrency_limit: 10,
      } as unknown as ActivityBenefit,
      { ...legacyBenefitRaw, key: 'b' } as unknown as ActivityBenefit,
    ])

    expect(rows[0]).toMatchObject({
      limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
      concurrency_limit: 10,
    })
    expect(rows[1]).toMatchObject({
      limit_mode: ACTIVITY_LIMIT_MODE.RPM,
      rpm_limit: 60,
      concurrency_limit: 0,
    })
  })
})

describe('limit mode payload conversion', () => {
  it('submits the rpm limit and zeroes the unused concurrency limit', () => {
    const payload = transformFormValuesToPayload(
      ratioValues({
        limit_mode: ACTIVITY_LIMIT_MODE.RPM,
        rpm_limit: 60,
        concurrency_limit: 10,
      })
    )

    expect(payload.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)
    expect(payload.rpm_limit).toBe(60)
    expect(payload.concurrency_limit).toBe(0)
  })

  it('submits the concurrency limit and zeroes the unused rpm limit', () => {
    const payload = transformFormValuesToPayload(
      ratioValues({
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        rpm_limit: 60,
        concurrency_limit: 10,
      })
    )

    expect(payload.limit_mode).toBe(ACTIVITY_LIMIT_MODE.CONCURRENCY)
    expect(payload.rpm_limit).toBe(0)
    expect(payload.concurrency_limit).toBe(10)
  })

  it('normalizes the top level limits of a benefit code to rpm + 0 + 0', () => {
    const payload = transformFormValuesToPayload(
      ratioValues({
        type: ACTIVITY_CODE_TYPE.BENEFIT,
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        rpm_limit: 60,
        concurrency_limit: 10,
        benefits: [
          createActivityBenefitRow({
            group_name: 'vip',
            limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
            rpm_limit: 60,
            concurrency_limit: 10,
          }),
        ],
      })
    )

    expect(payload.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)
    expect(payload.rpm_limit).toBe(0)
    expect(payload.concurrency_limit).toBe(0)
  })

  it('keeps every benefit row on its own limit type when mixing rpm and concurrency', () => {
    const payload = transformBenefitRowsToPayload([
      createActivityBenefitRow({
        group_name: 'vip',
        limit_mode: ACTIVITY_LIMIT_MODE.RPM,
        rpm_limit: 60,
        concurrency_limit: 10,
      }),
      createActivityBenefitRow({
        group_name: 'default',
        scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
        model_name: 'gpt-5',
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        rpm_limit: 30,
        concurrency_limit: 8,
      }),
    ])

    expect(payload[0]).toMatchObject({
      group_name: 'vip',
      limit_mode: ACTIVITY_LIMIT_MODE.RPM,
      rpm_limit: 60,
      concurrency_limit: 0,
    })
    expect(payload[1]).toMatchObject({
      group_name: 'default',
      model_name: 'gpt-5',
      limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
      rpm_limit: 0,
      concurrency_limit: 8,
    })
  })
})

describe('limit mode value validation', () => {
  it('accepts 0 and 100000 for both limit inputs', () => {
    for (const value of [0, 100_000]) {
      expect(
        schema.safeParse(
          ratioValues({
            limit_mode: ACTIVITY_LIMIT_MODE.RPM,
            rpm_limit: value,
            concurrency_limit: 0,
          })
        ).success
      ).toBe(true)
      expect(
        schema.safeParse(
          ratioValues({
            limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
            rpm_limit: 0,
            concurrency_limit: value,
          })
        ).success
      ).toBe(true)
    }
  })

  it('rejects out-of-range or fractional limit values', () => {
    const cases: Array<{
      values: Partial<ActivityCodeFormValues>
      message: string
    }> = [
      {
        values: {
          limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
          concurrency_limit: -1,
        },
        message: 'Concurrency limit must be 0 or greater',
      },
      {
        values: {
          limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
          concurrency_limit: 100_001,
        },
        message: 'Concurrency limit is too large',
      },
      {
        values: {
          limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
          concurrency_limit: 1.5,
        },
        message: 'Concurrency limit must be a whole number',
      },
      {
        values: { limit_mode: ACTIVITY_LIMIT_MODE.RPM, rpm_limit: 100_001 },
        message: 'RPM limit is too large',
      },
      {
        values: { limit_mode: ACTIVITY_LIMIT_MODE.RPM, rpm_limit: -1 },
        message: 'RPM limit must be 0 or greater',
      },
      {
        values: { limit_mode: ACTIVITY_LIMIT_MODE.RPM, rpm_limit: 1.5 },
        message: 'RPM limit must be a whole number',
      },
    ]

    for (const testCase of cases) {
      const parsed = schema.safeParse(ratioValues(testCase.values))
      expect(parsed.success).toBe(false)
      if (!parsed.success) {
        expect(parsed.error.issues.map((issue) => issue.message)).toContain(
          testCase.message
        )
      }
    }
  })

  it('validates every benefit row against the same limit ranges', () => {
    const parsed = schema.safeParse(
      ratioValues({
        type: ACTIVITY_CODE_TYPE.BENEFIT,
        benefits: [
          createActivityBenefitRow({
            group_name: 'vip',
            limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
            concurrency_limit: 100_001,
          }),
        ],
      })
    )

    expect(parsed.success).toBe(false)
    if (!parsed.success) {
      expect(parsed.error.issues.map((issue) => issue.message)).toContain(
        'Concurrency limit is too large'
      )
    }
  })
})

describe('unused limit input never blocks saving', () => {
  it('accepts an out-of-range concurrency value while the rpm type is selected', () => {
    const parsed = schema.safeParse(
      ratioValues({
        limit_mode: ACTIVITY_LIMIT_MODE.RPM,
        rpm_limit: 60,
        concurrency_limit: 100_001,
      })
    )

    expect(parsed.success).toBe(true)
  })

  it('accepts an out-of-range rpm value while the concurrency type is selected', () => {
    const parsed = schema.safeParse(
      ratioValues({
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        rpm_limit: 100_001,
        concurrency_limit: 3,
      })
    )

    expect(parsed.success).toBe(true)
  })

  it('still rejects an out-of-range value on the selected limit type', () => {
    expect(
      schema.safeParse(
        ratioValues({
          limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
          rpm_limit: 60,
          concurrency_limit: 100_001,
        })
      ).success
    ).toBe(false)
    expect(
      schema.safeParse(
        ratioValues({
          limit_mode: ACTIVITY_LIMIT_MODE.RPM,
          rpm_limit: -1,
          concurrency_limit: 3,
        })
      ).success
    ).toBe(false)
  })

  it('ignores the unused limit value on every benefit row', () => {
    expect(
      schema.safeParse(
        ratioValues({
          type: ACTIVITY_CODE_TYPE.BENEFIT,
          benefits: [
            createActivityBenefitRow({
              group_name: 'vip',
              limit_mode: ACTIVITY_LIMIT_MODE.RPM,
              rpm_limit: 60,
              concurrency_limit: 100_001,
            }),
            createActivityBenefitRow({
              group_name: 'default',
              limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
              rpm_limit: 100_001,
              concurrency_limit: 3,
            }),
          ],
        })
      ).success
    ).toBe(true)
    expect(
      schema.safeParse(
        ratioValues({
          type: ACTIVITY_CODE_TYPE.BENEFIT,
          benefits: [
            createActivityBenefitRow({
              group_name: 'vip',
              limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
              concurrency_limit: 100_001,
            }),
          ],
        })
      ).success
    ).toBe(false)
  })

  it('ignores the zeroed top level limits of a benefit code', () => {
    const parsed = schema.safeParse(
      ratioValues({
        type: ACTIVITY_CODE_TYPE.BENEFIT,
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        rpm_limit: 100_001,
        concurrency_limit: 100_001,
        benefits: [createActivityBenefitRow({ group_name: 'vip' })],
      })
    )

    expect(parsed.success).toBe(true)
    if (parsed.success) {
      const payload = transformFormValuesToPayload(parsed.data)
      expect(payload.rpm_limit).toBe(0)
      expect(payload.concurrency_limit).toBe(0)
      expect(payload.limit_mode).toBe(ACTIVITY_LIMIT_MODE.RPM)
    }
  })
})
