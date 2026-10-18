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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import { ApiAddressesPopover } from '../api-addresses-popover'

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'API Addresses': 'API Addresses',
        'Copy API URL': 'Copy API URL',
        Copied: 'Copied',
        'Copied!': 'Copied!',
        'Current domain': 'Current domain',
        'Default API address': 'Default API address',
        'Loading...': 'Loading...',
      },
    },
  },
})

let writeText: ReturnType<typeof vi.fn>
const initialUrl = window.location.href
let testClients: QueryClient[] = []

function setTestOrigin(url: string) {
  const happyWindow = window as unknown as Window & {
    happyDOM: { setURL: (nextUrl: string) => void }
  }
  happyWindow.happyDOM.setURL(url)
}

function renderWithClient(client: QueryClient) {
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <TooltipProvider>
          <ApiAddressesPopover />
        </TooltipProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
}

function createTestClient() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  testClients.push(client)
  return client
}

function renderPopover(status: Record<string, unknown>) {
  const client = createTestClient()
  client.setQueryData(['status'], status)
  renderWithClient(client)
  return client
}

async function openAddresses() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'API Addresses' }))
  const dialog = await screen.findByRole('dialog', { name: 'API Addresses' })
  return { user, dialog }
}

beforeEach(() => {
  localStorage.clear()
  setTestOrigin('https://console.example.com/')
  writeText = vi.fn(async () => {})
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText },
  })
})

afterEach(() => {
  cleanup()
  for (const client of testClients) client.clear()
  testClients = []
  localStorage.clear()
  Reflect.deleteProperty(navigator, 'clipboard')
  setTestOrigin(initialUrl)
})

describe('ApiAddressesPopover', () => {
  it('lists configured addresses and copies only the chosen URL', async () => {
    renderPopover({
      server_address: 'https://console.example.com',
      api_info_enabled: true,
      api_info: [
        {
          route: 'Global',
          description: 'Worldwide access',
          url: 'https://api.example.com',
          color: 'blue',
        },
        {
          route: 'Asia',
          description: 'Regional access',
          url: 'https://asia.example.com/gateway/v1/',
          color: 'green',
        },
      ],
    })

    const { user, dialog } = await openAddresses()
    const rows = within(dialog).getAllByRole('listitem')

    expect(rows).toHaveLength(2)
    expect(rows[0].textContent).toContain('Global')
    expect(rows[0].textContent).toContain('Worldwide access')
    expect(rows[0].textContent).toContain('https://api.example.com/v1')
    expect(rows[1].textContent).toContain('Asia')
    expect(rows[1].textContent).toContain('https://asia.example.com/gateway/v1')
    expect(writeText).not.toHaveBeenCalled()

    await user.click(
      within(rows[1]).getByRole('button', {
        name: 'Copy API URL: https://asia.example.com/gateway/v1',
      })
    )

    expect(writeText).toHaveBeenCalledWith(
      'https://asia.example.com/gateway/v1'
    )
    expect(
      await within(rows[1]).findByRole('button', { name: 'Copied' })
    ).toBeTruthy()
  })

  it('falls back to the configured server address with /v1', async () => {
    renderPopover({
      api_info: [],
      server_address: 'https://gateway.example.com/proxy/',
    })

    const { dialog } = await openAddresses()
    const rows = within(dialog).getAllByRole('listitem')

    expect(rows).toHaveLength(1)
    expect(rows[0].textContent).toContain('Default API address')
    expect(rows[0].textContent).toContain(
      'https://gateway.example.com/proxy/v1'
    )
  })

  it('falls back to the current domain with /v1 when no address is configured', async () => {
    renderPopover({ api_info: [] })

    const { dialog } = await openAddresses()
    const rows = within(dialog).getAllByRole('listitem')

    expect(rows).toHaveLength(1)
    expect(rows[0].textContent).toContain('Current domain')
    expect(rows[0].textContent).toContain('https://console.example.com/v1')
  })

  it('ignores configured addresses when the API info panel is disabled', async () => {
    renderPopover({
      api_info_enabled: false,
      api_info: [
        {
          route: 'Hidden',
          description: 'Disabled address',
          url: 'https://hidden.example.com',
          color: 'blue',
        },
      ],
      server_address: 'https://gateway.example.com',
    })

    const { dialog } = await openAddresses()
    const rows = within(dialog).getAllByRole('listitem')

    expect(rows).toHaveLength(1)
    expect(rows[0].textContent).toContain('Default API address')
    expect(rows[0].textContent).toContain('https://gateway.example.com/v1')
    expect(dialog.textContent).not.toContain('https://hidden.example.com')
  })

  it('keeps long addresses readable inside a constrained popover', async () => {
    const url =
      'https://regional-api-gateway.example.com/organization/production/openai-compatible'
    renderPopover({
      api_info: [
        {
          route: 'Regional gateway',
          description: 'Regional production gateway',
          url,
          color: 'blue',
        },
      ],
    })

    const { dialog } = await openAddresses()
    const normalizedUrl = `${url}/v1`
    const code = within(dialog).getByText(normalizedUrl)

    expect(dialog.className).toContain('max-w-[calc(100vw-2rem)]')
    expect(dialog.className).toContain('overflow-y-auto')
    expect(code.className).toContain('break-all')
    expect(
      within(dialog).getByRole('button', {
        name: `Copy API URL: ${normalizedUrl}`,
      })
    ).toBeTruthy()
  })

  it('updates the list when the status configuration changes', async () => {
    const client = renderPopover({
      api_info: [
        {
          route: 'Old gateway',
          description: '',
          url: 'https://old.example.com',
          color: 'blue',
        },
      ],
    })

    const { dialog } = await openAddresses()
    expect(within(dialog).getByText('https://old.example.com/v1')).toBeTruthy()

    act(() => {
      client.setQueryData(['status'], {
        api_info: [
          {
            route: 'New gateway',
            description: '',
            url: 'https://new.example.com/v1',
            color: 'blue',
          },
        ],
      })
    })

    expect(
      await within(dialog).findByText('https://new.example.com/v1')
    ).toBeTruthy()
    expect(within(dialog).queryByText('https://old.example.com/v1')).toBeNull()
  })

  it('opens from the keyboard, copies and returns focus on Escape', async () => {
    renderPopover({
      api_info: [],
      server_address: 'https://gateway.example.com',
    })

    const user = userEvent.setup()
    await user.tab()
    const trigger = screen.getByRole('button', { name: 'API Addresses' })
    expect(document.activeElement).toBe(trigger)

    await user.keyboard('{Enter}')
    const dialog = await screen.findByRole('dialog', { name: 'API Addresses' })
    expect(trigger.getAttribute('aria-expanded')).toBe('true')

    const copy = within(dialog).getByRole('button', {
      name: 'Copy API URL: https://gateway.example.com/v1',
    })
    copy.focus()
    expect(document.activeElement).toBe(copy)

    await user.keyboard('{Enter}')
    expect(writeText).toHaveBeenCalledWith('https://gateway.example.com/v1')

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger)
  })

  it('shows a loading row while the status request is pending', async () => {
    const client = createTestClient()
    // Start a pending status fetch before mounting so the component observes
    // the loading state without performing a real request.
    void client.prefetchQuery({
      queryKey: ['status'],
      queryFn: () => new Promise<never>(() => {}),
    })
    renderWithClient(client)

    const { dialog } = await openAddresses()
    expect(within(dialog).getByRole('status').textContent).toContain(
      'Loading...'
    )
  })
})
