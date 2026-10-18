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
const API_VERSION_PATH_PATTERN = /(?:^|\/)v1(?:beta)?(?:\/|$)/

/**
 * Normalize a configured API address into the base URL clients should use.
 *
 * The relay exposes its OpenAI-compatible routes under `/v1`, so an address
 * without a version segment gets one appended. Addresses that already carry a
 * `/v1` or `/v1beta` segment are kept as-is, including full endpoint URLs.
 */
export function normalizeApiBaseUrl(
  address?: string | null,
  fallbackOrigin = ''
): string {
  const base = (address?.trim() || fallbackOrigin.trim()).replace(/\/+$/, '')
  if (!base) return ''

  let pathname = base
  try {
    pathname = new URL(base).pathname
  } catch {
    // Keep non-absolute values as-is for the version segment check.
  }

  if (API_VERSION_PATH_PATTERN.test(pathname)) return base

  return `${base}/v1`
}
