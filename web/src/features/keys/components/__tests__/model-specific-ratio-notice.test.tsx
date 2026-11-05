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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, test } from 'vitest'

import { api } from '@/lib/api'

import type { ApiKey } from '../../types'
import { ApiKeysMutateDrawer } from '../api-keys-mutate-drawer'
import { ApiKeysProvider } from '../api-keys-provider'

const GROUPS_WITH_MODEL_SPECIFIC_RATIO = {
  auto: { desc: 'Automatic routing', ratio: '自动' },
  default: { desc: 'Standard access', ratio: 1 },
  vip: {
    desc: 'Exclusive pricing group',
    ratio: 2,
    has_model_specific_ratio: true,
  },
  'chain:primary': {
    custom_chain: true,
    chain_groups: ['vip'],
    desc: 'Primary chain',
    ratio: '自动',
    has_model_specific_ratio: true,
  },
}

// The drawer only reads the fields listed here; schema defaults fill the rest.
const EXISTING_KEY = {
  id: 7,
  name: 'existing-key',
  key: 'sk-existing',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  unlimited_quota: true,
  expired_time: -1,
  created_time: 1_700_000_000,
  accessed_time: 1_700_000_000,
  group: 'default',
  model_limits_enabled: false,
} as unknown as ApiKey

const NOTICE_TITLE = 'This group has model-specific ratios'
const MODEL_SQUARE_LINK = 'View in Model Square'
const MODEL_SQUARE_SENTENCE = 'The model square shows the exact ratio.'

type RenderOptions = {
  pricingEnabled?: boolean
  currentRow?: ApiKey
}

type MockableApi = {
  get: (url: string, data?: unknown) => Promise<{ data: unknown }>
  post: (url: string, data?: unknown) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post

function installApiFixtures() {
  apiClient.get = async (url) => {
    switch (url) {
      case '/api/status':
        return { data: { data: { default_use_auto_group: false } } }
      case '/api/user/models':
        return { data: { success: true, data: [] } }
      case '/api/user/self/groups':
        return {
          data: { success: true, data: GROUPS_WITH_MODEL_SPECIFIC_RATIO },
        }
      case '/api/token/auto-groups':
        return {
          data: { success: true, data: { groups: ['vip'], max_count: 3 } },
        }
      case `/api/token/${EXISTING_KEY.id}`:
        return { data: { success: true, data: EXISTING_KEY } }
      default:
        throw new Error(`Unexpected GET ${url}`)
    }
  }
  apiClient.post = async (url) => {
    throw new Error(`Unexpected POST ${url}`)
  }
}

async function renderDrawer(options: RenderOptions = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  // The drawer reads module availability from this status payload; without it
  // the parser falls back to its defaults.
  const status: Record<string, unknown> = {
    default_use_auto_group: false,
    HeaderNavModules: JSON.stringify({
      pricing: {
        enabled: options.pricingEnabled !== false,
        requireAuth: false,
      },
    }),
  }
  queryClient.setQueryData(['status'], status, { updatedAt: freshAt })
  queryClient.setQueryData(
    ['user-models'],
    { success: true, data: [] },
    {
      updatedAt: freshAt,
    }
  )
  queryClient.setQueryData(
    ['user-groups'],
    { success: true, data: GROUPS_WITH_MODEL_SPECIFIC_RATIO },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['token-auto-groups'],
    { success: true, data: { groups: ['vip'], max_count: 3 } },
    { updatedAt: freshAt }
  )
  if (options.currentRow) {
    queryClient.setQueryData(
      ['api-key', options.currentRow.id],
      { success: true, data: options.currentRow },
      { updatedAt: freshAt }
    )
  }

  function KeysProbe() {
    return (
      <ApiKeysMutateDrawer
        open
        onOpenChange={() => undefined}
        currentRow={options.currentRow}
      />
    )
  }

  const rootRoute = createRootRoute({ component: Outlet })
  const keysRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: KeysProbe,
  })
  const pricingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/pricing',
    component: () => <p>Model square</p>,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([keysRoute, pricingRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })

  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })

  const view = render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ApiKeysProvider>
          <RouterProvider router={router} />
        </ApiKeysProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
  await act(async () => {
    await router.load()
  })
  await screen.findAllByRole('combobox')
  return { ...view, router, queryClient }
}

/** The group selector is the first combobox: auto order, model mapping and
 * fallback group pickers render below it. */
function getGroupTrigger(): HTMLButtonElement {
  const trigger = screen.getAllByRole('combobox')[0]
  if (!trigger) {
    throw new Error('Group combobox not found')
  }
  return trigger as HTMLButtonElement
}

async function chooseGroupOption(
  trigger: HTMLButtonElement,
  description: string
) {
  const user = userEvent.setup()
  await user.click(trigger)
  const option = [
    ...document.querySelectorAll<HTMLElement>('[data-slot="command-item"]'),
  ].find((candidate) => candidate.textContent?.includes(description))
  if (!option) throw new Error(`Group option not found: ${description}`)
  await user.click(option)
}

async function selectGroup(description: string) {
  await chooseGroupOption(getGroupTrigger(), description)
}

