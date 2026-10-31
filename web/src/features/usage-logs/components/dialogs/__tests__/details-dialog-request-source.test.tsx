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
  'customElements',
  'matchMedia',
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
const { DetailsDialog } = await import('../details-dialog')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let host: HTMLDivElement | undefined
let root: ReturnType<typeof createRoot> | undefined

async function renderDetailsDialog(other: Record<string, unknown>) {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)

  await act(async () =>
    root?.render(
      <I18nextProvider i18n={i18n}>
        <DetailsDialog
          log={{
            id: 1,
            user_id: 1,
            created_at: 1,
            type: 2,
            content: 'example request',
            username: 'alice',
            token_name: 'default',
            model_name: 'gpt-4o',
            quota: 1000,
            prompt_tokens: 10,
            completion_tokens: 5,
            use_time: 2,
            is_stream: false,
            channel: 3,
            channel_name: 'primary',
            token_id: 1,
            group: 'default',
            ip: '',
            other: JSON.stringify(other),
            request_id: 'req-1',
            upstream_request_id: '',
          }}
          isAdmin
          open
          onOpenChange={() => undefined}
        />
      </I18nextProvider>
    )
  )
}

function detailLabel(label: string): Element | undefined {
  return [...document.querySelectorAll('span')].find(
    (element) => element.textContent?.trim() === label
  )
}

describe('log details request source', () => {
  afterEach(async () => {
    await act(async () => root?.unmount())
    host?.remove()
    document.body.replaceChildren()
    host = undefined
    root = undefined
  })

  after(() => domWindow.close())

  test('shows the site the request came through, with its referring site', async () => {
    await renderDetailsDialog({
      request_source: { host: 'api.example.com', origin: 'chat.example.com' },
    })

    assert.ok(detailLabel('Request Source'), 'request source row is missing')
    const rowText = detailLabel('Request Source')?.parentElement?.textContent
    assert.match(String(rowText), /api\.example\.com/)
    assert.match(String(rowText), /chat\.example\.com/)
  })

  test('shows only the site when no referring site was sent', async () => {
    await renderDetailsDialog({ request_source: { host: 'api.example.com' } })

    const rowText = detailLabel('Request Source')?.parentElement?.textContent
    assert.match(String(rowText), /api\.example\.com/)
    assert.doesNotMatch(String(rowText), /←/)
  })

  test('hides the row for logs recorded before the field existed', async () => {
    await renderDetailsDialog({})

    assert.equal(detailLabel('Request Source'), undefined)
  })
})
