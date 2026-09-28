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
/**
 * Test doubles for the browser APIs this module needs. The happy-dom window in
 * this runtime exposes neither a working Storage nor matchMedia, so both are
 * installed explicitly instead of being inherited from the environment.
 */
export function createMemoryStorage(): Storage {
  const entries = new Map<string, string>()
  return {
    get length() {
      return entries.size
    },
    clear() {
      entries.clear()
    },
    getItem(key: string) {
      const value = entries.get(key)
      return value === undefined ? null : value
    },
    key(index: number) {
      return [...entries.keys()][index] ?? null
    },
    removeItem(key: string) {
      entries.delete(key)
    },
    setItem(key: string, value: string) {
      entries.set(key, String(value))
    },
  }
}

export function installMemoryStorage(): Storage {
  const storage = createMemoryStorage()
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: storage,
  })
  return storage
}

/** Marks only the reduced-motion query as matching, so animations settle at once. */
export function installReducedMotion(): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: (query: string) => ({
      matches: query.includes('prefers-reduced-motion'),
      media: query,
      onchange: null,
      addListener() {
        return undefined
      },
      removeListener() {
        return undefined
      },
      addEventListener() {
        return undefined
      },
      removeEventListener() {
        return undefined
      },
      dispatchEvent() {
        return false
      },
    }),
  })
}
