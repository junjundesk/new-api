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
import { createInstance, t } from 'i18next'
import type { ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, test } from 'vitest'

import { parseSidebarModulesAdmin } from '@/features/system-settings/maintenance/config'
import { getOperationsSectionNavItems } from '@/features/system-settings/operations/section-registry'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useSidebarConfig } from '../use-sidebar-config'
import { useSidebarData } from '../use-sidebar-data'

const clients: QueryClient[] = []

async function navigation(config: string, userConfig?: string) {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'admin',
    role: ROLE.ADMIN,
    sidebar_modules: userConfig,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  clients.push(client)
  client.setQueryData(['status'], { SidebarModulesAdmin: config })
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
  return renderHook(() => useSidebarConfig(useSidebarData().navGroups), {
    wrapper: (props: { children: ReactNode }) => (
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>{props.children}</I18nextProvider>
      </QueryClientProvider>
    ),
  })
}

afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  useAuthStore.getState().auth.reset()
})

describe('Email Center sidebar', () => {
  test('legacy configurations keep a single Email Center entry and no duplicate SMTP setting entry', async () => {
    const { result } = await navigation(
      '{"admin":{"enabled":true,"user":true}}'
    )
    const emailItems = result.current
      .flatMap((group) => group.items)
      .filter((item) => 'url' in item && item.url === '/email')
    expect(emailItems).toHaveLength(1)
    expect(emailItems[0]).toMatchObject({
      title: 'Email Center',
      requiredRole: ROLE.ADMIN,
    })
    expect(
      getOperationsSectionNavItems(t).some(
        (item) => item.url === '/system-settings/operations/email'
      )
    ).toBe(false)
    expect(
      parseSidebarModulesAdmin('{"admin":{"enabled":true}}').admin.email
    ).toBe(true)
  })

  test.each([
    [
      'admin email switch',
      '{"admin":{"enabled":true,"email":false}}',
      undefined,
    ],
    [
      'admin section switch',
      '{"admin":{"enabled":false,"email":true}}',
      undefined,
    ],
    [
      'personal narrowing',
      '{"admin":{"enabled":true,"email":true}}',
      '{"admin":{"email":false}}',
    ],
  ])('disabling %s hides Email Center', async (_name, config, userConfig) => {
    const { result } = await navigation(config, userConfig)
    expect(
      result.current
        .flatMap((group) => group.items)
        .some((item) => 'url' in item && item.url === '/email')
    ).toBe(false)
  })
})
