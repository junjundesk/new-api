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
import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'

import enLocale from '@/i18n/locales/en.json'
import zhLocale from '@/i18n/locales/zh.json'

import { ACTIVITY_CODE_TYPE, ACTIVITY_LIMIT_MODE } from '../../constants'
import {
  type ActivityBenefitRow,
  createActivityBenefitRow,
} from '../../lib/activity-benefit'
import {
  ACTIVITY_CODE_FORM_DEFAULT_VALUES,
  type ActivityCodeFormValues,
} from '../../lib/activity-form'
import { ActivityBenefitsEditor } from '../activity-benefits-editor'

const LIMIT_TYPE_KEY = 'Limit type for benefit {{index}}'
const CONCURRENCY_OPTION_KEY = 'Concurrency'
const CONCURRENCY_HELP_KEY =
  'Concurrency is shared per user and group; a single-model benefit is not its own pool. 0 means unlimited.'

const en = enLocale.translation as Record<string, string>
const zh = zhLocale.translation as Record<string, string>

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: { translation: en },
    zh: { translation: zh },
  },
  interpolation: { escapeValue: false },
})

function Fixture(props: { rows: ActivityBenefitRow[] }) {
  const [queryClient] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } })
  )
  const form = useForm<ActivityCodeFormValues>({
    defaultValues: {
      ...ACTIVITY_CODE_FORM_DEFAULT_VALUES,
      type: ACTIVITY_CODE_TYPE.BENEFIT,
      benefits: props.rows,
    },
  })
  return (
    <QueryClientProvider client={queryClient}>
      <ActivityBenefitsEditor form={form} groupNames={['vip']} />
    </QueryClientProvider>
  )
}

function concurrencyRow(): ActivityBenefitRow {
  return createActivityBenefitRow({
    group_name: 'vip',
    limit_mode: ACTIVITY_LIMIT_MODE.CONCURRENCY,
    concurrency_limit: 10,
  })
}

// 带插值参数的键按 i18next 规则把 {{index}} 替换为行号。
function rowLabel(localeValue: string): string {
  return localeValue.replace('{{index}}', '1')
}

afterEach(async () => {
  cleanup()
  await i18n.changeLanguage('en')
})

describe('limit type copy follows the active language', () => {
  it('renders the limit type field, concurrency option and shared-pool help in English', async () => {
    const user = userEvent.setup()
    render(
      <I18nextProvider i18n={i18n}>
        <Fixture rows={[concurrencyRow()]} />
      </I18nextProvider>
    )

    expect(
      screen.getByRole('combobox', {
        name: rowLabel(en[LIMIT_TYPE_KEY]),
      })
    ).toBeVisible()
    await user.click(
      screen.getByRole('combobox', { name: rowLabel(en[LIMIT_TYPE_KEY]) })
    )
    expect(
      await screen.findByRole('option', { name: en[CONCURRENCY_OPTION_KEY] })
    ).toBeVisible()
    expect(screen.getByText(en[CONCURRENCY_HELP_KEY])).toBeVisible()
  })

  it('switches the limit type label, option and concurrency help to Chinese', async () => {
    render(
      <I18nextProvider i18n={i18n}>
        <Fixture rows={[concurrencyRow()]} />
      </I18nextProvider>
    )
    expect(
      screen.getByRole('combobox', { name: rowLabel(en[LIMIT_TYPE_KEY]) })
    ).toBeVisible()

    await act(async () => {
      await i18n.changeLanguage('zh')
    })

    // 中文文案必须来自 zh.json 的翻译，而不是英文原文的回退。
    expect(zh[LIMIT_TYPE_KEY]).not.toBe(en[LIMIT_TYPE_KEY])
    expect(zh[CONCURRENCY_HELP_KEY]).not.toBe(en[CONCURRENCY_HELP_KEY])
    expect(
      screen.getByRole('combobox', { name: rowLabel(zh[LIMIT_TYPE_KEY]) })
    ).toBeVisible()
    expect(screen.getByText(zh[CONCURRENCY_HELP_KEY])).toBeVisible()
    expect(screen.queryByText(en[CONCURRENCY_HELP_KEY])).not.toBeInTheDocument()
  })
})

describe('limit type locale coverage', () => {
  it('translates every limit type and concurrency label in both locales', () => {
    const keys = [
      'Limit type',
      LIMIT_TYPE_KEY,
      'Choose how usage is limited. The unused limit type is not enforced.',
      CONCURRENCY_OPTION_KEY,
      'Concurrency limit',
      'Concurrency for benefit {{index}}',
      'Concurrent in-flight requests per user in this group. 0 means unlimited.',
      CONCURRENCY_HELP_KEY,
      'Rate limit',
    ]

    for (const key of keys) {
      expect(en[key], `missing en value for ${key}`).toBeTruthy()
      expect(zh[key], `missing zh value for ${key}`).toBeTruthy()
    }
    expect(zh['Rate limit']).not.toBe(en['Rate limit'])
    expect(zh['Limit type']).not.toBe(en['Limit type'])
    expect(zh['Concurrency limit']).not.toBe(en['Concurrency limit'])
  })
})
