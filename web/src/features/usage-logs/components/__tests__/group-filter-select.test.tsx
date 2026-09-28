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
import type { ReactNode } from 'react'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
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
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { GroupFilterSelect } = await import('../group-filter-select')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'All Groups': 'All Groups',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type HarnessProps = {
  value?: string
  groups?: Record<string, unknown>
}

function Harness(props: HarnessProps): ReactNode {
  return (
    <I18nextProvider i18n={i18n}>
      <GroupFilterSelect
        value={props.value}
        groups={props.groups}
        onChange={() => {}}
      />
    </I18nextProvider>
  )
}

async function renderTriggerText(props: HarnessProps): Promise<string> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => root.render(<Harness {...props} />))

  const text = container.querySelector('button')?.textContent ?? ''
  await act(async () => root.unmount())
  container.remove()
  return text
}

describe('log group filter select', () => {
  after(() => {
    domWindow.close()
  })

  test('shows the all-groups label when no group filter is applied', async () => {
    const text = await renderTriggerText({
      groups: { default: {}, vip: {} },
    })

    assert.equal(text, 'All Groups')
  })

  test('shows the selected group name in the trigger', async () => {
    const text = await renderTriggerText({
      value: 'vip',
      groups: { default: {}, vip: {} },
    })

    assert.equal(text, 'vip')
  })

  test('keeps a deep-linked group visible even when it is not in the usable list', async () => {
    const text = await renderTriggerText({
      value: 'legacy-group',
      groups: { default: {} },
    })

    assert.equal(text, 'legacy-group')
  })
})
