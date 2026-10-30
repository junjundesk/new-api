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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { useEffect } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ActivityCode, ActivityGrant } from '../../types'
import {
  ActivityCodesProvider,
  useActivityCodes,
} from '../activity-codes-provider'
import { ActivityGrantsDialog } from '../activity-grants-dialog'

vi.mock('../../api', () => ({
  getActivityCodeGrants: vi.fn(),
  revokeActivityGrant: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: Array<unknown[]> }
}

const api = await import('../../api')
const mockedGetGrants = api.getActivityCodeGrants as unknown as AsyncMock

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
})

const benefitCode: ActivityCode = {
  id: 12,
  code: 'summer-benefit',
  name: 'Summer benefit',
  description: '',
  status: 1,
  group_name: 'vip',
  duration_seconds: 24 * 3600,
  rpm_limit: 0,
  price_quota: 500_000,
  ratio_mode: 'inherit',
  ratio_override: 0,
  ratio_scope: 'group',
  model_ratio_overrides: null,
  start_time: 0,
  end_time: 0,
  total_limit: 0,
  used_count: 0,
  per_user_limit: 1,
  public_visible: 0,
  type: 'benefit',
  benefits: [
    {
      key: 'benefit-1',
      group_name: 'vip',
      scope: 'group',
      model_name: '',
      rpm_limit: 0,
      token_total: 1_000_000,
    },
    {
      key: 'benefit-2',
      group_name: 'default',
      scope: 'model',
      model_name: 'gpt-5',
      rpm_limit: 0,
      token_total: 0,
    },
  ],
  created_by: 1,
  created_time: 0,
  updated_time: 0,
}

function grant(overrides: Partial<ActivityGrant>): ActivityGrant {
  return {
    id: 1,
    user_id: 3,
    code_id: benefitCode.id,
    code: benefitCode.code,
    code_name: benefitCode.name,
    group_name: 'vip',
    ratio_mode: 'inherit',
    ratio_override: 0,
    ratio_scope: 'group',
    model_ratio_overrides: null,
    rpm_limit: 0,
    price_quota: 500_000,
    start_time: 0,
    expire_time: 0,
    status: 'active',
    source: 'redeem',
    remark: '',
    created_time: 0,
    redeem_batch: 'batch-a',
    benefit_key: 'benefit-1',
    token_total: 1_000_000,
    token_used: 250_000,
    ...overrides,
  }
}

function DialogHarness() {
  const { setOpen, setCurrentRow } = useActivityCodes()

  useEffect(() => {
    setCurrentRow(benefitCode)
    setOpen('grants')
  }, [setCurrentRow, setOpen])

  return <ActivityGrantsDialog />
}

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ActivityCodesProvider>
          <DialogHarness />
        </ActivityCodesProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  mockedGetGrants.mockReset()
})

afterEach(cleanup)

describe('activity grants dialog benefits', () => {
  it('shows the scope of each grant, one paid amount per redeem batch and token usage', async () => {
    mockedGetGrants.mockResolvedValue({
      success: true,
      data: {
        items: [
          grant({ id: 1, redeem_batch: 'batch-a', benefit_key: 'benefit-1' }),
          grant({
            id: 2,
            redeem_batch: 'batch-a',
            benefit_key: 'benefit-2',
            group_name: 'default',
            token_total: 0,
            token_used: 0,
          }),
          grant({
            id: 3,
            user_id: 4,
            redeem_batch: 'batch-b',
            benefit_key: 'benefit-1',
          }),
        ],
        total: 3,
      },
    })
    renderDialog()

    expect(await screen.findAllByText(/vip · Whole group/)).toHaveLength(2)
    expect(screen.getByText(/default · gpt-5/)).toBeVisible()
    // 三次授权分属两个兑换批次，因此已支付金额只出现两次。
    expect(screen.getAllByText(/Paid:/)).toHaveLength(2)
    expect(screen.getAllByText('Used 250K / 1M tokens')).toHaveLength(2)
    expect(screen.getByText('Used 0 tokens · Unlimited')).toBeVisible()
  })
})
