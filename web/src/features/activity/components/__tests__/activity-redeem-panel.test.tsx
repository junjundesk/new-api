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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { toast, Toaster } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ActivityBenefit, ActivityPreview } from '../../types'
import { ActivityRedeemPanel } from '../activity-redeem-panel'

vi.mock('../../api', () => ({
  getActivityOffers: vi.fn(),
  verifyActivityCode: vi.fn(),
  redeemActivityCode: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  getSelf: vi.fn(),
  handleServerError: vi.fn(),
}))

interface AsyncMock {
  mockReset: () => void
  mockResolvedValue: (value: unknown) => void
  mock: { calls: unknown[][] }
}

const activityApi = await import('../../api')
const libApi = await import('@/lib/api')
const getOffersMock = activityApi.getActivityOffers as unknown as AsyncMock
const verifyMock = activityApi.verifyActivityCode as unknown as AsyncMock
const redeemMock = activityApi.redeemActivityCode as unknown as AsyncMock
const getSelfMock = libApi.getSelf as unknown as AsyncMock

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Enter your activity code': 'Enter your activity code',
        Verify: 'Verify',
        Redeem: 'Redeem',
        'Confirm Redemption': 'Confirm Redemption',
        'Confirm and Redeem': 'Confirm and Redeem',
        Cancel: 'Cancel',
        'Wallet balance': 'Wallet balance',
        'Bonus credit cannot be used for redemption.':
          'Bonus credit cannot be used for redemption.',
        'Spend {{quota}} of your wallet balance to redeem {{group}}?':
          'Spend {{quota}} balance to redeem {{group}}?',
        Group: 'Group',
        Validity: 'Validity',
        Ratio: 'Ratio',
        'RPM limit': 'RPM limit',
        Unlimited: 'Unlimited',
        'Group default ratio': 'Group default ratio',
        'Fully free': 'Fully free',
        Permanent: 'Permanent',
        '{{count}} hours': '{{count}} hours',
        '{{count}} days': '{{count}} days',
        '{{count}} minutes': '{{count}} minutes',
        'Invalid activity code': 'Invalid activity code',
        'Free activity codes are redeemed right away; paid ones ask for confirmation first.':
          'Free activity codes are redeemed right away; paid ones ask for confirmation first.',
        'Redeemed {{group}}. Switch your token group to start using it.':
          'Redeemed {{group}}. Switch your token group to start using it.',
        'This activity code is not redeemable now':
          'This activity code is not redeemable now',
        Benefits: 'Benefits',
        'Whole group': 'Whole group',
        'Model {{model}}': 'Model {{model}}',
        '{{amount}} tokens': '{{amount}} tokens',
        'Unlimited tokens': 'Unlimited tokens',
        'Models (ratio)': 'Models (ratio)',
        'All models in the group': 'All models in the group',
      },
    },
  },
  interpolation: { escapeValue: false },
})

function previewFixture(
  overrides: Partial<ActivityPreview> = {}
): ActivityPreview {
  return {
    code_id: 1,
    code: 'hdmfree',
    name: 'Activity',
    description: '',
    group_name: 'hdfree',
    group_desc: 'Free Group',
    duration_seconds: 24 * 3600,
    rpm_limit: 5,
    ratio_mode: 'override',
    ratio_override: 0,
    price_quota: 0,
    balance_quota: 100000,
    bonus_quota: 0,
    can_afford: true,
    need_confirm: false,
    already_granted: false,
    current_expire_time: 0,
    remaining_total: -1,
    redeemable: true,
    blocked_reason: '',
    message: '',
    ...overrides,
  }
}

function benefitFixture(
  overrides: Partial<ActivityBenefit> = {}
): ActivityBenefit {
  return {
    key: 'benefit-1',
    group_name: 'hdfree',
    group_desc: 'Free Group',
    scope: 'group',
    model_name: '',
    rpm_limit: 0,
    token_total: 10_000_000,
    ...overrides,
  }
}

function mockActivityApi(
  verifyResult: ActivityPreview | { failure: string },
  redeemResult: { grantId: number; groupNames?: string[] } | { failure: string }
): void {
  if ('failure' in verifyResult) {
    verifyMock.mockResolvedValue({
      success: false,
      message: verifyResult.failure,
    })
  } else {
    verifyMock.mockResolvedValue({ success: true, data: verifyResult })
  }
  if ('failure' in redeemResult) {
    redeemMock.mockResolvedValue({
      success: false,
      message: redeemResult.failure,
    })
    return
  }
  redeemMock.mockResolvedValue({
    success: true,
    data: {
      grant_id: redeemResult.grantId,
      code_id: 1,
      code: 'hdmfree',
      code_name: 'Activity',
      group_name: 'hdfree',
      group_names: redeemResult.groupNames,
      expire_time: 0,
      price_quota: 0,
      balance_after: 0,
    },
  })
}

