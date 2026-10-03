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
import type { BackendModule, ReadCallback } from 'i18next'

// Non-bundled locales are code-split by the bundler: each dynamic import becomes
// its own chunk, so the initial JS payload only carries the bundled languages.
const localeLoaders: Record<string, () => Promise<{ default: unknown }>> = {
  fr: () => import('./locales/fr.json'),
  ja: () => import('./locales/ja.json'),
  ru: () => import('./locales/ru.json'),
  vi: () => import('./locales/vi.json'),
  zhTW: () => import('./locales/zh-TW.json'),
}

/**
 * i18next backend that serves the code-split locale chunks.
 *
 * `partialBundledLanguages` in `config.ts` makes i18next consult this backend
 * for every language, including the statically bundled ones; those resolve to
 * an empty bundle here because their resources already live in the store.
 */
export const lazyLocaleBackend: BackendModule = {
  type: 'backend',
  init() {},
  read(language: string, namespace: string, callback: ReadCallback) {
    const loader = localeLoaders[language]
    if (!loader) {
      callback(null, {})
      return
    }

    const load = async () => {
      try {
        const module = await loader()
        // Locale files are shaped `{ translation: { ...keys } }`; i18next stores
        // backend results under [lng][namespace], so hand it the namespace layer.
        const resource = module.default as Record<string, unknown>
        return { data: resource?.[namespace] ?? {} }
      } catch (error) {
        return {
          error: error instanceof Error ? error : new Error(String(error)),
        }
      }
    }

    // i18next's backend contract is callback-based; the promise rules do not apply.
    // eslint-disable-next-line promise/no-callback-in-promise, promise/catch-or-return
    load().then(({ data, error }) => callback(error ?? null, data ?? null))
  },
}
