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
import { LogClientBadge } from '../log-client-badge'

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
    model_name: 'gpt-4o',
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

const codexOther = JSON.stringify({
  client: {
    name: 'Codex Desktop',
    category: 'coding',
    variant: 'desktop',
    version: '0.158.0',
    source: 'User-Agent',
    confidence: 'identified',
  },
  user_agent: 'Codex Desktop/0.158.0 (Windows)',
})

describe('LogClientBadge', () => {
  it('when the log has no client info then renders nothing', () => {
    const { container } = render(<LogClientBadge log={makeLog()} />)
    expect(container.childElementCount).toBe(0)
  });

  it('when an error log has client info then still renders the chip', () => {
    const log = makeLog({ type: 5, other: codexOther })
    render(<LogClientBadge log={log} />)
    expect(
      screen.getByRole('button', {
        name: 'View client details of Codex Desktop',
      })
    ).not.toBeNull()
  });

  it('when client info exists then shows the client chip with an accessible label', () => {
    render(<LogClientBadge log={makeLog({ other: codexOther })} />)
    const trigger = screen.getByRole('button', {
      name: 'View client details of Codex Desktop',
    })
    expect(trigger.getAttribute('aria-haspopup')).toBe('dialog')
    expect(screen.getByText('Codex Desktop 0.158.0')).not.toBeNull()
  });

  it('when the chip is clicked then the detail popover opens with client fields', async () => {
    const user = userEvent.setup()
    render(<LogClientBadge log={makeLog({ other: codexOther })} />)
    await user.click(
      screen.getByRole('button', {
        name: 'View client details of Codex Desktop',
      })
    )
    const dialog = await screen.findByRole('dialog')
    expect(dialog).not.toBeNull()
    expect(screen.getByText('Client Category')).not.toBeNull()
    expect(screen.getByText('desktop')).not.toBeNull()
    expect(screen.getByText('0.158.0')).not.toBeNull()
    expect(screen.getByText('User-Agent')).not.toBeNull()
    expect(
      screen.getByText('Codex Desktop/0.158.0 (Windows)')
    ).not.toBeNull()
  });

  it('when the trigger is focused then it keeps keyboard operability', () => {
    render(
      <LogClientBadge
        log={makeLog({
          other: JSON.stringify({
            client: {
              name: 'SomeTool',
              category: 'other',
              source: 'User-Agent',
              confidence: 'guessed',
            },
          }),
        })}
      />
    )
    const trigger = screen.getByRole('button', {
      name: 'View client details of SomeTool',
    }) as HTMLButtonElement
    trigger.focus()
    expect(document.activeElement).toBe(trigger)
  });
});
