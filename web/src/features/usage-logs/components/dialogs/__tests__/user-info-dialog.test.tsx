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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'NodeFilter',
  'KeyboardEvent',
  'PointerEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { formatQuota, parseQuotaFromDollars } = await import('@/lib/format')
const { ROLE } = await import('@/lib/roles')
const { useAuthStore } = await import('@/stores/auth-store')
const { UserInfoDialog } = await import('../user-info-dialog')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiClient = {
  get: (url: string, config?: unknown) => Promise<{ data: unknown }>
  post: (
    url: string,
    body?: unknown,
    config?: unknown
  ) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get
const originalPost = apiClient.post

const INITIAL_QUOTA = 500000
const ADJUSTED_QUOTA = 2500000

let host: HTMLDivElement | undefined
let root: ReturnType<typeof createRoot> | undefined
let quota = INITIAL_QUOTA

function mockUserInfo() {
  apiClient.get = async (url) => {
    assert.equal(url, '/api/user/42')
    return {
      data: {
        success: true,
        data: {
          id: 42,
          username: 'alice',
          display_name: 'Alice Example',
          quota,
          used_quota: 10,
          request_count: 1,
        },
      },
    }
  }
}

async function renderUserInfoDialog(): Promise<void> {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)

  await act(async () =>
    root?.render(
      <I18nextProvider i18n={i18n}>
        <UserInfoDialog userId={42} open onOpenChange={() => undefined} />
      </I18nextProvider>
    )
  )
}

function findButton(label: string): HTMLButtonElement | undefined {
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find(
    (button) => button.textContent?.trim() === label
  )
}

function readInfoValue(label: string): string | null {
  const element = [...document.querySelectorAll('label')].find(
    (candidate) => candidate.textContent === label
  )
  return element?.nextElementSibling?.textContent ?? null
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
    }, 1500)

    observer.observe(document.body, {
      attributes: true,
      childList: true,
      characterData: true,
      subtree: true,
    })
  })
}

async function changeInput(
  input: HTMLInputElement,
  value: string
): Promise<void> {
  await act(async () => {
    const valueSetter = Object.getOwnPropertyDescriptor(
      domWindow.HTMLInputElement.prototype,
      'value'
    )?.set
    assert.ok(valueSetter)
    valueSetter.call(input, value)
    input.dispatchEvent(
      new domWindow.Event('input', { bubbles: true }) as unknown as Event
    )
  })
}

describe('user info dialog', () => {
  afterEach(async () => {
    apiClient.get = originalGet
    apiClient.post = originalPost
    useAuthStore.getState().auth.setUser(null)
    quota = INITIAL_QUOTA
    await act(async () => root?.unmount())
    host?.remove()
    document.body.replaceChildren()
    host = undefined
    root = undefined
  })

  after(() => domWindow.close())

  test('shows the user ID instead of the display name after loading user info', async () => {
    mockUserInfo()

    await renderUserInfoDialog()

    await act(async () =>
      waitForCondition(
        () =>
          [...document.querySelectorAll('label')].some(
            (label) => label.textContent === 'User ID'
          ),
        'user ID field did not render'
      )
    )

    assert.equal(readInfoValue('User ID'), '42')
    assert.equal(
      [...document.querySelectorAll('label')].some(
        (label) => label.textContent === 'Display Name'
      ),
      false
    )
  })

  test('hides the quota entry point from non-admin viewers', async () => {
    mockUserInfo()

    await renderUserInfoDialog()

    await act(async () =>
      waitForCondition(
        () => readInfoValue('User ID') === '42',
        'user ID field did not render'
      )
    )

    assert.equal(findButton('Add Quota'), undefined)
  })

  test('adds quota to the viewer balance and refreshes the dialog for admins', async () => {
    useAuthStore
      .getState()
      .auth.setUser({ id: 1, username: 'admin', role: ROLE.ADMIN })
    mockUserInfo()

    let postedBody: unknown
    apiClient.post = async (url, body) => {
      assert.equal(url, '/api/user/manage')
      postedBody = body
      quota = ADJUSTED_QUOTA
      return { data: { success: true } }
    }

    await renderUserInfoDialog()

    await act(async () =>
      waitForCondition(
        () => readInfoValue('Balance') === formatQuota(INITIAL_QUOTA),
        'balance field did not render'
      )
    )

    const addQuotaButton = findButton('Add Quota')
    assert.ok(addQuotaButton)
    await act(async () => addQuotaButton.click())

    await act(async () =>
      waitForCondition(
        () => findButton('Confirm') !== undefined,
        'quota dialog did not open'
      )
    )

    const amountInput = document.querySelector<HTMLInputElement>(
      'input[type="number"]'
    )
    assert.ok(amountInput)
    await changeInput(amountInput, '5')

    const confirmButton = findButton('Confirm')
    assert.ok(confirmButton)
    await act(async () => confirmButton.click())

    await act(async () =>
      waitForCondition(
        () => readInfoValue('Balance') === formatQuota(ADJUSTED_QUOTA),
        'balance did not refresh after adjusting quota'
      )
    )

    assert.deepEqual(postedBody, {
      id: 42,
      action: 'add_quota',
      mode: 'add',
      value: parseQuotaFromDollars(5),
    })
  })
})
