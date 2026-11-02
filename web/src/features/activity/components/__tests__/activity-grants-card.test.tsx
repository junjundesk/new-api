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
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { ActivityGrant } from '../../types'
import { ActivityGrantsCard } from '../activity-grants-card'

vi.mock('../../api', () => ({
  getActivityGrants: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: unknown[][] }
}

const activityApi = await import('../../api')
const grantsMock = activityApi.getActivityGrants as unknown as AsyncMock

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'My Activity Grants': 'My Activity Grants',
        'Set a token group to the granted group to use it while the grant is active.':
          'Set a token group to the granted group to use it while the grant is active.',
        'Loading...': 'Loading...',
        'No activity grants yet.': 'No activity grants yet.',
        Active: 'Active',
        Revoked: 'Revoked',
        Expired: 'Expired',
        'Whole group': 'Whole group',
        'Model {{model}}': 'Model {{model}}',
        'Used {{used}} / {{total}}': 'Used {{used}} / {{total}}',
        'Remaining {{amount}}': 'Remaining {{amount}}',
        'Token quota used up (billed at the original price)':
          'Token quota used up (billed at the original price)',
        Unlimited: 'Unlimited',
        'RPM limit': 'RPM limit',
        RPM: 'RPM',
        Concurrency: 'Concurrency',
        'Expires at': 'Expires at',
        Paid: 'Paid',
        'Group default ratio': 'Group default ratio',
        'Fully free': 'Fully free',
        '{{ratio}}x ratio': '{{ratio}}x ratio',
        'Free of charge': 'Free of charge',
        'Models (ratio)': 'Models (ratio)',
      },
    },
  },
  interpolation: { escapeValue: false },
})

// 固定历史时间戳：用「已经过去的时间」当 fixture，而不是 mock 时钟。
const ACTIVITY_ENDED_AT = 1_700_000_000 // 2023-11-14，早于当前时间
const GRANT_VALID_UNTIL = 4_000_000_000 // 2096 年，远晚于活动结束

function grantFixture(overrides: Partial<ActivityGrant> = {}): ActivityGrant {
  const now = Math.floor(Date.now() / 1000)
  return {
    id: 1,
    user_id: 1,
    code_id: 1,
    code: 'hdmfree',
    code_name: 'Free Pack',
    group_name: 'hdfree',
    ratio_mode: 'inherit',
    ratio_override: 0,
    rpm_limit: 0,
    price_quota: 0,
    start_time: now,
    expire_time: now + 24 * 3600,
    status: 'active',
    remark: '',
    created_time: now,
    ...overrides,
  }
}

