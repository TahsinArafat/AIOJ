import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
    plugins: [react()],
    test: {
        globals: true,
        environment: 'jsdom',
        setupFiles: './src/test/setup.ts',
        // `web/e2e/` holds Playwright specs. Playwright's `test` and vitest's
        // are different APIs with different fixtures, so vitest importing them
        // fails with a hard error ("Playwright Test did not expect test() to be
        // called here"). Excluding them keeps `npm test` reporting unit-test
        // results only; `npm run test:e2e` owns the browser specs.
        exclude: ['**/node_modules/**', '**/dist/**', '**/e2e/**'],
    },
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
        },
    },
})
