// Test environment preload: install happy-dom as the global DOM for bun test
import { Window } from 'happy-dom'

const window = new Window()
const globalShell = globalThis as unknown as Record<string, unknown>

for (const key of Object.getOwnPropertyNames(window)) {
  if (globalShell[key] === undefined) {
    try {
      globalShell[key] = (window as unknown as Record<string, unknown>)[key]
    } catch {
      // skip non-configurable properties
    }
  }
}
globalShell.window = window
globalShell.document = window.document
