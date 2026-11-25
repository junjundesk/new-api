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
// Only an operator-supplied web destination. Do not resolve relative URLs
// against the dashboard or forward credentials to a frame.
export function parseCardStoreUrl(value: string): URL | null {
  const raw = value.trim()
  if (
    !/^https?:[/][/][^/?#@]+(?:[/?#]|$)/i.test(raw) ||
    /[\\\s\p{Cc}]/u.test(raw) ||
    new TextEncoder().encode(raw).length > 2048
  ) {
    return null
  }
  try {
    const url = new URL(raw)
    if (!url.hostname || url.username || url.password || url.port === '0') {
      return null
    }
    return url
  } catch {
    return null
  }
}

export function decodeCardStoreSettings(raw: string) {
  try {
    const data: unknown = JSON.parse(raw)
    if (
      typeof data === 'object' &&
      data !== null &&
      'enabled' in data &&
      typeof data.enabled === 'boolean' &&
      'url' in data &&
      typeof data.url === 'string'
    ) {
      return { CardStoreEnabled: data.enabled, CardStoreURL: data.url }
    }
  } catch {
    /* Older servers do not have this option. */
  }
  return { CardStoreEnabled: false, CardStoreURL: '' }
}
