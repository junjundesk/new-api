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
import { createInstance } from 'i18next'
import { describe, expect, it } from 'vitest'

import { describeActivityModelScope } from '../activity-format'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'All models in the group': 'All models in the group',
      },
    },
  },
  interpolation: { escapeValue: false },
})

describe('describeActivityModelScope', () => {
  it('reports the whole group when no model is listed', () => {
    expect(describeActivityModelScope(i18n.t.bind(i18n), null)).toBe(
      'All models in the group'
    )
    expect(describeActivityModelScope(i18n.t.bind(i18n), undefined)).toBe(
      'All models in the group'
    )
    expect(describeActivityModelScope(i18n.t.bind(i18n), {})).toBe(
      'All models in the group'
    )
  })

  it('lists the concrete model ids with their own ratios', () => {
    expect(describeActivityModelScope(i18n.t.bind(i18n), { 'gpt-5': 0 })).toBe(
      'gpt-5: 0'
    )
    expect(
      describeActivityModelScope(i18n.t.bind(i18n), { 'gpt-5': 0, o3: 0.5 })
    ).toBe('gpt-5: 0, o3: 0.5')
  })
})
