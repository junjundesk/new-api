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
import { describe, expect, it } from 'vitest'

import {
  getApiKeyFormSchema,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../api-key-form'
import {
  MAX_FALLBACK_MODELS,
  createFallbackModel,
  getFallbackIssues,
  getFallbackModelsSchema,
  normalizeFallbackModels,
} from '../fallback'
import type { ApiKey } from '../../types'

const t = ((key: string, options?: Record<string, unknown>) => {
  let text = key
  for (const [name, value] of Object.entries(options ?? {})) {
    text = text.replaceAll(`{{${name}}}`, String(value))
  }
  return text
}) as TFunction

const baseApiKey: ApiKey = {
  id: 1,
  name: 'test',
  key: 'sk-test',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  today_used_quota: 0,
  unlimited_quota: true,
  display_wallet_quota: false,
  expired_time: -1,
  created_time: 1,
  accessed_time: 0,
  group: 'default',
  auto_groups: null,
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
  model_mappings: [],
  fallback_enabled: false,
  fallback_models: [],
}

function validForm(overrides: Record<string, unknown> = {}) {
  return {
    name: 'test',
    remain_quota_dollars: 0,
    unlimited_quota: true,
    display_wallet_quota: false,
    model_limits: [],
    model_mappings: [],
    fallback_enabled: false,
    fallback_models: [],
    allow_ips: '',
    group: 'default',
    auto_groups_mode: 'inherit' as const,
    auto_groups: [],
    cross_group_retry: false,
    tokenCount: 1,
    ...overrides,
  }
}

describe('fallback rows', () => {
  it('assigns a form-local id that survives normalization stripping', () => {
    const first = createFallbackModel()
    const second = createFallbackModel({ group: 'vip', model: 'backup' })

    expect(first.id).toBeTruthy()
    expect(second.id).toBeTruthy()
    expect(first.id).not.toBe(second.id)
    expect(second.group).toBe('vip')
    expect(second.model).toBe('backup')
    expect(second.source_model).toBe('')
  })

  it('trims every field and drops the form-local id from the API payload', () => {
    const rows = [
      createFallbackModel({
        source_model: ' gpt-5 ',
        group: ' vip ',
        model: ' claude-sonnet-4-5 ',
      }),
    ]

    expect(normalizeFallbackModels(rows)).toEqual([
      { source_model: 'gpt-5', group: 'vip', model: 'claude-sonnet-4-5' },
    ])
  })

  it('reports missing group and model while validation is requested', () => {
    const issues = getFallbackIssues([
      createFallbackModel(),
      createFallbackModel({ group: 'vip' }),
    ])

    expect(issues).toEqual([
      { index: 0, field: 'group', code: 'group-required' },
      { index: 0, field: 'model', code: 'model-required' },
      { index: 1, field: 'model', code: 'model-required' },
    ])
    expect(getFallbackIssues([createFallbackModel()], false)).toEqual([])
  })

  it('flags a repeated target only after normalization', () => {
    const issues = getFallbackIssues([
      createFallbackModel({ group: 'vip', model: 'backup' }),
      createFallbackModel({ group: ' vip ', model: ' backup ' }),
      createFallbackModel({ group: 'vip', model: 'other' }),
    ])

    expect(issues).toEqual([
      { index: 1, field: 'model', code: 'duplicate-target' },
    ])
  })

  it('treats the same model in different groups as distinct targets', () => {
    expect(
      getFallbackIssues([
        createFallbackModel({ group: 'vip', model: 'backup' }),
        createFallbackModel({ group: 'azure', model: 'backup' }),
      ])
    ).toEqual([])
  })

  it('rejects more than the configured maximum', () => {
    const rows = Array.from({ length: MAX_FALLBACK_MODELS + 1 }, (_, index) =>
      createFallbackModel({ group: 'vip', model: `backup-${index}` })
    )

    const result = getFallbackModelsSchema(t).safeParse(rows)

    expect(result.success).toBe(false)
    expect(MAX_FALLBACK_MODELS).toBe(5)
  })

  it('accepts the maximum number of distinct targets', () => {
    const rows = Array.from({ length: MAX_FALLBACK_MODELS }, (_, index) =>
      createFallbackModel({ group: 'vip', model: `backup-${index}` })
    )

    expect(getFallbackModelsSchema(t).safeParse(rows).success).toBe(true)
  })
})

describe('API key form fallback validation', () => {
  it('keeps an incomplete list when fallback is disabled so it can be saved', () => {
    const result = getApiKeyFormSchema(t).safeParse(
      validForm({
        fallback_enabled: false,
        fallback_models: [createFallbackModel()],
      })
    )

    expect(result.success).toBe(true)
  })

  it('rejects an incomplete list when fallback is enabled', () => {
    const result = getApiKeyFormSchema(t).safeParse(
      validForm({
        fallback_enabled: true,
        fallback_models: [createFallbackModel({ group: 'vip' })],
      })
    )

    expect(result.success).toBe(false)
    const paths = result.error?.issues.map((issue) => issue.path.join('.')) ?? []
    expect(paths).toContain('fallback_models.0.model')
  })

  it('validates the list only when fallback is enabled', () => {
    const rows = Array.from({ length: MAX_FALLBACK_MODELS + 1 }, (_, index) =>
      createFallbackModel({ group: 'vip', model: `backup-${index}` })
    )

    expect(
      getApiKeyFormSchema(t).safeParse(
        validForm({ fallback_enabled: false, fallback_models: rows })
      ).success
    ).toBe(true)
    expect(
      getApiKeyFormSchema(t).safeParse(
        validForm({ fallback_enabled: true, fallback_models: rows })
      ).success
    ).toBe(false)
  })

  it('sends the enabled flag with trimmed targets and no form-local id', () => {
    const payload = transformFormDataToPayload(
      validForm({
        fallback_enabled: true,
        fallback_models: [
          createFallbackModel({
            source_model: ' gpt-5 ',
            group: ' vip ',
            model: ' backup ',
          }),
        ],
      })
    )

    expect(payload.fallback_enabled).toBe(true)
    expect(payload.fallback_models).toEqual([
      { source_model: 'gpt-5', group: 'vip', model: 'backup' },
    ])
  })

  it('restores stored targets with fresh row ids when editing', () => {
    const defaults = transformApiKeyToFormDefaults({
      ...baseApiKey,
      fallback_enabled: true,
      fallback_models: [{ source_model: '', group: 'vip', model: 'backup' }],
    })

    expect(defaults.fallback_enabled).toBe(true)
    expect(defaults.fallback_models).toHaveLength(1)
    expect(defaults.fallback_models[0].id).toBeTruthy()
    expect(defaults.fallback_models[0].group).toBe('vip')
  })
})
