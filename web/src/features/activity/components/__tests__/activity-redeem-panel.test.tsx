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

import { Window } from 'happy-dom'

import type { ActivityPreview } from '../../types'

// Use Bun's runner at runtime while reusing the Node test types installed here.
const bunTestModule = 'bun:test'
const { afterAll, afterEach, test } = (await import(bunTestModule)) as {
  afterAll: typeof import('node:test').after
  afterEach: typeof import('node:test').afterEach
  test: typeof import('node:test').test
}

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'HTMLFormElement',
  'HTMLLabelElement',
  'HTMLFieldSetElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'KeyboardEvent',
  'PointerEvent',
  'MouseEvent',
  'FocusEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const i18n = (await import('i18next')).default
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query')
const { Toaster, toast } = await import('sonner')
const { api } = await import('@/lib/api')
const { ActivityRedeemPanel } = await import('../activity-redeem-panel')

await i18n.use(initReactI18next).init({
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
        'Group': 'Group',
        'Validity': 'Validity',
        'Ratio': 'Ratio',
        'RPM limit': 'RPM limit',
        'Unlimited': 'Unlimited',
        'Group default ratio': 'Group default ratio',
        'Fully free': 'Fully free',
        'Permanent': 'Permanent',
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
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
}
type RedeemCall = {
  code: string
  confirm: boolean
  expected_price_quota: number
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post
let renderedHost: HTMLDivElement | null = null
let renderedRoot: ReturnType<typeof createRoot> | null = null
let queryClient: InstanceType<typeof QueryClient> | null = null

function previewFixture(overrides: Partial<ActivityPreview> = {}): ActivityPreview {
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

async function waitForCondition(
  condition: () => boolean,
  failureMessage: string
): Promise<void> {
  if (condition()) return

  await new Promise<void>((resolve, reject) => {
    const observer = new MutationObserver(() => {
      if (!condition()) return
      clearTimeout(timeoutId)
      observer.disconnect()
      resolve()
    })
    const timeoutId = setTimeout(() => {
      observer.disconnect()
      reject(new Error(`${failureMessage}: ${document.body.textContent}`))
    }, 2000)

    observer.observe(document, {
      attributes: true,
      childList: true,
      characterData: true,
      subtree: true,
    })
  })
}

function findButton(label: string): HTMLButtonElement | undefined {
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find(
    (button) => button.textContent?.trim() === label
  )
}

async function renderPanel(): Promise<void> {
  const host = document.createElement('div')
  document.body.append(host)
  renderedHost = host
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  renderedRoot = createRoot(host)
  const client = queryClient
  await act(async () =>
    renderedRoot?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <ActivityRedeemPanel />
        </I18nextProvider>
        <Toaster duration={60_000} />
      </QueryClientProvider>
    )
  )
}

async function typeCode(code: string): Promise<void> {
  const input = document.querySelector<HTMLInputElement>('#activity-code')
  assert.ok(input, 'activity code input must render')
  await act(async () => {
    const valueSetter = Object.getOwnPropertyDescriptor(
      domWindow.HTMLInputElement.prototype,
      'value'
    )?.set
    assert.ok(valueSetter)
    valueSetter.call(input, code)
    input.dispatchEvent(
      new domWindow.Event('input', { bubbles: true }) as unknown as Event
    )
  })
}

async function clickVerify(): Promise<void> {
  const button = findButton('Verify')
  assert.ok(button, 'verify button must render')
  await act(async () => button.click())
}

function redeemCalls(): RedeemCall[] {
  return capturedRedeems
}

let capturedRedeems: RedeemCall[] = []

function mockActivityApi(
  verifyResult: ActivityPreview | { failure: string },
  redeemResult: { grantId: number } | { failure: string }
): void {
  apiClient.get = async (url: string) => {
    if (url.startsWith('/api/user/activity/offers')) {
      return { data: { success: true, data: { enabled: true, offers: [] } } }
    }
    return { data: { success: true, data: {} } }
  }
  apiClient.post = async (url: string, data?: unknown) => {
    if (url === '/api/user/activity/verify') {
      if ('failure' in verifyResult) {
        return { data: { success: false, message: verifyResult.failure } }
      }
      return { data: { success: true, data: verifyResult } }
    }
    if (url === '/api/user/activity/redeem') {
      capturedRedeems.push(data as RedeemCall)
      if ('failure' in redeemResult) {
        return { data: { success: false, message: redeemResult.failure } }
      }
      return {
        data: {
          success: true,
          data: {
            grant_id: redeemResult.grantId,
            code_id: 1,
            code: 'hdmfree',
            code_name: 'Activity',
            group_name: 'hdfree',
            expire_time: 0,
            price_quota: 0,
            balance_after: 0,
          },
        },
      }
    }
    return { data: { success: true } }
  }
}

afterEach(async () => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  capturedRedeems = []
  toast.dismiss()
  if (renderedRoot) {
    const root = renderedRoot
    await act(async () => root.unmount())
  }
  renderedRoot = null
  queryClient?.clear()
  queryClient = null
  renderedHost?.remove()
  renderedHost = null
  document.body.replaceChildren()
})

afterAll(() => {
  domWindow.close()
})

test('free activity codes are redeemed without a confirmation dialog', async () => {
  mockActivityApi(previewFixture({ price_quota: 0, need_confirm: false }), {
    grantId: 11,
  })

  await renderPanel()
  await typeCode('hdmfree')
  await clickVerify()
  await act(async () =>
    waitForCondition(
      () => redeemCalls().length === 1,
      'free code must be redeemed directly'
    )
  )

  assert.deepEqual(redeemCalls()[0], {
    code: 'hdmfree',
    confirm: false,
    expected_price_quota: 0,
  })
  assert.equal(
    document.body.textContent?.includes('Confirm Redemption'),
    false,
    'free codes must not open the confirmation dialog'
  )
})

test('paid activity codes ask for confirmation before charging the wallet', async () => {
  mockActivityApi(
    previewFixture({ price_quota: 500, need_confirm: true }),
    { grantId: 12 }
  )

  await renderPanel()
  await typeCode('hdmpaid')
  await clickVerify()
  await act(async () =>
    waitForCondition(
      () =>
        document.body.textContent?.includes('Confirm Redemption') === true,
      'paid code must open the confirmation dialog'
    )
  )
  assert.equal(redeemCalls().length, 0, 'no charge before confirmation')

  const confirmButton = findButton('Confirm and Redeem')
  assert.ok(confirmButton, 'confirmation button must render')
  await act(async () => confirmButton.click())
  await act(async () =>
    waitForCondition(
      () => redeemCalls().length === 1,
      'confirmed redemption must call the redeem endpoint'
    )
  )

  assert.deepEqual(redeemCalls()[0], {
    code: 'hdmfree',
    confirm: true,
    expected_price_quota: 500,
  })
})

test('insufficient balance disables the confirmation button', async () => {
  mockActivityApi(
    previewFixture({
      price_quota: 900000,
      balance_quota: 1000,
      can_afford: false,
      need_confirm: true,
    }),
    { grantId: 13 }
  )

  await renderPanel()
  await typeCode('hdmpaid')
  await clickVerify()
  await act(async () =>
    waitForCondition(
      () =>
        document.body.textContent?.includes('Bonus credit cannot be used') ===
        true,
      'confirmation dialog must explain that bonus credit cannot be used'
    )
  )

  const confirmButton = findButton('Confirm and Redeem')
  assert.ok(confirmButton, 'confirmation button must render')
  assert.equal(
    confirmButton.disabled,
    true,
    'confirmation must be blocked when the wallet balance is too low'
  )
  assert.equal(redeemCalls().length, 0)
})
