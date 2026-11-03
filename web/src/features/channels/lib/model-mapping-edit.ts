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

import { normalizeCandidateModels } from './model-mapping-validation'

/**
 * Parse model_mapping JSON into trimmed public name -> upstream candidates.
 * Returns null when the stored value is not a valid mapping object.
 */
export function parseModelMappingEntries(
  modelMapping: string
): Record<string, string[]> | null {
  const trimmed = (modelMapping || '').trim()
  if (!trimmed) return {}

  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null
    }

    const entries: Record<string, string[]> = {}
    for (const [rawName, rawValue] of Object.entries(parsed)) {
      const name = rawName.trim()
      const targets = normalizeCandidateModels(rawValue)
      if (!name || targets === null) return null
      const cleaned = targets.map((target) => target.trim()).filter(Boolean)
      if (cleaned.length === 0) continue
      entries[name] = cleaned
    }
    return entries
  } catch {
    return null
  }
}

/**
 * Read the public model names that currently forward to one upstream model.
 */
export function getExposedNames(
  modelMapping: string,
  upstreamModel: string
): string[] | null {
  const entries = parseModelMappingEntries(modelMapping)
  if (entries === null) return null
  const upstream = upstreamModel.trim()
  return Object.keys(entries).filter((name) => entries[name].includes(upstream))
}

/**
 * Move one upstream model to a new public name, dropping its previous public
 * name when that name would otherwise become a separate alias.
 * Returns a null mapping when the JSON is invalid, and conflict=true when the
 * target public name is already taken by another entry.
 */
export function setUpstreamExposedName(
  modelMapping: string,
  upstreamModel: string,
  previousName: string,
  nextName: string
): { mapping: string | null; conflict: boolean } {
  const upstream = upstreamModel.trim()
  const previous = previousName.trim()
  const next = nextName.trim()
  if (!upstream) return { mapping: null, conflict: false }

  const entries = parseModelMappingEntries(modelMapping)
  if (entries === null) return { mapping: null, conflict: false }

  if (previous === next) {
    return { mapping: serializeModelMappingEntries(entries), conflict: false }
  }

  if (previous) {
    const candidates = (entries[previous] ?? []).filter(
      (candidate) => candidate !== upstream
    )
    if (candidates.length > 0) {
      entries[previous] = candidates
    } else {
      delete entries[previous]
    }
  }

  if (next) {
    if (entries[next]) return { mapping: null, conflict: true }
    entries[next] = [upstream]
  }

  return { mapping: serializeModelMappingEntries(entries), conflict: false }
}

/**
 * Parse a single public model name typed into the inline editor.
 * An empty value is allowed and removes the mapping.
 */
export function parseExposedNameInput(value: string): {
  name: string
  error: 'invalid-name' | null
} {
  const name = value.trim()
  if (!name) return { name: '', error: null }
  if (name.length > 255 || /[\s,]/.test(name)) {
    return { name, error: 'invalid-name' }
  }
  return { name, error: null }
}

function serializeModelMappingEntries(
  entries: Record<string, string[]>
): string {
  const names = Object.keys(entries)
  if (names.length === 0) return ''

  const serialized: Record<string, string | string[]> = {}
  for (const name of names) {
    const targets = entries[name]
    serialized[name] = targets.length === 1 ? targets[0] : targets
  }
  return JSON.stringify(serialized, null, 2)
}
