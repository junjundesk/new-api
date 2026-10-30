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
import { createInstance } from 'i18next'
import { describe, expect, it } from 'vitest'

import type { ActivityBenefit } from '../../types'
import {
  describeActivityBenefits,
  describeActivityModelScope,
  describeTokenUsage,
} from '../activity-format'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'All models in the group': 'All models in the group',
        'Free of charge': 'Free of charge',
        '{{ratio}}x ratio': '{{ratio}}x ratio',
        'Whole group': 'Whole group',
        'Model {{model}}': 'Model {{model}}',
        '{{amount}} tokens': '{{amount}} tokens',
        'Unlimited tokens': 'Unlimited tokens',
        'Used {{used}} / {{total}}': 'Used {{used}} / {{total}}',
        'Remaining {{amount}}': 'Remaining {{amount}}',
        'Token quota used up (billed at the original price)':
          'Token quota used up (billed at the original price)',
        Unlimited: 'Unlimited',
      },
    },
  },
  interpolation: { escapeValue: false },
})

// 中文文案单独用一份资源，用来钉住用户可见的最终展示结果。
const zhI18n = createInstance()
await zhI18n.init({
  lng: 'zh',
  resources: {
    zh: {
      translation: {
        'All models in the group': '分组内全部模型',
        'Free of charge': '免费',
        '{{ratio}}x ratio': '{{ratio}}x倍率',
        'Whole group': '整个分组',
        'Model {{model}}': '模型 {{model}}',
        '{{amount}} tokens': '{{amount}} Token',
        'Unlimited tokens': '不限量 Token',
        'Used {{used}} / {{total}}': '已用 {{used}} / {{total}}',
        'Remaining {{amount}}': '剩余 {{amount}}',
        'Token quota used up (billed at the original price)':
          'Token 已用完（按原价计费）',
        Unlimited: '无限制',
      },
    },
  },
  interpolation: { escapeValue: false },
})

function benefitFixture(
  overrides: Partial<ActivityBenefit> = {}
): ActivityBenefit {
  return {
    key: 'benefit-1',
    group_name: 'hdfree',
    group_desc: 'Free Group',
    scope: 'group',
    model_name: '',
    rpm_limit: 0,
    token_total: 0,
    ...overrides,
  }
}

describe('describeActivityModelScope', () => {
  it('reports the whole group when no model is listed', () => {
    expect(describeActivityModelScope(i18n.t.bind(i18n), null)).toBe(
      'All models in the group'
    )
    expect(describeActivityModelScope(i18n.t.bind(i18n), undefined)).toBe(
      'All models in the group'
    )
    expect(describeActivityModelScope(i18n.t.bind(i18n), {})).toBe(
      'All models in the group'
    )
  })

  it('lists the concrete model ids with their own ratios', () => {
    expect(describeActivityModelScope(i18n.t.bind(i18n), { 'gpt-5': 0 })).toBe(
      'gpt-5: Free of charge'
    )
    expect(
      describeActivityModelScope(i18n.t.bind(i18n), { 'gpt-5': 0, o3: 0.5 })
    ).toBe('gpt-5: Free of charge, o3: 0.5x ratio')
  })

  it('keeps at least one decimal so 1 reads as 1.0x', () => {
    expect(describeActivityModelScope(i18n.t.bind(i18n), { 'gpt-5': 1 })).toBe(
      'gpt-5: 1.0x ratio'
    )
    expect(
      describeActivityModelScope(i18n.t.bind(i18n), { 'gpt-5': 0.25 })
    ).toBe('gpt-5: 0.25x ratio')
  })

  it('renders the Chinese copy shown in the activity center', () => {
    expect(
      describeActivityModelScope(zhI18n.t.bind(zhI18n), {
        'deepseek-flash': 0,
        'deepseek-v4-pro': 1,
      })
    ).toBe('deepseek-flash: 免费, deepseek-v4-pro: 1.0x倍率')
  })
})

describe('describeActivityBenefits', () => {
  it('lists every benefit with its group, scope and token total in English units', () => {
    expect(
      describeActivityBenefits(
        i18n.t.bind(i18n),
        [
          benefitFixture({
            group_desc: 'Free Group',
            scope: 'group',
            token_total: 10_000_000,
          }),
          benefitFixture({
            key: 'benefit-2',
            group_name: 'hdpro',
            group_desc: 'Pro Group',
            scope: 'model',
            model_name: 'gpt-5',
            token_total: 0,
          }),
        ],
        'en'
      )
    ).toBe(
      'Free Group · Whole group · 10M tokens\nPro Group · Model gpt-5 · Unlimited tokens'
    )
  })

  it('renders one line per benefit with Chinese 万 units', () => {
    expect(
      describeActivityBenefits(
        zhI18n.t.bind(zhI18n),
        [
          benefitFixture({
            group_desc: '免费分组',
            scope: 'group',
            token_total: 10_000_000,
          }),
          benefitFixture({
            key: 'benefit-2',
            group_name: '专业分组',
            group_desc: '',
            scope: 'model',
            model_name: 'gpt-5',
            token_total: 0,
          }),
        ],
        'zhCN'
      )
    ).toBe(
      '免费分组 · 整个分组 · 1000万 Token\n专业分组 · 模型 gpt-5 · 不限量 Token'
    )
  })

  it('returns an empty string when the old API has no benefits', () => {
    expect(describeActivityBenefits(i18n.t.bind(i18n), null, 'en')).toBe('')
    expect(describeActivityBenefits(i18n.t.bind(i18n), [], 'en')).toBe('')
  })
})

describe('describeTokenUsage', () => {
  it('shows the used, total and remaining amount with English units', () => {
    expect(
      describeTokenUsage(i18n.t.bind(i18n), 250_000, 1_000_000, 'en')
    ).toBe('Used 250K / 1M · Remaining 750K')
  })

  it('shows the original-price fallback once the token package is used up', () => {
    expect(
      describeTokenUsage(i18n.t.bind(i18n), 1_000_000, 1_000_000, 'en')
    ).toBe('Used 1M / 1M · Token quota used up (billed at the original price)')
  })

  it('renders an unlimited package without a remaining amount', () => {
    expect(describeTokenUsage(i18n.t.bind(i18n), 10_000, 0, 'en')).toBe(
      'Used 10K / Unlimited'
    )
  })

  it('renders Chinese 万 units and the exhausted copy', () => {
    expect(
      describeTokenUsage(zhI18n.t.bind(zhI18n), 2_500_000, 10_000_000, 'zhCN')
    ).toBe('已用 250万 / 1000万 · 剩余 750万')
    expect(
      describeTokenUsage(zhI18n.t.bind(zhI18n), 10_000_000, 10_000_000, 'zhCN')
    ).toBe('已用 1000万 / 1000万 · Token 已用完（按原价计费）')
    expect(describeTokenUsage(zhI18n.t.bind(zhI18n), 10_000, 0, 'zhCN')).toBe(
      '已用 1万 / 无限制'
    )
  })
})
