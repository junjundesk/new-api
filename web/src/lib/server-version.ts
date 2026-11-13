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

/**
 * Holds the backend version disclosed only to administrator sessions.
 *
 * The server emits `X-New-Api-Version` exclusively on responses to
 * administrator-authorized endpoints, so the value is captured from any admin
 * API response header rather than from the anonymous `/api/status` payload.
 * Non-admin sessions never receive the header, which keeps the deployed
 * version invisible to regular users and anonymous visitors.
 */

export const SERVER_VERSION_HEADER = 'x-new-api-version'

let serverVersion: string | null = null
const listeners = new Set<() => void>()

function remember(value: string | null): void {
  const next = value?.trim() || null
  if (next === serverVersion) return
  serverVersion = next
  for (const listener of listeners) listener()
}

/**
 * Record the version carried by an administrator response. Safe to call for
 * every response: responses without the header leave the cached value intact.
 */
export function captureServerVersion(headers: unknown): void {
  if (!headers || typeof headers !== 'object') return
  const getter = (headers as { get?: unknown }).get
  const raw =
    typeof getter === 'function'
      ? (getter as (name: string) => unknown).call(headers, SERVER_VERSION_HEADER)
      : (headers as Record<string, unknown>)[SERVER_VERSION_HEADER]

  if (typeof raw === 'string' && raw.trim()) {
    remember(raw)
  }
}

/** Returns the administrator-visible backend version, or null when unknown. */
export function getServerVersion(): string | null {
  return serverVersion
}

/** Subscribe to version updates; returns an unsubscribe function. */
export function subscribeServerVersion(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** Clear the cached version, e.g. when the administrator signs out. */
export function resetServerVersion(): void {
  remember(null)
}
