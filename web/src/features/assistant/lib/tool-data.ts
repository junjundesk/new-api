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
import type { AssistantCreateApiKeyData } from '../types'

function readRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function readString(value: unknown): string | undefined {
  if (typeof value !== 'string' || value.trim() === '') return undefined
  return value.trim()
}

function readNumber(value: unknown): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) return undefined
  return value
}

/**
 * Mask a secret API key for display. Already-masked values (the backend masks
 * created keys before they ever reach the browser) are returned unchanged.
 */
export function maskApiKey(key: string): string {
  const trimmed = key.trim()
  if (trimmed === '') return ''
  const secret = trimmed.startsWith('sk-') ? trimmed.slice(3) : trimmed
  if (secret.includes('*')) return `sk-${secret}`
  if (secret.length <= 8) return 'sk-********'
  return `sk-${secret.slice(0, 4)}**********${secret.slice(-4)}`
}

export function parseCreateApiKeyResult(
  value: unknown
): AssistantCreateApiKeyData | null {
  const record = readRecord(value)
  if (!record) return null
  const name = readString(record.name)
  const rawKey =
    readString(record.masked_key) ??
    readString(record.maskedKey) ??
    readString(record.key)
  if (!name || !rawKey) return null

  const id = readNumber(record.id)
  return {
    id: id !== undefined && Number.isInteger(id) && id > 0 ? id : undefined,
    name,
    maskedKey: maskApiKey(rawKey),
    statusText: readString(record.status_text),
    unlimitedQuota: record.unlimited_quota === true,
    expiredTime: readNumber(record.expired_time),
    neverExpires: record.never_expires === true,
    group: readString(record.group),
  }
}
