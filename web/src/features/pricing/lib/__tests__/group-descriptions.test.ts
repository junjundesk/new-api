import { describe, expect, it } from 'vitest'

import type { PricingModel } from '../../types'
import { applyGroupDescriptions } from '../model-helpers'

function model(name: string, description?: string): PricingModel {
  return {
    id: 1,
    model_name: name,
    description,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['vip'],
  }
}

describe('applyGroupDescriptions', () => {
  it('keeps the original list when the selected group has no descriptions', () => {
    const models = [model('gpt-4o', 'global')]
    expect(applyGroupDescriptions(models, undefined)).toBe(models)
    expect(applyGroupDescriptions(models, {})).toBe(models)
  })

  it('overrides only the models described for the selected group', () => {
    const models = [model('gpt-4o', 'global'), model('claude', 'claude global')]

    const result = applyGroupDescriptions(models, {
      'gpt-4o': 'VIP 专享',
    })

    expect(result[0]?.description).toBe('VIP 专享')
    expect(result[1]?.description).toBe('claude global')
    expect(models[0]?.description).toBe('global')
  })

  it('keeps the original list when the group description equals the model one', () => {
    const models = [model('gpt-4o', 'global')]

    expect(applyGroupDescriptions(models, { 'gpt-4o': 'global' })).toBe(models)
  })
})
