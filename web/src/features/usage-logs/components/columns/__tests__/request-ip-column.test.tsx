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
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'customElements',
  'MutationObserver',
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
const { useCommonLogsColumns } = await import('../common-logs-columns')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: { 'IP Address': 'IP Address' } } },
})

async function hasRequestIpColumn(isAdmin: boolean): Promise<boolean> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  let hasColumn = false

  function ColumnProbe() {
    hasColumn = useCommonLogsColumns(isAdmin).some(
      (column) => column.id === 'request_ip'
    )
    return null
  }

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <ColumnProbe />
      </I18nextProvider>
    )
  })
  await act(async () => root.unmount())
  container.remove()
  return hasColumn
}

describe('request IP log column', () => {
  after(() => domWindow.close())

  test('shows the request IP column only to admins', async () => {
    assert.equal(await hasRequestIpColumn(true), true)
    assert.equal(await hasRequestIpColumn(false), false)
  })
})

