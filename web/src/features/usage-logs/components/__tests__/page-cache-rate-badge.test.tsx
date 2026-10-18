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
import { afterEach, describe, expect, it, vi } from 'vitest'

import { PageCacheRateBadge } from '../page-cache-rate-badge'

// bun test runs without the app i18n bootstrap; stub useTranslation with an
// identity translator so assertions target the source label.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

afterEach(() => {
  document.body.innerHTML = ''
})

describe('PageCacheRateBadge', () => {
  it('when a rate is available then shows the label and one-decimal value', () => {
    render(<PageCacheRateBadge rate={42.5} />)

    expect(screen.getByText('Page Cache Rate')).not.toBeNull()
    expect(screen.getByText('42.5%')).not.toBeNull()
  })

  it('when the page has no cache tokens then still renders a zero rate', () => {
    render(<PageCacheRateBadge rate={0} />)

    expect(screen.getByText('0.0%')).not.toBeNull()
  })

  it('when no rate can be computed then shows an em dash', () => {
    render(<PageCacheRateBadge rate={null} />)

    expect(screen.getByText('—')).not.toBeNull()
  })
})
