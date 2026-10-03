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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import i18next from 'i18next'

import { lazyLocaleBackend } from '../lazy-locale-backend'
import en from '../locales/en.json'
import zhCN from '../locales/zh.json'

const NON_BUNDLED_LANGUAGES = ['fr', 'ja', 'ru', 'vi', 'zhTW']

async function createInstance(lng: string) {
  const instance = i18next.createInstance()
  await instance.use(lazyLocaleBackend).init({
    resources: { en, zhCN },
    partialBundledLanguages: true,
    fallbackLng: 'en',
    supportedLngs: ['en', 'zhCN', ...NON_BUNDLED_LANGUAGES],
    load: 'currentOnly',
    nsSeparator: false,
    lng,
  })
  return instance
}

describe('lazy locale backend', () => {
  test('bundled languages translate from the static resources', async () => {
    const i18n = await createInstance('zhCN')

    const key = Object.keys(zhCN.translation)[0]
    assert.equal(i18n.t(key), Object.values(zhCN.translation)[0])
  })

  for (const language of NON_BUNDLED_LANGUAGES) {
    test(`loads ${language} on demand`, async () => {
      const i18n = await createInstance(language)

      assert.equal(i18n.hasResourceBundle(language, 'translation'), true)
      // The backend must hand i18next the namespace layer, not the
      // `{ translation: ... }` wrapper; otherwise every key stays untranslated
      // and silently falls back to English (or to the raw key).
      assert.notEqual(i18n.t('Dashboard'), en.translation.Dashboard)
      assert.notEqual(i18n.t('Dashboard'), 'Dashboard')
    })
  }
})
