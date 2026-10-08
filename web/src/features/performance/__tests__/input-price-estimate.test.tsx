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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import type { PerfGroupPricing } from '@/features/performance-metrics/lib/pricing'

import { InputPriceEstimate } from '../components/input-price-estimate'

const PRICING: PerfGroupPricing = {
  nominalRatio: 0.17,
  referenceCacheRate: 0.9,
  observedCacheRate: 0.85,
  effectiveRatio: 1.234,
  estimatedInputPricePerMillion: 0.041,
  referenceInputPricePerMillion: 0.0333,
  baseInputPricePerMillion: 0.25,
  baseCacheReadPricePerMillion: 0.025,
  observedInputMultiplier: 0.164,
  pricedInputShare: 0.95,
  coverage: 'reliable',
  models: [],
}

let language = 'en'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: {
      get resolvedLanguage() {
        return language
      },
      language,
    },
  }),
}))

afterEach(() => {
  cleanup()
})

describe('InputPriceEstimate i18n locale handling', () => {
  test('formats currency for the zhCN interface code instead of throwing', () => {
    language = 'zhCN'
    render(<InputPriceEstimate pricing={PRICING} />)
    expect(screen.getByText('Input price estimate')).not.toBeNull()
    expect(document.body.textContent ?? '').not.toContain('NaN')
  })

  test('falls back to the runtime locale for an unknown tag', () => {
    language = 'not-a-tag'
    render(<InputPriceEstimate pricing={PRICING} />)
    expect(screen.getByText('Input price estimate')).not.toBeNull()
  })
})
