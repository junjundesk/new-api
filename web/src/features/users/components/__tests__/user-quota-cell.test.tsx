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
const { TooltipProvider } = await import('@/components/ui/tooltip')
const { formatQuota } = await import('@/lib/format')
const { UserQuotaCell } = await import('../user-quota-cell')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'No Quota': 'No Quota',
        Bonus: 'Bonus',
        'Used:': 'Used:',
        'Remaining:': 'Remaining:',
        'Total:': 'Total:',
        'Percentage:': 'Percentage:',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderQuotaCell(props: {
  used: number
  remaining: number
  bonus?: number
}): Promise<string> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <TooltipProvider>
          <UserQuotaCell {...props} />
        </TooltipProvider>
      </I18nextProvider>
    )
  )

  const text = container.textContent ?? ''
  await act(async () => root.unmount())
  container.remove()
  return text
}

describe('user quota cell bonus display', () => {
  after(() => {
    domWindow.close()
  })

  // Regression: the zero-quota branch used to return early, so a user holding
  // only bonus appeared to have no quota at all on the users page.
  test('shows bonus even when the user has no regular quota left', async () => {
    const bonus = 5000
    const text = await renderQuotaCell({ used: 0, remaining: 0, bonus })

    assert.ok(text.includes('No Quota'))
    assert.ok(
      text.includes('Bonus'),
      'bonus must stay visible when regular quota is zero'
    )
    assert.ok(
      text.includes(formatQuota(bonus)),
      'the bonus amount must be rendered, not just the label'
    )
  })

  test('shows bonus alongside a regular balance', async () => {
    const text = await renderQuotaCell({ used: 1000, remaining: 2000, bonus: 500 })

    assert.ok(text.includes('Bonus'))
    assert.equal(text.includes('No Quota'), false)
  })

  test('omits the bonus line when the user holds no bonus', async () => {
    const text = await renderQuotaCell({ used: 0, remaining: 0 })

    assert.ok(text.includes('No Quota'))
    assert.equal(text.includes('Bonus'), false)
  })

  test('omits the bonus line when bonus is zero', async () => {
    const text = await renderQuotaCell({ used: 100, remaining: 200, bonus: 0 })

    assert.equal(text.includes('Bonus'), false)
  })
})
