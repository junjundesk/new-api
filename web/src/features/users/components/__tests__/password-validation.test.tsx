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
  'MouseEvent',
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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { UsersProvider } = await import('../users-provider')
const { UsersMutateDrawer } = await import('../users-mutate-drawer')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const PASSWORD_POLICY_MESSAGE = 'Password must be between 8 and 20 characters'

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = { get: ApiMethod; put: ApiMethod; post: ApiMethod }

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPut = apiClient.put
const originalPost = apiClient.post

let host: HTMLDivElement | undefined
let root: ReturnType<typeof createRoot> | undefined
let queryClient: InstanceType<typeof QueryClient> | undefined

const EXISTING_USER = {
  id: 42,
  username: 'alice',
  display_name: 'Alice Example',
  role: 1,
  status: 1,
  quota: 0,
  used_quota: 0,
  request_count: 0,
  group: 'default',
  remark: '',
}

function installApiFixtures(
  updatedPayloads: Array<Record<string, unknown>>,
  createdPayloads: Array<Record<string, unknown>> = []
) {
  apiClient.get = async (url) => {
    switch (url) {
      case '/api/user/42':
        return { data: { success: true, data: EXISTING_USER } }
      case '/api/group/':
        return { data: { success: true, data: ['default'] } }
      case '/api/authz/catalog':
        return { data: { success: true, data: { resources: [], roles: [] } } }
      default:
        throw new Error(`Unexpected GET ${url}`)
    }
  }
  apiClient.put = async (url, data) => {
    assert.equal(url, '/api/user/')
    updatedPayloads.push(data as Record<string, unknown>)
    return { data: { success: true, data: {} } }
  }
  apiClient.post = async (url, data) => {
    assert.equal(url, '/api/user/')
    createdPayloads.push(data as Record<string, unknown>)
    return { data: { success: true, data: {} } }
  }
}

function getSubmitButton(): HTMLButtonElement | undefined {
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find(
    (button) => button.textContent?.trim() === 'Save changes'
  )
}

function getPasswordInput(): HTMLInputElement | null {
  return document.querySelector<HTMLInputElement>('input[type="password"]')
}

function getUsernameInput(): HTMLInputElement | null {
  return document.querySelector<HTMLInputElement>(
    'input[placeholder="Enter username"]'
  )
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

async function renderUpdateDrawer(): Promise<void> {
  await renderDrawer(EXISTING_USER as never)
}

async function renderDrawer(row?: never): Promise<void> {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient = client

  await act(async () =>
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <UsersProvider>
            <UsersMutateDrawer
              open
              onOpenChange={() => undefined}
              currentRow={row}
            />
          </UsersProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  )
}

describe('users drawer password validation', () => {
  afterEach(async () => {
    apiClient.get = originalGet
    apiClient.put = originalPut
    apiClient.post = originalPost
    await act(async () => root?.unmount())
    queryClient?.clear()
    host?.remove()
    document.body.replaceChildren()
    host = undefined
    root = undefined
    queryClient = undefined
  })

  after(() => domWindow.close())

  test('rejects a password shorter than 8 characters before calling the update API', async () => {
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(updatedPayloads)

    await renderUpdateDrawer()

    await act(async () =>
      waitForCondition(
        () => getPasswordInput() !== null,
        'password field did not render'
      )
    )

    const passwordInput = getPasswordInput()
    assert.ok(passwordInput)
    await changeInput(passwordInput, 'short1')

    const submitButton = getSubmitButton()
    assert.ok(submitButton)
    await act(async () => submitButton.click())

    await act(async () =>
      waitForCondition(
        () =>
          (document.body.textContent ?? '').includes(PASSWORD_POLICY_MESSAGE),
        'password policy message did not render'
      )
    )

    assert.deepEqual(updatedPayloads, [])
  })

  test('clears the password error and submits once the password is corrected', async () => {
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(updatedPayloads)

    await renderUpdateDrawer()

    await act(async () =>
      waitForCondition(
        () => getPasswordInput() !== null,
        'password field did not render'
      )
    )

    const passwordInput = getPasswordInput()
    assert.ok(passwordInput)
    await changeInput(passwordInput, 'short1')

    const submitButton = getSubmitButton()
    assert.ok(submitButton)
    await act(async () => submitButton.click())

    await act(async () =>
      waitForCondition(
        () =>
          (document.body.textContent ?? '').includes(PASSWORD_POLICY_MESSAGE),
        'password policy message did not render'
      )
    )

    await changeInput(passwordInput, 'longenough1')
    await act(async () => submitButton.click())

    await act(async () =>
      waitForCondition(
        () => updatedPayloads.length === 1,
        'update request was not sent after the password was corrected'
      )
    )

    assert.equal(updatedPayloads[0].id, 42)
    assert.equal(updatedPayloads[0].password, 'longenough1')
    assert.equal(
      (document.body.textContent ?? '').includes(PASSWORD_POLICY_MESSAGE),
      false
    )
  })

  test('still requires a password of at least 8 characters when creating a user', async () => {
    const updatedPayloads: Array<Record<string, unknown>> = []
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(updatedPayloads, createdPayloads)

    await renderDrawer()

    await act(async () =>
      waitForCondition(
        () => getUsernameInput() !== null,
        'username field did not render'
      )
    )

    // The password policy is what this case protects, so the required username
    // is filled first to let the form reach the submit handler.
    const usernameInput = getUsernameInput()
    assert.ok(usernameInput)
    await changeInput(usernameInput, 'bob')

    const submitButton = getSubmitButton()
    assert.ok(submitButton)
    await act(async () => submitButton.click())

    await act(async () =>
      waitForCondition(
        () =>
          (document.body.textContent ?? '').includes(PASSWORD_POLICY_MESSAGE),
        'password policy message did not render'
      )
    )

    assert.deepEqual(createdPayloads, [])
  })
})
