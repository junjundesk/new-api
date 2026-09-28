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

import type { User } from '../../types'

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
const { useUsersColumns } = await import('../users-columns')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Invited: 'Invited',
        'Number of users invited': 'Number of users invited',
        Revenue: 'Revenue',
        'Total invitation revenue': 'Total invitation revenue',
        'Inviter ID': 'Inviter ID',
        'Inviter username': 'Inviter username',
        'Invited by user ID': 'Invited by user ID',
        'No Inviter': 'No Inviter',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function buildUser(overrides: Partial<User>): User {
  return {
    id: 11,
    username: 'invitee-user',
    display_name: 'Invitee User',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
    ...overrides,
  }
}

function InviteInfoProbe(props: { user: User }) {
  const columns = useUsersColumns()
  const inviteInfoColumn = columns.find((column) => column.id === 'invite_info')
  if (!inviteInfoColumn?.cell) return null

  const renderCell = inviteInfoColumn.cell as (context: {
    row: { original: User }
  }) => ReactNode
  return <>{renderCell({ row: { original: props.user } })}</>
}

async function renderInviteInfoCell(user: User): Promise<string> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <TooltipProvider>
          <InviteInfoProbe user={user} />
        </TooltipProvider>
      </I18nextProvider>
    )
  )

  const text = container.textContent ?? ''
  await act(async () => root.unmount())
  container.remove()
  return text
}

describe('user invite info cell', () => {
  after(() => {
    domWindow.close()
  })

  test('shows the inviter ID together with the inviter username when the inviter is known', async () => {
    const text = await renderInviteInfoCell(
      buildUser({ inviter_id: 4, inviter_username: 'inviter-four' })
    )

    assert.ok(text.includes('Inviter ID: 4'))
    assert.ok(text.includes('Inviter username: inviter-four'))
    assert.equal(text.includes('No Inviter'), false)
  })

  test('falls back to the inviter ID when the inviter username is unavailable', async () => {
    const text = await renderInviteInfoCell(buildUser({ inviter_id: 4 }))

    assert.ok(text.includes('Inviter ID: 4'))
    assert.equal(text.includes('Inviter username:'), false)
    assert.equal(text.includes('No Inviter'), false)
  })

  test('keeps the no-inviter placeholder for users without an inviter', async () => {
    const text = await renderInviteInfoCell(buildUser({ inviter_id: 0 }))

    assert.ok(text.includes('No Inviter'))
    assert.equal(text.includes('Inviter ID:'), false)
  })
})
