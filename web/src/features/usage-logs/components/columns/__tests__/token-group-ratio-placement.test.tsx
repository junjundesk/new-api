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
import { describe, test } from 'node:test'

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
const { UsageLogsProvider } = await import('../../usage-logs-provider')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const LOG_TYPE_CONSUME = 2

interface TokenCellRender {
  container: HTMLDivElement
  unmount: () => Promise<void>
}

async function renderTokenCell(
  other: Record<string, unknown>
): Promise<TokenCellRender> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  const log = {
    type: LOG_TYPE_CONSUME,
    token_name: 'my-token',
    group: 'vip',
    other: JSON.stringify(other),
  }

  function TokenCellProbe() {
    const columns = useCommonLogsColumns(false)
    const tokenColumn = columns.find(
      (column) => 'accessorKey' in column && column.accessorKey === 'token_name'
    )
    if (!tokenColumn || typeof tokenColumn.cell !== 'function') return null
    return <>{tokenColumn.cell({ row: { original: log } } as never)}</>
  }

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <UsageLogsProvider>
          <TokenCellProbe />
        </UsageLogsProvider>
      </I18nextProvider>
    )
  })

  return {
    container,
    unmount: async () => {
      await act(async () => root.unmount())
      container.remove()
    },
  }
}

function findGroupNameSpans(container: HTMLElement): HTMLSpanElement[] {
  return [...container.querySelectorAll('span')].filter(
    (element) => element.textContent === 'vip'
  )
}

describe('usage log token cell group ratio placement', () => {
  test('renders the group ratio on the token name line instead of the group line', async () => {
    const { container, unmount } = await renderTokenCell({
      group: 'vip',
      group_ratio: 2.5,
    })

    const ratio = container.querySelector('[data-log-group-ratio]')
    assert.ok(ratio, 'expected a group ratio element')
    assert.equal(ratio.textContent, '2.5x')

    const tokenLine = ratio.parentElement
    assert.ok(tokenLine, 'expected the ratio to have a parent element')
    assert.match(tokenLine.textContent ?? '', /my-token/)
    assert.doesNotMatch(tokenLine.textContent ?? '', /vip/)

    const groupNameSpans = findGroupNameSpans(container)
    assert.ok(groupNameSpans.length > 0, 'expected the group name to render')
    for (const span of groupNameSpans) {
      assert.equal(span.querySelector('[data-log-group-ratio]'), null)
    }

    await unmount()
  })

  test('prefers the user exclusive ratio over the group ratio', async () => {
    const { container, unmount } = await renderTokenCell({
      group: 'vip',
      group_ratio: 2.5,
      user_group_ratio: 1.5,
    })

    const ratio = container.querySelector('[data-log-group-ratio]')
    assert.ok(ratio)
    assert.equal(ratio.textContent, '1.5x')

    await unmount()
  })

  test('omits the ratio when no ratio is recorded', async () => {
    const { container, unmount } = await renderTokenCell({ group: 'vip' })

    assert.equal(container.querySelector('[data-log-group-ratio]'), null)
    assert.ok(findGroupNameSpans(container).length > 0)

    await unmount()
  })
})
