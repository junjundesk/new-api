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
import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, renderHook } from '@testing-library/react'
import { createInstance } from 'i18next'
import type { ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, test } from 'vitest'

import { parseSidebarModulesAdmin } from '@/features/system-settings/maintenance/config'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useSidebarConfig } from '../use-sidebar-config'
import { useSidebarData } from '../use-sidebar-data'

const clients: QueryClient[] = []

async function sidebarUrls(config: string): Promise<string[]> {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'admin',
    role: ROLE.ADMIN,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)
  client.setQueryData(['status'], { SidebarModulesAdmin: config })
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
  const { result } = renderHook(
    () => useSidebarConfig(useSidebarData().navGroups),
    {
      wrapper: (props: { children: ReactNode }) => (
        <QueryClientProvider client={client}>
          <I18nextProvider i18n={i18n}>{props.children}</I18nextProvider>
        </QueryClientProvider>
      ),
    }
  )
  return result.current
    .flatMap((group) => group.items)
    .flatMap((item) => ('url' in item && item.url ? [String(item.url)] : []))
}

afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  useAuthStore.getState().auth.reset()
})

describe('invite sidebar entry', () => {
  // Sites that saved their sidebar settings before the entry existed have no
  // "referral" key; the entry must still appear after an upgrade.
  test('a configuration saved before the entry existed still shows it', async () => {
    const legacy =
      '{"personal":{"enabled":true,"topup":true,"personal":true},"admin":{"enabled":true,"user":true}}'

    const urls = await sidebarUrls(legacy)

    expect(urls).toContain('/referral')
    expect(parseSidebarModulesAdmin(legacy).personal.referral).toBe(true)
  })

  test('the entry sits between Wallet and Activity Center', async () => {
    const urls = await sidebarUrls('')

    expect(urls.indexOf('/referral')).toBe(urls.indexOf('/wallet') + 1)
    expect(urls.indexOf('/activity')).toBe(urls.indexOf('/referral') + 1)
  })

  // Admin invite management is opened from the invite page to save sidebar
  // space, so admins get no second sidebar entry and no separate switch.
  test('admins get no separate invite management sidebar entry', async () => {
    const urls = await sidebarUrls('')

    expect(urls).not.toContain('/referral-management')
    expect(parseSidebarModulesAdmin('').admin.referral).toBeUndefined()
  })

  test('turning off the personal switch hides the entry', async () => {
    const urls = await sidebarUrls(
      '{"personal":{"enabled":true,"referral":false}}'
    )

    expect(urls).not.toContain('/referral')
  })
})
