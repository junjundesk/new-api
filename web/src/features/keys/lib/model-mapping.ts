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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { tokenModelMappingSchema, type TokenModelMapping } from '../types'

export const MAX_MODEL_MAPPINGS = 100

export const REASONING_EFFORT_PRESETS = new Set([
  '',
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
])

const INVALID_MODEL_NAME_PATTERN = /[\p{White_Space}\p{Cc}]/u
const REASONING_EFFORT_PATTERN = /^[A-Za-z0-9._-]*$/

export type ModelMappingIssueCode =
  | 'source-required'
  | 'target-required'
  | 'duplicate-source'
  | 'unavailable-target'
  | 'invalid-model-name'
  | 'invalid-effort'

export type ModelMappingIssue = {
  index: number
  field: 'source_model' | 'target_model' | 'reasoning_effort'
  code: ModelMappingIssueCode
}

function isInvalidModelName(name: string) {
  return [...name].length > 255 || INVALID_MODEL_NAME_PATTERN.test(name)
}

export function createMappingRowId() {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
}

/**
 * Validates mappings with the same rules the backend enforces. When
 * `availableModels` is provided, targets outside that set are reported.
 */
export function getModelMappingIssues(
  mappings: TokenModelMapping[],
  availableModels?: string[]
): ModelMappingIssue[] {
  const issues: ModelMappingIssue[] = []
  const sourceCounts = new Map<string, number>()
  for (const mapping of mappings) {
    const source = mapping.source_model.trim()
    if (source) sourceCounts.set(source, (sourceCounts.get(source) ?? 0) + 1)
  }
  const available = availableModels ? new Set(availableModels) : undefined

  mappings.forEach((mapping, index) => {
    const source = mapping.source_model.trim()
    const target = mapping.target_model.trim()
    const effort = mapping.reasoning_effort?.trim() ?? ''

    if (!source) {
      issues.push({ index, field: 'source_model', code: 'source-required' })
    } else if ((sourceCounts.get(source) ?? 0) > 1) {
      issues.push({ index, field: 'source_model', code: 'duplicate-source' })
    } else if (isInvalidModelName(source)) {
      issues.push({ index, field: 'source_model', code: 'invalid-model-name' })
    }

    if (!target) {
      issues.push({ index, field: 'target_model', code: 'target-required' })
    } else if (isInvalidModelName(target)) {
      issues.push({ index, field: 'target_model', code: 'invalid-model-name' })
    } else if (available && !available.has(target)) {
      issues.push({ index, field: 'target_model', code: 'unavailable-target' })
    }

    if (
      new TextEncoder().encode(effort).length > 64 ||
      !REASONING_EFFORT_PATTERN.test(effort)
    ) {
      issues.push({ index, field: 'reasoning_effort', code: 'invalid-effort' })
    }
  })
  return issues
}

export function getModelMappingIssueMessage(
  issue: ModelMappingIssue,
  t: TFunction
) {
  switch (issue.code) {
    case 'source-required':
      return t('Enter the model name sent by your client')
    case 'target-required':
      return t('Select a target model')
    case 'duplicate-source':
      return t('Each source model can only have one mapping')
    case 'unavailable-target':
      return t(
        'This target model is unavailable in the selected group. Select another model or remove this mapping.'
      )
    case 'invalid-model-name':
      return t(
        'Model names must be at most 255 characters and contain no spaces or control characters'
      )
    case 'invalid-effort':
      return t(
        'Use up to 64 letters, numbers, dots, underscores or hyphens for reasoning effort'
      )
  }
}

export function getModelMappingsSchema(
  t: TFunction,
  availableModels?: string[]
) {
  return z
    .array(tokenModelMappingSchema)
    .max(MAX_MODEL_MAPPINGS, t('Use at most 100 model mappings'))
    .superRefine((mappings, ctx) => {
      for (const issue of getModelMappingIssues(mappings, availableModels)) {
        ctx.addIssue({
          code: 'custom',
          path: [issue.index, issue.field],
          message: getModelMappingIssueMessage(issue, t),
        })
      }
    })
}

export function normalizeModelMappings(
  mappings: TokenModelMapping[]
): TokenModelMapping[] {
  return mappings.map((mapping) => {
    const effort = mapping.reasoning_effort?.trim()
    return {
      source_model: mapping.source_model.trim(),
      target_model: mapping.target_model.trim(),
      ...(effort ? { reasoning_effort: effort } : {}),
    }
  })
}

/**
 * Merges preset mappings into the current list. Without `replace`, any
 * source-model conflict is returned unapplied so the caller can confirm.
 */
export function applyModelMappingPreset(
  current: TokenModelMapping[],
  presetMappings: TokenModelMapping[],
  replace = false
): { mappings: TokenModelMapping[]; conflicts: string[] } {
  const presetSources = new Set(
    presetMappings.map((mapping) => mapping.source_model.trim())
  )
  const conflicts = [
    ...new Set(
      current
        .map((mapping) => mapping.source_model.trim())
        .filter((source) => presetSources.has(source))
    ),
  ]
  if (conflicts.length > 0 && !replace) {
    return { mappings: current, conflicts }
  }
  return {
    mappings: [
      ...current.filter(
        (mapping) => !presetSources.has(mapping.source_model.trim())
      ),
      ...presetMappings.map((mapping) => ({ ...mapping })),
    ],
    conflicts,
  }
}