function renderPanel(): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ActivityRedeemPanel />
      </I18nextProvider>
      <Toaster duration={60_000} />
    </QueryClientProvider>
  )
}

async function submitCode(
  user: ReturnType<typeof userEvent.setup>,
  code: string
): Promise<void> {
  await user.type(screen.getByPlaceholderText('Enter your activity code'), code)
  await user.click(screen.getByRole('button', { name: 'Verify' }))
}

beforeEach(() => {
  vi.clearAllMocks()
  getOffersMock.mockResolvedValue({
    success: true,
    data: { enabled: true, offers: [] },
  })
  getSelfMock.mockResolvedValue({ success: true })
})

afterEach(() => {
  cleanup()
  toast.dismiss()
})

describe('activity redeem panel', () => {
  it('redeems free activity codes without a confirmation dialog', async () => {
    mockActivityApi(previewFixture({ price_quota: 0, need_confirm: false }), {
      grantId: 11,
    })

    const user = userEvent.setup()
    renderPanel()
    await submitCode(user, 'hdmfree')

    await waitFor(() =>
      expect(redeemMock).toHaveBeenCalledWith({
        code: 'hdmfree',
        confirm: false,
        expectedPriceQuota: 0,
      })
    )
    expect(screen.queryByText('Confirm Redemption')).not.toBeInTheDocument()
  })

  it('paid activity codes ask for confirmation before charging the wallet', async () => {
    mockActivityApi(previewFixture({ price_quota: 500, need_confirm: true }), {
      grantId: 12,
    })

    const user = userEvent.setup()
    renderPanel()
    await submitCode(user, 'hdmpaid')

    const confirmButton = await screen.findByRole('button', {
      name: 'Confirm and Redeem',
    })
    expect(redeemMock).not.toHaveBeenCalled()
    // 回归：ratio 类型的确认弹窗仍要展示倍率相关行。
    expect(screen.getByText('Ratio')).toBeVisible()

    await user.click(confirmButton)
    await waitFor(() =>
      expect(redeemMock).toHaveBeenCalledWith({
        code: 'hdmfree',
        confirm: true,
        expectedPriceQuota: 500,
      })
    )
  })

  it('a paid benefit code lists every benefit and hides the ratio rows', async () => {
    mockActivityApi(
      previewFixture({
        price_quota: 500,
        need_confirm: true,
        type: 'benefit',
        benefits: [
          benefitFixture(),
          benefitFixture({
            key: 'benefit-2',
            group_name: 'hdpro',
            group_desc: 'Pro Group',
            scope: 'model',
            model_name: 'gpt-5',
            token_total: 0,
          }),
        ],
      }),
      { grantId: 14 }
    )

    const user = userEvent.setup()
    renderPanel()
    await submitCode(user, 'hdfree')

    expect(
      await screen.findByText('Free Group · Whole group · 10M tokens')
    ).toBeVisible()
    expect(
      screen.getByText('Pro Group · Model gpt-5 · Unlimited tokens')
    ).toBeVisible()
    expect(screen.getByText('Benefits')).toBeVisible()
    expect(screen.queryByText('Ratio')).not.toBeInTheDocument()
    expect(screen.queryByText('RPM limit')).not.toBeInTheDocument()
  })

  it('insufficient balance disables the confirmation button', async () => {
    mockActivityApi(
      previewFixture({
        price_quota: 900000,
        balance_quota: 1000,
        can_afford: false,
        need_confirm: true,
      }),
      { grantId: 13 }
    )

    const user = userEvent.setup()
    renderPanel()
    await submitCode(user, 'hdmpaid')

    expect(
      await screen.findByText('Bonus credit cannot be used for redemption.')
    ).toBeVisible()
    expect(
      await screen.findByRole('button', { name: 'Confirm and Redeem' })
    ).toBeDisabled()
    expect(redeemMock).not.toHaveBeenCalled()
  })

  it('the success toast joins every redeemed group from group_names', async () => {
    mockActivityApi(previewFixture({ price_quota: 0, need_confirm: false }), {
      grantId: 21,
      groupNames: ['hdfree', 'hdpro'],
    })

    const user = userEvent.setup()
    renderPanel()
    await submitCode(user, 'hdmfree')

    await waitFor(() =>
      expect(document.body).toHaveTextContent('Redeemed hdfree, hdpro.')
    )
  })

  it('the success toast falls back to group_name for old redeem responses', async () => {
    mockActivityApi(previewFixture({ price_quota: 0, need_confirm: false }), {
      grantId: 22,
    })

    const user = userEvent.setup()
    renderPanel()
    await submitCode(user, 'hdmfree')

    await waitFor(() =>
      expect(document.body).toHaveTextContent('Redeemed hdfree.')
    )
  })
})
