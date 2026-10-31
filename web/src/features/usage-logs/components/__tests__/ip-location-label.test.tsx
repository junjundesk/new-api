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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { IpLocationLabel } = await import('../ip-location-label')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiClient = { get: (url: string, config?: unknown) => Promise<unknown> }
const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get

let host: HTMLDivElement | undefined
let root: ReturnType<typeof createRoot> | undefined

async function renderLabel(ip: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)

  await act(async () =>
    root?.render(
      <QueryClientProvider client={queryClient}>
        <IpLocationLabel ip={ip} />
      </QueryClientProvider>
    )
  )
}

async function waitForRenderedText(): Promise<void> {
  if (host?.textContent) return

  await new Promise<void>((resolve) => {
    const timeoutId = setTimeout(() => {
      observer.disconnect()
      resolve()
    }, 500)
    const observer = new MutationObserver(() => {
      if (!host?.textContent) return
      clearTimeout(timeoutId)
      observer.disconnect()
      resolve()
    })
    observer.observe(document.body, {
      childList: true,
      characterData: true,
      subtree: true,
    })
  })
}

describe('IP location label', () => {
  afterEach(async () => {
    apiClient.get = originalGet
    await act(async () => root?.unmount())
    host?.remove()
    document.body.replaceChildren()
    host = undefined
    root = undefined
  })

  after(() => domWindow.close())

  test('renders the resolved location under the IP', async () => {
    const requested: string[] = []
    apiClient.get = async (url: string) => {
      requested.push(url)
      return {
        data: { success: true, data: { location: '中国 广东 广州 联通' } },
      }
    }

    await renderLabel('112.96.150.7')
    await act(async () => waitForRenderedText())

    assert.deepEqual(requested, ['/api/log/self/ip_location?ip=112.96.150.7'])
    assert.equal(host?.textContent, '中国 广东 广州 联通')
    assert.equal(
      host?.querySelector('span')?.getAttribute('title'),
      '中国 广东 广州 联通'
    )
  })

  test('renders nothing when the backend cannot resolve the IP', async () => {
    apiClient.get = async () => ({
      data: { success: true, data: { location: '' } },
    })

    await renderLabel('10.0.0.1')
    await act(async () => waitForRenderedText())

    assert.equal(host?.textContent, '')
  })

  test('renders nothing when the lookup fails', async () => {
    apiClient.get = async () => ({ data: { success: false } })

    await renderLabel('112.96.150.7')
    await act(async () => waitForRenderedText())

    assert.equal(host?.textContent, '')
  })
})
