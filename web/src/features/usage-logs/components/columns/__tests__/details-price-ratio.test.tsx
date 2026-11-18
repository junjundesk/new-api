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
import { describe, test } from 'vitest'

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

interface DetailsCellRender {
  container: HTMLDivElement
  unmount: () => Promise<void>
}

async function renderDetailsCell(
  other: Record<string, unknown>
): Promise<DetailsCellRender> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  const log = {
    type: LOG_TYPE_CONSUME,
    other: JSON.stringify(other),
  }

  function DetailsCellProbe() {
    const columns = useCommonLogsColumns(false)
    const detailsColumn = columns.find(
      (column) => 'accessorKey' in column && column.accessorKey === 'content'
    )
    if (!detailsColumn || typeof detailsColumn.cell !== 'function') return null
    return <>{detailsColumn.cell({ row: { original: log } } as never)}</>
  }

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <UsageLogsProvider>
          <DetailsCellProbe />
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

describe('usage log details cell price display', () => {
  test('shows the ratio-applied price after the struck-through original', async () => {
    const { container, unmount } = await renderDetailsCell({
      group_ratio: 2,
      model_ratio: 1,
      completion_ratio: 2,
    })

    const original = container.querySelector('[data-slot="original-price"]')
    assert.ok(original, 'expected the original price to be struck through')
    assert.equal(original.textContent, '$2 / $4/M')

    const preview = container.querySelector('button')
    assert.ok(preview)
    assert.equal(preview.textContent, '$2 / $4/MStandard · $4 / $8/M')

    await unmount()
  })

  test('omits the strikethrough when no ratio is recorded', async () => {
    const { container, unmount } = await renderDetailsCell({
      model_ratio: 1,
      completion_ratio: 2,
    })

    assert.equal(container.querySelector('[data-slot="original-price"]'), null)
    assert.equal(
      container.querySelector('button')?.textContent,
      'Standard · $2 / $4/M'
    )

    await unmount()
  })

  test('prefers the user exclusive ratio over the group ratio', async () => {
    const { container, unmount } = await renderDetailsCell({
      group_ratio: 2,
      user_group_ratio: 3,
      model_ratio: 1,
    })

    const original = container.querySelector('[data-slot="original-price"]')
    assert.ok(original)
    assert.equal(original.textContent, '$2/M')
    assert.equal(
      container.querySelector('button')?.textContent,
      '$2/MStandard · $6/M'
    )

    await unmount()
  })
})
