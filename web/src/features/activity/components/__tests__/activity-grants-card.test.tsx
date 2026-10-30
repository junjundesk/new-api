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
    expect(
      screen.getByText(
        /Free Pack · Whole group · Used 2\.5M \/ 10M · Remaining 7\.5M/
      )
    ).toBeVisible()
    expect(screen.getByText('hdpro')).toBeVisible()
    expect(
      screen.getByText(/Free Pack · Model gpt-5 · Used 10K \/ Unlimited/)
    ).toBeVisible()
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

  it('falls back to the ratio summary when a grant has no benefit fields', async () => {
    renderGrants([grantFixture({ id: 3, group_name: 'hdlegacy' })])

    expect(await screen.findByText(/Group default ratio/)).toBeVisible()
    expect(screen.queryByText(/Whole group/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Used /)).not.toBeInTheDocument()
  })
})