async function findNotice(): Promise<HTMLElement> {
  return screen.findByRole('alertdialog', { name: NOTICE_TITLE })
}

/** The dialog keeps playing its exit animation after it closes. */
async function expectNoticeGone() {
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
}

function findModelSquareLink(notice: HTMLElement): HTMLElement | null {
  return (
    [...notice.querySelectorAll('a')].find((candidate) =>
      candidate.textContent?.includes(MODEL_SQUARE_LINK)
    ) ?? null
  )
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  cleanup()
  localStorage.clear()
})

describe('model-specific ratio notice when creating an API key', () => {
  test.beforeEach(() => {
    installApiFixtures()
  })

  test('warns and keeps the chosen group when switching to a group with model-specific ratios', async () => {
    await renderDrawer()
    const groupTrigger = getGroupTrigger()

    await selectGroup('Exclusive pricing group')

    const notice = await findNotice()
    expect(
      within(notice).getByRole('heading', { name: NOTICE_TITLE })
    ).toBeVisible()
    expect(notice).toHaveTextContent(MODEL_SQUARE_SENTENCE)
    expect(notice).toHaveTextContent('vip')
    expect(findModelSquareLink(notice)).toHaveTextContent(MODEL_SQUARE_LINK)
    expect(within(notice).getByRole('button', { name: 'OK' })).toBeVisible()
    expect(groupTrigger).toHaveTextContent('vip')
  })

  test('stays quiet for a group without model-specific ratios', async () => {
    await renderDrawer()

    await selectGroup('Standard access')

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(getGroupTrigger()).toHaveTextContent('default')
  })

  test('does not warn again when the group already shown in the notice is reselected', async () => {
    await renderDrawer()
    await selectGroup('Exclusive pricing group')
    await findNotice()
    await userEvent.setup().click(screen.getByRole('button', { name: 'OK' }))
    await expectNoticeGone()

    await selectGroup('Exclusive pricing group')

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(getGroupTrigger()).toHaveTextContent('vip')
  })

  test('confirming closes the notice and returns focus to the group selector', async () => {
    await renderDrawer()
    await selectGroup('Exclusive pricing group')
    await findNotice()

    await userEvent.setup().click(screen.getByRole('button', { name: 'OK' }))

    await expectNoticeGone()
    expect(getGroupTrigger()).toHaveTextContent('vip')
    expect(getGroupTrigger()).toHaveFocus()
  })

  test('Escape closes only the notice and keeps the filled-in drawer open', async () => {
    await renderDrawer()
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Name'), 'keep-me')
    await selectGroup('Exclusive pricing group')
    await findNotice()

    await user.keyboard('{Escape}')

    await expectNoticeGone()
    expect(screen.getByLabelText('Name')).toHaveValue('keep-me')
    expect(getGroupTrigger()).toHaveTextContent('vip')
  })

  test('offers a model square link filtered by the chosen group in a new tab', async () => {
    const { router } = await renderDrawer()
    const groupTrigger = getGroupTrigger()
    await selectGroup('Exclusive pricing group')
    const notice = await findNotice()

    const link = findModelSquareLink(notice)
    expect(link).toHaveAttribute(
      'href',
      router.buildLocation({ to: '/pricing', search: { group: 'vip' } }).href
    )
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    // The link opens a new tab, so the drawer and its form stay untouched.
    expect(groupTrigger).toHaveTextContent('vip')
  })

  test('sends custom chain groups to the unfiltered model square', async () => {
    const { router } = await renderDrawer()
    await selectGroup('Primary chain')
    const notice = await findNotice()

    expect(findModelSquareLink(notice)).toHaveAttribute(
      'href',
      router.buildLocation({ to: '/pricing', search: {} }).href
    )
  })

  test('drops the model square copy and entry when that module is disabled', async () => {
    await renderDrawer({ pricingEnabled: false })
    await selectGroup('Exclusive pricing group')
    const notice = await findNotice()

    expect(findModelSquareLink(notice)).toBeNull()
    expect(notice).not.toHaveTextContent(MODEL_SQUARE_SENTENCE)
    expect(notice).toHaveTextContent(
      'Models with a model-specific ratio use their own ratio instead of the group ratio.'
    )
    expect(screen.getByRole('button', { name: 'OK' })).toBeVisible()
  })
})

describe('model-specific ratio notice outside the main group selector', () => {
  test.beforeEach(() => {
    installApiFixtures()
  })

  test('stays quiet when an update switches to a group with model-specific ratios', async () => {
    await renderDrawer({ currentRow: EXISTING_KEY })

    await selectGroup('Exclusive pricing group')

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(getGroupTrigger()).toHaveTextContent('vip')
  })

  test('stays quiet when the auto group order picks a group with model-specific ratios', async () => {
    await renderDrawer()
    await selectGroup('Automatic routing')
    const autoOrderTrigger = screen.getAllByRole('combobox')[1] as
      | HTMLButtonElement
      | undefined
    if (!autoOrderTrigger) {
      throw new Error('Auto group order combobox not found')
    }

    await chooseGroupOption(autoOrderTrigger, 'Exclusive pricing group')

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })
})
