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
import { describe, expect, it } from 'vitest'

import { normalizeApiBaseUrl } from '../api-address'

describe('normalizeApiBaseUrl', () => {
  it('appends /v1 to a bare origin', () => {
    expect(normalizeApiBaseUrl('https://api.example.com')).toBe(
      'https://api.example.com/v1'
    )
  })

  it('trims trailing slashes before appending /v1', () => {
    expect(normalizeApiBaseUrl('https://api.example.com///')).toBe(
      'https://api.example.com/v1'
    )
  })

  it('keeps an address that already ends with /v1', () => {
    expect(normalizeApiBaseUrl('https://api.example.com/v1')).toBe(
      'https://api.example.com/v1'
    )
  })

  it('keeps an address that already contains a /v1beta segment', () => {
    expect(normalizeApiBaseUrl('https://api.example.com/gateway/v1beta/')).toBe(
      'https://api.example.com/gateway/v1beta'
    )
  })

  it('appends /v1 after a path prefix', () => {
    expect(normalizeApiBaseUrl('https://api.example.com/gateway')).toBe(
      'https://api.example.com/gateway/v1'
    )
  })

  it('does not append /v1 to a full chat completions endpoint', () => {
    expect(
      normalizeApiBaseUrl('https://api.example.com/v1/chat/completions')
    ).toBe('https://api.example.com/v1/chat/completions')
  })

  it('trims the configured address before normalizing', () => {
    expect(normalizeApiBaseUrl('  https://api.example.com/v1/  ')).toBe(
      'https://api.example.com/v1'
    )
  })

  it('falls back to the current origin and appends /v1', () => {
    expect(normalizeApiBaseUrl('', 'https://console.example.com/')).toBe(
      'https://console.example.com/v1'
    )
    expect(
      normalizeApiBaseUrl(undefined, 'https://console.example.com/v1')
    ).toBe('https://console.example.com/v1')
  })

  it('returns an empty string when both the address and fallback are blank', () => {
    expect(normalizeApiBaseUrl('', '')).toBe('')
    expect(normalizeApiBaseUrl(null)).toBe('')
  })
})
