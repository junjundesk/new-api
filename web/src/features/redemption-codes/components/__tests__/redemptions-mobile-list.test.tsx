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
import { after, test } from 'node:test'

import type { Table } from '@tanstack/react-table'
import { Window } from 'happy-dom'

import type { Redemption } from '../../types'

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
const { RedemptionsMobileList } = await import('../redemptions-mobile-list')
const { RedemptionsProvider } = await import('../redemptions-provider')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'No Redemption Codes Found': 'No Redemption Codes Found',
        'No redemption codes available. Create your first redemption code to get started.':
          'No redemption codes available. Create your first redemption code to get started.',
        'Create Code': 'Create Code',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function emptyTable(): Table<Redemption> {
  return {
    getRowModel: () => ({ rows: [] }),
  } as unknown as Table<Redemption>
}

function rowTable(redemption: Redemption): Table<Redemption> {
  return {
    getRowModel: () => ({
      rows: [{ id: 'redemption-1', original: redemption }],
    }),
  } as unknown as Table<Redemption>
}

function renderList(props?: {
  table?: Table<Redemption>
  emptyTitle?: string
  emptyDescription?: string
  showCreateAction?: boolean
}) {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)

  return {
    host,
    root,
    render: async () => {
      await act(async () =>
        root.render(
          <I18nextProvider i18n={i18n}>
            <RedemptionsProvider>
              <RedemptionsMobileList
                table={props?.table ?? emptyTable()}
                isLoading={false}
                {...props}
              />
            </RedemptionsProvider>
          </I18nextProvider>
        )
      )
    },
  }
}

after(() => {
  domWindow.close()
})

test('mobile empty state offers a direct create action without filters', async () => {
  const rendered = renderList()
  await rendered.render()

  assert.equal(
    rendered.host.textContent?.includes('No Redemption Codes Found'),
    true
  )
  const createButton = [...rendered.host.querySelectorAll('button')].find(
    (button) => button.textContent?.trim() === 'Create Code'
  )
  assert.ok(createButton)

  await act(async () => rendered.root.unmount())
  rendered.host.remove()
})

test('mobile filtered empty state hides create action and uses filter guidance', async () => {
  const rendered = renderList({
    emptyTitle: 'No matching results',
    emptyDescription: 'No records found. Try adjusting your filters.',
    showCreateAction: false,
  })
  await rendered.render()

  assert.equal(rendered.host.textContent?.includes('No matching results'), true)
  assert.equal(
    rendered.host.textContent?.includes(
      'No records found. Try adjusting your filters.'
    ),
    true
  )
  const createButton = [...rendered.host.querySelectorAll('button')].find(
    (button) => button.textContent?.trim() === 'Create Code'
  )
  assert.equal(createButton, undefined)

  await act(async () => rendered.root.unmount())
  rendered.host.remove()
})

test('mobile redemption card shows expiration and redemption details', async () => {
  const rendered = renderList({
    table: rowTable({
      id: 1,
      user_id: 1,
      name: 'Spring promotion',
      key: '12345678901234567890123456789012',
      status: 3,
      quota: 500000,
      created_time: 1,
      redeemed_time: 1_700_000_000,
      expired_time: 1_800_000_000,
      used_user_id: 42,
    }),
  })
  await rendered.render()

  assert.equal(rendered.host.textContent?.includes('Spring promotion'), true)
  assert.equal(rendered.host.textContent?.includes('Quota'), true)
  assert.equal(rendered.host.textContent?.includes('Expires'), true)
  assert.equal(rendered.host.textContent?.includes('Redeemed By'), true)
  assert.equal(rendered.host.textContent?.includes('User 42'), true)
  assert.equal(rendered.host.textContent?.includes('2023-11'), true)

  await act(async () => rendered.root.unmount())
  rendered.host.remove()
})
