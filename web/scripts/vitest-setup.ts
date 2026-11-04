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

import { Window } from 'happy-dom'
import { afterEach } from 'vitest'

/**
 * Node 25 ships a built-in `globalThis.localStorage` that is an empty shell:
 * the property exists, so Vitest's happy-dom environment skips installing
 * its own, but the shell has no `setItem`/`clear`/`getItem` methods. Zustand's
 * persist middleware and direct `localStorage.clear()` calls then crash.
 *
 * Replacing the global with a real happy-dom `Storage` restores the behavior
 * every test in the suite expects.
 */
function installWorkingStorage() {
  const probe = (globalThis as Record<string, unknown>).localStorage as
    | { setItem?: unknown }
    | undefined
  if (probe && typeof probe.setItem === 'function') return

  const window = new Window()
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    enumerable: true,
    value: window.localStorage,
  })
  Object.defineProperty(globalThis, 'sessionStorage', {
    configurable: true,
    enumerable: true,
    value: window.sessionStorage,
  })
}

installWorkingStorage()

afterEach(() => {
  globalThis.localStorage.clear()
  globalThis.sessionStorage.clear()
})
