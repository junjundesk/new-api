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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { LogCacheRateBadge } from '../log-cache-rate-badge'

// bun test runs without the app i18n bootstrap; stub useTranslation with an
// identity translator that still interpolates {{name}} placeholders.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      let text = key
      if (opts) {
        for (const [k, v] of Object.entries(opts)) {
          text = text.replaceAll(`{{${k}}}`, String(v))
        }
      }
      return text
    },
    i18n: { language: 'en' },
  }),
}))

afterEach(() => {
  document.body.innerHTML = ''
})

function makeLog(overrides: Partial<UsageLog> = {}): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1700000000,
    type: 2,
    content: '',
    username: 'tester',
    token_name: 'tok',
    model_name: 'claude-opus-5',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 0,
    channel_name: '',
    token_id: 0,
    group: '',
    ip: '',
    other: '',
    request_id: '',
    upstream_request_id: '',
    ...overrides,
  }
}

const claudeCacheLog = makeLog({
  prompt_tokens: 17176,
  completion_tokens: 1943,
  other: JSON.stringify({ claude: true, cache_tokens: 274623 }),
})

function badgeFor(log: UsageLog): HTMLElement {
  render(<LogCacheRateBadge log={log} />)
  return screen.getByRole('button', { name: /^Cache hit rate: / })
}

describe('LogCacheRateBadge', () => {
  it('when a consume log carries cache reads then shows the hit rate', () => {
    const badge = badgeFor(claudeCacheLog)

    expect(badge.getAttribute('aria-label')).toBe('Cache hit rate: 94.1%')
    expect(badge.textContent).toBe('94.1%')
  })

  it('when the rate is above 90 percent then uses the green tone', () => {
    expect(badgeFor(claudeCacheLog).className).toContain('bg-emerald-50/35')
  })

  it('when the rate is between 50 and 90 percent then uses the yellow tone', () => {
    const log = makeLog({
      prompt_tokens: 1000,
      other: JSON.stringify({ cache_tokens: 700 }),
    })

    expect(badgeFor(log).className).toContain('bg-amber-50/35')
  })

  it('when the rate is below 50 percent then uses the red tone', () => {
    const log = makeLog({
      prompt_tokens: 1000,
      other: JSON.stringify({ cache_tokens: 200 }),
    })

    expect(badgeFor(log).className).toContain('bg-rose-50/35')
  })

  it('when no cache read is recorded then renders nothing', () => {
    const { container } = render(
      <LogCacheRateBadge log={makeLog({ prompt_tokens: 1000 })} />
    )

    expect(container.childElementCount).toBe(0)
  })

  it('when the log is not a consume log then renders nothing', () => {
    const { container } = render(
      <LogCacheRateBadge log={{ ...claudeCacheLog, type: 5 }} />
    )

    expect(container.childElementCount).toBe(0)
  })

  it('when the usage is missing then renders nothing', () => {
    const { container } = render(<LogCacheRateBadge log={makeLog()} />)

    expect(container.childElementCount).toBe(0)
  })

  it('when the badge is hovered then the tooltip explains how the rate is built', async () => {
    const user = userEvent.setup()
    const badge = badgeFor(claudeCacheLog)

    await user.hover(badge)

    expect(
      await screen.findByText(
        'Cache reads 274,623 / total input 291,799 tokens'
      )
    ).not.toBeNull()
    expect(
      screen.getByText(
        'Cache rate = total cached input tokens ÷ total input tokens. Cache writes count as input, not cache hits; output tokens are excluded.'
      )
    ).not.toBeNull()
  })
})
