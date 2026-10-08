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
// ============================================================================
// Model Mapping Validation Utilities
// ============================================================================

/**
 * Upper bound for the replacement candidates of a single source model. The
 * relay tries them in order, so the backend rejects longer lists.
 */
export const MAX_CANDIDATE_MODELS_PER_SOURCE = 32

/**
 * Normalize one model_mapping value into a candidate list.
 * A single replacement is stored as a string, several as an array.
 */
export function normalizeCandidateModels(value: unknown): string[] | null {
  if (typeof value === 'string') return [value]
  if (!Array.isArray(value)) return null
  const candidates: string[] = []
  for (const item of value) {
    if (typeof item !== 'string') return null
    candidates.push(item)
  }
  return candidates
}

/**
 * Parse models string to array
 */
export function parseModelsString(modelsStr: string): string[] {
  return modelsStr
    ? modelsStr
        .split(',')
        .map((m) => m.trim())
        .filter(Boolean)
    : []
}

/**
 * Format models array to string
 */
export function formatModelsArray(models: string[]): string {
  return Array.from(new Set(models)).join(',')
}

/**
 * Normalize model name
 */
export function normalizeModelName(model: string): string {
  return typeof model === 'string' ? model.trim() : ''
}

/**
 * Extract source keys from model_mapping JSON
 * (the keys of the mapping object — models being remapped FROM)
 */
export function extractMappingSourceModels(modelMapping: string): string[] {
  if (typeof modelMapping !== 'string') return []
  const trimmed = modelMapping.trim()
  if (!trimmed) return []

  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return []
    }

    const keys = Object.keys(parsed)
      .map((key) => key.trim())
      .filter(Boolean)

    return Array.from(new Set(keys))
  } catch {
    return []
  }
}

/**
 * Extract redirect models from model_mapping JSON.
 * Sources with several candidates expand to every candidate.
 */
export function extractRedirectModels(modelMapping: string): string[] {
  const mapping = modelMapping
  if (typeof mapping !== 'string') return []
  const trimmed = mapping.trim()
  if (!trimmed) return []

  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return []
    }

    const values = Object.values(parsed)
      .flatMap((value) => normalizeCandidateModels(value) ?? [])
      .map((value) => value.trim())
      .filter((value): value is string => Boolean(value))

    return Array.from(new Set(values))
  } catch {
    return []
  }
}

/**
 * Check if model configuration has changed
 */
export function hasModelConfigChanged(
  currentModels: string[],
  currentModelMapping: string,
  initialModels: string[],
  initialModelMapping: string
): boolean {
  // Always return true if not editing (new channel)
  if (initialModels.length === 0 && !initialModelMapping) {
    return true
  }

  // Check if models array changed
  if (currentModels.length !== initialModels.length) {
    return true
  }
  for (let i = 0; i < currentModels.length; i++) {
    if (currentModels[i] !== initialModels[i]) {
      return true
    }
  }

  // Check if model_mapping changed
  const normalizedCurrent = (currentModelMapping || '').trim()
  const normalizedInitial = (initialModelMapping || '').trim()

  return normalizedCurrent !== normalizedInitial
}

/**
 * Find models in model_mapping that are missing from the models list
 */
export function findMissingModelsInMapping(
  modelMapping: string,
  currentModels: string[]
): string[] {
  if (!modelMapping || modelMapping.trim() === '') {
    return []
  }

  let parsedMapping: Record<string, unknown>
  try {
    parsedMapping = JSON.parse(modelMapping)
    if (
      !parsedMapping ||
      typeof parsedMapping !== 'object' ||
      Array.isArray(parsedMapping)
    ) {
      return []
    }
  } catch {
    return []
  }

  const modelSet = new Set(currentModels.map((m) => normalizeModelName(m)))
  const missingModels = Object.keys(parsedMapping)
    .map((key) => normalizeModelName(key))
    .filter((key) => key && !modelSet.has(key))

  return Array.from(new Set(missingModels))
}

/**
 * Validate model mapping JSON format
 */
export function validateModelMappingJson(modelMapping: string): {
  valid: boolean
  error?: string
} {
  if (!modelMapping || modelMapping.trim() === '') {
    return { valid: true }
  }

  try {
    const parsed = JSON.parse(modelMapping)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {
        valid: false,
        error: 'Model mapping must be a valid JSON object',
      }
    }
    const hasInvalidValue = Object.values(parsed).some(
      (value) => normalizeCandidateModels(value) === null
    )
    if (hasInvalidValue) {
      return {
        valid: false,
        error:
          'Model mapping values must be a model name or an array of model names',
      }
    }
    // Blank sources and blank candidates are dropped instead of rejected, which
    // matches how the backend normalizes the payload.
    for (const [rawSource, value] of Object.entries(parsed)) {
      const source = rawSource.trim()
      const candidates = (normalizeCandidateModels(value) ?? [])
        .map((candidate) => candidate.trim())
        .filter(Boolean)
      if (!source) continue
      if (!isUsableModelName(source)) {
        return {
          valid: false,
          error: 'Model names must not contain spaces',
        }
      }
      if (candidates.length > MAX_CANDIDATE_MODELS_PER_SOURCE) {
        return {
          valid: false,
          error: `Use at most ${MAX_CANDIDATE_MODELS_PER_SOURCE} candidate models per source`,
        }
      }
      if (candidates.some((candidate) => !isUsableModelName(candidate))) {
        return {
          valid: false,
          error: 'Model names must not contain spaces',
        }
      }
    }
    return { valid: true }
  } catch {
    return {
      valid: false,
      error: 'Model mapping must be valid JSON format',
    }
  }
}

function isUsableModelName(name: string): boolean {
  if (name.length > 255) return false
  return !/\s/.test(name)
}

/**
 * Get redirect models that are also in the models list
 * (These should be removed from models list to keep /v1/models clean)
 */
export function findExposedTargetModels(
  modelMapping: string,
  currentModels: string[]
): string[] {
  const redirectModels = extractRedirectModels(modelMapping)
  if (redirectModels.length === 0) return []

  const normalizedModels = currentModels.map((m) => normalizeModelName(m))
  const modelSet = new Set(normalizedModels)

  return redirectModels.filter((model) =>
    modelSet.has(normalizeModelName(model))
  )
}

/**
 * Categorize models into different sets for UI display
 */
export function categorizeModelsWithRedirect(
  currentModels: string[],
  redirectModels: string[]
): {
  normalizedCurrentModels: Set<string>
  normalizedRedirectModels: Set<string>
  classificationSet: Set<string>
  redirectOnlySet: Set<string>
} {
  const normalizedCurrentModels = new Set(
    currentModels.map((m) => normalizeModelName(m)).filter(Boolean)
  )

  const normalizedRedirectModels = new Set(
    redirectModels.map((m) => normalizeModelName(m)).filter(Boolean)
  )

  const classificationSet = new Set([
    ...normalizedCurrentModels,
    ...normalizedRedirectModels,
  ])

  const redirectOnlySet = new Set(
    Array.from(normalizedRedirectModels).filter(
      (m) => !normalizedCurrentModels.has(m)
    )
  )

  return {
    normalizedCurrentModels,
    normalizedRedirectModels,
    classificationSet,
    redirectOnlySet,
  }
}
