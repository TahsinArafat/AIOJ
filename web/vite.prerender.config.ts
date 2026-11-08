import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

/**
 * SSR build used only to generate static HTML for the legal pages.
 *
 * `scripts/prerender-legal.mjs` imports the result and writes
 * `dist/legal/{privacy,terms,dmca}/index.html`. It runs after the normal client
 * build, so the emitted pages can link the real hashed stylesheet.
 *
 * This is a separate build rather than a dev-server `ssrLoadModule` call because
 * `react-dom/server` cannot be loaded through the module runner in this setup
 * ("require is not defined"); a real build externalises it correctly.
 */
export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: { '@': path.resolve(__dirname, './src') },
    },
    build: {
        ssr: 'src/prerender/legalEntry.tsx',
        outDir: 'dist-ssr',
        emptyOutDir: true,
        target: 'node22',
    },
})
