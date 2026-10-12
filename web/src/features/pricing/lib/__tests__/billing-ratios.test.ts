import { describe, expect, it } from 'vitest'

import type { PricingModel } from '../../types'
import {
  formatDynamicUnitPrice,
  getDynamicPricingSummary,
} from '../dynamic-price'
import { sortModels } from '../filters'
import { getDisplayBillingRatio, getGroupBillingRatio } from '../model-helpers'
import {
  formatPrice,
  formatGroupPrice,
  formatFixedPrice,
  formatRequestPrice,
  stripTrailingZeros,
} from '../price'

const model: PricingModel = {
  id: 1,
  model_name: 'public-model',
  quota_type: 0,
  model_ratio: 2,
  completion_ratio: 3,
  cache_ratio: 0.1,
  create_cache_ratio: 1.25,
  image_ratio: 2,
  audio_ratio: 4,
  audio_completion_ratio: 5,
  enable_groups: ['default', 'vip', 'hidden'],
  group_ratio: { default: 2, vip: 0.5 },
  billing_ratios: {
    default: { min: 0.8, max: 1.2, source: 'channel_model' },
    vip: { min: 0.9, max: 0.9, source: 'user_group' },
  },
}
const compact = (text: string) => stripTrailingZeros(text)

describe('effective billing price contract', () => {
  it('uses channel ranges instead of default group pricing for every token price category', () => {
    const expected = {
      input: '$3.2–$4.8',
      output: '$9.6–$14.4',
      cache: '$0.32–$0.48',
      create_cache: '$4–$6',
      image: '$6.4–$9.6',
      audio_input: '$12.8–$19.2',
      audio_output: '$64–$96',
    }
    for (const [type, price] of Object.entries(expected)) {
      expect(
        compact(
          formatPrice(
            model,
            type as keyof typeof expected,
            'M',
            false,
            1,
            1,
            'default'
          )
        )
      ).toBe(price)
      expect(
        compact(
          formatGroupPrice(
            model,
            'default',
            type as keyof typeof expected,
            'M',
            false,
            1,
            1,
            model.group_ratio!
          )
        )
      ).toBe(price)
    }
  })
  it('preserves unit and recharge conversion across both ends of the range', () => {
    expect(
      compact(formatPrice(model, 'input', 'K', true, 0.5, 1, 'default'))
    ).toBe('$0.0016–$0.0024')
  })
  it('quotes request prices with the same effective range', () => {
    const fixed = { ...model, quota_type: 1, model_price: 2 }
    expect(compact(formatRequestPrice(fixed, false, 1, 1, 'default'))).toBe(
      '$1.6–$2.4'
    )
    expect(
      compact(
        formatFixedPrice(fixed, 'default', false, 1, 1, model.group_ratio!)
      )
    ).toBe('$1.6–$2.4')
  })
  it('uses effective minima for all groups without inventing inaccessible cheaper groups', () => {
    expect(getDisplayBillingRatio(model, 'all')).toMatchObject({
      min: 0.8,
      max: 0.8,
    })
    expect(compact(formatPrice(model, 'input', 'M'))).toBe('$3.2')
  })
  it('supports legacy responses, free overrides, explicit one and mixed same-price sources', () => {
    const legacy = { ...model, billing_ratios: undefined }
    expect(getGroupBillingRatio(legacy, 'default')).toEqual({
      min: 2,
      max: 2,
      source: 'group',
    })
    expect(getGroupBillingRatio(legacy, 'missing').min).toBe(1)
    for (const ratio of [0, 1]) {
      const current = {
        ...model,
        billing_ratios: {
          default: { min: ratio, max: ratio, source: 'mixed' as const },
        },
      }
      expect(getGroupBillingRatio(current, 'default')).toEqual({
        min: ratio,
        max: ratio,
        source: 'mixed',
      })
      expect(
        compact(formatPrice(current, 'input', 'M', false, 1, 1, 'default'))
      ).toBe(`$${4 * ratio}`)
    }
  })
  it('uses the same ratio range for dynamic previews and handles free pricing', () => {
    const dynamic = {
      ...model,
      billing_mode: 'tiered_expr',
      billing_expr: 'tier("standard", p * 4 + c * 12 + cr * 0.4)',
    }
    const summary = getDynamicPricingSummary(dynamic, {
      tokenUnit: 'M',
      billingRatio: getGroupBillingRatio(dynamic, 'default'),
    })
    expect(compact(summary!.primaryEntries[0].formatted)).toBe('$3.2–$4.8')
    expect(
      compact(
        formatDynamicUnitPrice(4, {
          tokenUnit: 'K',
          billingRatio: { min: 0, max: 0 },
        })
      )
    ).toBe('$0')
  })
  it('sorts according to the prices actually displayed for the selected group', () => {
    const cheapBase = {
      ...model,
      id: 2,
      model_name: 'cheap-base',
      model_ratio: 1,
      billing_ratios: {
        default: { min: 5, max: 5, source: 'channel_model' as const },
      },
    }
    expect(
      sortModels([cheapBase, model], 'price-low', 'default').map(
        (entry) => entry.id
      )
    ).toEqual([1, 2])
  })
})
