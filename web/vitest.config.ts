import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { defineConfig } from 'vitest/config'

const dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(dirname, './src'),
    },
  },
  test: {
    environment: 'happy-dom',
    setupFiles: ['./scripts/vitest-setup.ts'],
    // Component suites mount large trees (charts, editors). Under full-suite
    // parallelism the default 5s budget is not enough on CI runners.
    testTimeout: 15000,
    server: {
      deps: {
        // `@lobehub/ui` re-exports `@lobehub/fluent-emoji` with a directory
        // import (`from "./FluentEmoji"`). Vite's ESM resolver rejects that
        // form, so both packages must be transformed instead of externalized.
        inline: ['@lobehub/ui', '@lobehub/fluent-emoji'],
      },
    },
  },
})
