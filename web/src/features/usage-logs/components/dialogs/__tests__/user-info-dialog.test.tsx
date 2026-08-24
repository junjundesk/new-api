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
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get

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

describe('user info dialog', () => {
  let host: HTMLDivElement | undefined
  let root: ReturnType<typeof createRoot> | undefined

  afterEach(async () => {
    apiClient.get = originalGet
    await act(async () => root?.unmount())
    host?.remove()
    document.body.replaceChildren()
    host = undefined
    root = undefined
  })

  after(() => domWindow.close())

  test('shows the user ID instead of the display name after loading user info', async () => {
    apiClient.get = async (url) => {
      assert.equal(url, '/api/user/42')
      return {
        data: {
          success: true,
          data: {
            id: 42,
            username: 'alice',
            display_name: 'Alice Example',
            quota: 100,
            used_quota: 10,
            request_count: 1,
          },
        },
      }
    }

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

    await act(async () =>
      waitForCondition(
        () =>
          [...document.querySelectorAll('label')].some(
            (label) => label.textContent === 'User ID'
          ),
        'user ID field did not render'
      )
    )

    const userIdLabel = [...document.querySelectorAll('label')].find(
      (label) => label.textContent === 'User ID'
    )
    assert.ok(userIdLabel)
    assert.equal(userIdLabel.nextElementSibling?.textContent, '42')
    assert.equal(
      [...document.querySelectorAll('label')].some(
        (label) => label.textContent === 'Display Name'
      ),
      false
    )
  })
})
