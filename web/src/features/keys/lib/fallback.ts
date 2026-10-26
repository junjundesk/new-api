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
import { nanoid } from 'nanoid'
import { z } from 'zod'

import type { TokenFallbackModel } from '../types'

export const MAX_FALLBACK_MODELS = 5

// The id is form-local identity, never part of the API payload. It keeps row
// state attached to the same configured target while rows are reordered.
export function createFallbackModel(
  value?: Partial<TokenFallbackModel>
): TokenFallbackModel {
  return {
    id: value?.id ?? nanoid(),
    source_model: value?.source_model ?? '',
    group: value?.group ?? '',
    model: value?.model ?? '',
  }
}

// Form rows always carry a source model value (empty means "all models"), so
// the form schema keeps it required while the API schema tolerates it missing.
export const fallbackModelFormSchema = z.object({
  id: z.string().optional(),
  source_model: z.string(),
  group: z.string(),
  model: z.string(),
})

export type FallbackIssueCode =
  | 'group-required'
  | 'model-required'
  | 'duplicate-target'

export type FallbackIssue = {
  index: number
  field: 'source_model' | 'group' | 'model'
  code: FallbackIssueCode
}

export function getFallbackIssues(
  fallbacks: TokenFallbackModel[],
  showRequired = true
): FallbackIssue[] {
  const issues: FallbackIssue[] = []
  const seen = new Map<string, number>()
  fallbacks.forEach((fallback, index) => {
    const group = fallback.group.trim()
    const model = fallback.model.trim()
    const source = fallback.source_model?.trim() ?? ''
    if (!group) {
      issues.push({ index, field: 'group', code: 'group-required' })
    }
    if (!model) {
      issues.push({ index, field: 'model', code: 'model-required' })
    }
    if (!group || !model) return

    const key = `${source}\u0000${group}\u0000${model}`
    const previous = seen.get(key)
    if (previous !== undefined) {
      issues.push({ index, field: 'model', code: 'duplicate-target' })
      return
    }
    seen.set(key, index)
  })
  return showRequired
    ? issues
    : issues.filter((issue) => issue.code === 'duplicate-target')
}

export function getFallbackIssueMessage(issue: FallbackIssue, t: TFunction) {
  switch (issue.code) {
    case 'group-required':
      return t('Select a fallback group')
    case 'model-required':
      return t('Select a fallback model')
    case 'duplicate-target':
      return t('This fallback target is already configured')
  }
}

export function getFallbackModelsSchema(t: TFunction) {
  return z
    .array(fallbackModelFormSchema)
    .max(MAX_FALLBACK_MODELS, t('Use at most {{max}} fallback models', { max: MAX_FALLBACK_MODELS }))
    .superRefine((fallbacks, ctx) => {
      for (const issue of getFallbackIssues(fallbacks)) {
        ctx.addIssue({
          code: 'custom',
          path: [issue.index, issue.field],
          message: getFallbackIssueMessage(issue, t),
        })
      }
    })
}

export function normalizeFallbackModels(
  fallbacks: TokenFallbackModel[]
): TokenFallbackModel[] {
  return fallbacks.map((fallback) => ({
    source_model: fallback.source_model?.trim() ?? '',
    group: fallback.group.trim(),
    model: fallback.model.trim(),
  }))
}