function renderGrants(grants: ActivityGrant[]): void {
  grantsMock.mockResolvedValue({
    success: true,
    data: { grants, active: [] },
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ActivityGrantsCard />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

afterEach(cleanup)

describe('activity grants card', () => {
  it('shows group, scope and token usage for each benefit grant', async () => {
    renderGrants([
      grantFixture({
        id: 1,
        group_name: 'hdfree',
        benefit_key: 'benefit-1',
        redeem_batch: 'batch-1',
        ratio_scope: 'group',
        rpm_limit: 5,
        token_total: 10_000_000,
        token_used: 2_500_000,
      }),
      grantFixture({
        id: 2,
        group_name: 'hdpro',
        benefit_key: 'benefit-2',
        redeem_batch: 'batch-1',
        ratio_scope: 'models',
        model_ratio_overrides: { 'gpt-5': 0 },
        token_total: 0,
        token_used: 10_000,
      }),
    ])

    expect(await screen.findByText('hdfree')).toBeVisible()
    // 未到期的授权仍然展示 Active（避免把「按时间判过期」误做成一律 Expired）。
    expect(screen.getAllByText('Active')).toHaveLength(2)
    // 权益授权自带 RPM 上限，必须同行展示，否则用户会误以为沿用分组的默认限速。
    expect(
      screen.getByText(
        /Free Pack · Whole group · Used 2\.5M \/ 10M · Remaining 7\.5M · RPM: 5 · Expires at/
      )
    ).toBeVisible()
    expect(screen.getByText('hdpro')).toBeVisible()
    // 未配置 RPM（默认 0）时展示不限速，避免出现 "RPM limit: 0" 的误导文案。
    expect(
      screen.getByText(
        /Free Pack · Model gpt-5 · Used 10K \/ Unlimited · RPM: Unlimited · Expires at/
      )
    ).toBeVisible()
  })

  it('shows the rpm and concurrency limits of mixed grants separately', async () => {
    renderGrants([
      grantFixture({
        id: 1,
        group_name: 'hdmixed',
        benefit_key: 'benefit-1',
        redeem_batch: 'batch-9',
        ratio_scope: 'group',
        limit_mode: 'rpm',
        rpm_limit: 5,
      }),
      grantFixture({
        id: 2,
        group_name: 'hdmixed',
        benefit_key: 'benefit-2',
        redeem_batch: 'batch-9',
        ratio_scope: 'group',
        limit_mode: 'concurrency',
        rpm_limit: 0,
        concurrency_limit: 10,
      }),
    ])

    expect(await screen.findByText(/· RPM: 5 · Expires at/)).toBeVisible()
    expect(screen.getByText(/· Concurrency: 10 · Expires at/)).toBeVisible()
  })

  it('tells the user a used-up token package falls back to the original price', async () => {
    renderGrants([
      grantFixture({
        benefit_key: 'benefit-1',
        ratio_scope: 'group',
        token_total: 1_000_000,
        token_used: 1_200_000,
      }),
    ])

    expect(
      await screen.findByText(
        /Used 1\.2M \/ 1M · Token quota used up \(billed at the original price\)/
      )
    ).toBeVisible()
  })

  it('shows the paid amount only once for the same redemption batch', async () => {
    renderGrants([
      grantFixture({
        id: 1,
        benefit_key: 'benefit-1',
        redeem_batch: 'batch-9',
        ratio_scope: 'group',
        token_total: 0,
        token_used: 0,
        price_quota: 500000,
      }),
      grantFixture({
        id: 2,
        group_name: 'hdpro',
        benefit_key: 'benefit-2',
        redeem_batch: 'batch-9',
        ratio_scope: 'group',
        token_total: 0,
        token_used: 0,
        price_quota: 500000,
      }),
    ])

    expect(await screen.findAllByText(/Paid: \$1/)).toHaveLength(1)
  })

  // 回归：活动先于卡密有效期结束时，后端仍返回 status=active，
  // 但授权实际已经失效，徽标必须是 Expired，到期时间展示生效值。
  it('marks a grant expired when the activity ended before the code expiry', async () => {
    renderGrants([
      grantFixture({
        id: 11,
        group_name: 'hdexpired',
        status: 'active',
        expire_time: GRANT_VALID_UNTIL,
        activity_end_time: ACTIVITY_ENDED_AT,
        effective_expire_time: ACTIVITY_ENDED_AT,
      }),
    ])

    expect(await screen.findByText('hdexpired')).toBeVisible()
    expect(screen.getByText('Expired')).toBeVisible()
    expect(screen.queryByText('Active')).not.toBeInTheDocument()

    const summary = screen.getByText(/Expires at:/)
    expect(summary).toHaveTextContent(
      `Expires at: ${new Date(ACTIVITY_ENDED_AT * 1000).toLocaleString()}`
    )
    // 展示生效到期时间，而不是卡密自身更晚的有效期。
    expect(summary).not.toHaveTextContent(
      new Date(GRANT_VALID_UNTIL * 1000).toLocaleString()
    )
  })

  it('falls back to expire_time when an old API omits effective_expire_time', async () => {
    renderGrants([
      grantFixture({
        id: 12,
        group_name: 'hdlegacyexpired',
        status: 'active',
        expire_time: ACTIVITY_ENDED_AT,
      }),
    ])

    expect(await screen.findByText('hdlegacyexpired')).toBeVisible()
    expect(screen.getByText('Expired')).toBeVisible()
    expect(screen.getByText(/Expires at:/)).toHaveTextContent(
      `Expires at: ${new Date(ACTIVITY_ENDED_AT * 1000).toLocaleString()}`
    )
  })

  it('falls back to the ratio summary when a grant has no benefit fields', async () => {
    renderGrants([grantFixture({ id: 3, group_name: 'hdlegacy' })])

    expect(await screen.findByText(/Group default ratio/)).toBeVisible()
    expect(screen.queryByText(/Whole group/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Used /)).not.toBeInTheDocument()
  })
})
