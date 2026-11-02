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
import { createInstance } from 'i18next'
import type { ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'

import {
  ACTIVITY_BENEFIT_SCOPE,
  ACTIVITY_CODE_TYPE,
  ACTIVITY_LIMIT_MODE,
} from '../../constants'
import type { ActivityCode } from '../../types'
import { useActivityCodesColumns } from '../activity-codes-columns'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Rate limit': 'Rate limit',
        RPM: 'RPM',
        Concurrency: 'Concurrency',
        Unlimited: 'Unlimited',
        Group: 'Group',
        Name: 'Name',
        Validity: 'Validity',
        Price: 'Price',
        Ratio: 'Ratio',
        Limits: 'Limits',
        'Redemption Window': 'Redemption Window',
        Status: 'Status',
        'Activity Code': 'Activity Code',
        'Select all': 'Select all',
        'Select row': 'Select row',
      },
    },
  },
  interpolation: { escapeValue: false },
})

// 表头文案在 hook 初始化时读取一次，这里只为拿到请求限制列的单元格渲染。
function RateLimitCell(props: { code: ActivityCode }) {
  const columns = useActivityCodesColumns()
  const column = columns.find((item) => item.id === 'rpm_limit') as {
    cell: (context: unknown) => ReactNode
  }
  return <>{column.cell({ row: { original: props.code } })}</>
}

function activityCode(overrides: Partial<ActivityCode>): ActivityCode {
  return {
    id: 1,
    code: 'limits',
    name: 'Limits',
    description: '',
    status: 1,
    group_name: 'vip',
    duration_seconds: 86_400,
    limit_mode: ACTIVITY_LIMIT_MODE.RPM,
    rpm_limit: 0,
    concurrency_limit: 0,
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

function renderCell(code: ActivityCode) {
  return render(
    <I18nextProvider i18n={i18n}>
      <RateLimitCell code={code} />
    </I18nextProvider>
  )
}

afterEach(cleanup)

describe('activity code rate limit column', () => {
  it('shows the limit type with its value for a ratio code', () => {
    renderCell(
      activityCode({
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        rpm_limit: 0,
        concurrency_limit: 10,
      })
    )

    expect(screen.getByText('Concurrency: 10')).toBeVisible()
  })

  it('falls back to rpm when an old API omits the limit fields', () => {
    renderCell(
      activityCode({
        rpm_limit: 5,
      }) as ActivityCode
    )

    expect(screen.getByText('RPM: 5')).toBeVisible()
    expect(screen.queryByText('RPM: 0')).not.toBeInTheDocument()
  })

  it('lists every benefit limit instead of the zeroed top level limits', () => {
    renderCell(
      activityCode({
        type: ACTIVITY_CODE_TYPE.BENEFIT,
        limit_mode: ACTIVITY_LIMIT_MODE.RPM,
        rpm_limit: 0,
        concurrency_limit: 0,
        benefits: [
          {
            key: 'a',
            group_name: 'vip',
            scope: ACTIVITY_BENEFIT_SCOPE.GROUP,
            model_name: '',
            limit_mode: ACTIVITY_LIMIT_MODE.RPM,
            rpm_limit: 60,
            concurrency_limit: 0,
            token_total: 0,
          },
          {
            key: 'b',
            group_name: 'default',
            scope: ACTIVITY_BENEFIT_SCOPE.MODEL,
            model_name: 'gpt-5',
            limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
            rpm_limit: 0,
            concurrency_limit: 10,
            token_total: 0,
          },
        ],
      })
    )

    expect(screen.getByText('RPM: 60; Concurrency: 10')).toBeVisible()
  })

  it('shows unlimited for a configured limit of 0', () => {
    renderCell(
      activityCode({
        limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
        concurrency_limit: 0,
      })
    )

    expect(screen.getByText('Concurrency: Unlimited')).toBeVisible()
  })
})
