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
import { after, before, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLInputElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
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

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('../../../../lib/api')
const { GroupRatioVisualEditor } = await import('../group-ratio-visual-editor')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function changeInputValue(input: HTMLInputElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(input, value)
  input.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

describe('group pricing visual editor', () => {
  let originalPost: typeof api.post

  before(() => {
    originalPost = api.post
  })

  after(() => {
    api.post = originalPost
    domWindow.close()
  })

  test('commits a renamed group before channel synchronization resolves', async () => {
    const resolveRename: {
      current: ((value: { data: { success: boolean } }) => void) | null
    } = { current: null }
    const renamePending = new Promise<{ data: { success: boolean } }>(
      (resolve) => {
        resolveRename.current = resolve
      }
    )
    api.post = (() => renamePending) as typeof api.post

    const changes: Array<{ field: string; value: string }> = []
    function ControlledEditor() {
      const [groupRatio, setGroupRatio] = useState('{"old":1}')
      const [topupGroupRatio, setTopupGroupRatio] = useState('{}')
      const [userUsableGroups, setUserUsableGroups] = useState('{}')

      return (
        <GroupRatioVisualEditor
          groupRatio={groupRatio}
          topupGroupRatio={topupGroupRatio}
          userUsableGroups={userUsableGroups}
          groupGroupRatio='{}'
          autoGroups='[]'
          maxTokenAutoGroupsField={null}
          groupSpecialUsableGroup='{}'
          onChange={(field, value) => {
            changes.push({ field, value })
            if (field === 'GroupRatio') setGroupRatio(value)
            if (field === 'TopupGroupRatio') setTopupGroupRatio(value)
            if (field === 'UserUsableGroups') setUserUsableGroups(value)
          }}
        />
      )
    }

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient()

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <ControlledEditor />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    const input = container.querySelector<HTMLInputElement>('input:not([type])')
    assert.ok(input)

    await act(async () => {
      input.focus()
      changeInputValue(input, 'renamed')
      input.blur()
    })

    assert.equal(changes[0]?.field, 'GroupRatio')
    assert.match(changes[0]?.value ?? '', /"renamed"\s*:\s*1/)

    resolveRename.current?.({ data: { success: true } })
    await act(async () => {})
    await act(async () => root.unmount())
    container.remove()
    queryClient.clear()
  })
})
