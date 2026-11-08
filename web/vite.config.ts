import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

/**
 * Fills `%SITE_ORIGIN%` in index.html from PUBLIC_ORIGIN.
 *
 * Open Graph image URLs have to be absolute — a relative one is ignored by
 * Telegram, Slack and X — but the origin is deployment configuration, not
 * source. `web/Dockerfile` already passes PUBLIC_ORIGIN through as the same
 * value the legal-page prerender bakes into its canonical tags, so this is one
 * source of truth for both.
 *
 * Runs before Vite's own `%ENV%` substitution: an undefined key there is left
 * as literal text, which would ship `content="%SITE_ORIGIN%/og-default.png"`.
 * When the variable is unset (a local `npm run dev`) the placeholder is
 * replaced with an empty string, leaving a root-relative path that still
 * resolves in dev.
 */
function siteOriginPlugin(): Plugin {
    return {
        name: 'aioj-site-origin',
        transformIndexHtml: {
            order: 'pre',
            handler(html) {
                const origin = (process.env.PUBLIC_ORIGIN ?? '').trim().replace(/\/+$/, '')
                return html.replaceAll('%SITE_ORIGIN%', origin)
            },
        },
    }
}

export default defineConfig({
    plugins: [siteOriginPlugin(), react(), tailwindcss()],
    resolve: {
        alias: { '@': path.resolve(__dirname, './src') },
    },
    build: {
        // The setter's VisualEditor (tiptap + prosemirror + lowlight) is the
        // one chunk that still crosses 500 kB. Shard its vendor stack into
        // separate on-demand chunks so no single file trips the size warning.
        rolldownOptions: {
            output: {
                codeSplitting: {
                    groups: [
                        { name: 'editor-highlight', test: /node_modules[\\/](highlight\.js|lowlight)[\\/]/ },
                        { name: 'editor-tiptap', test: /node_modules[\\/](@tiptap|prosemirror)[\\/]/ },
                        { name: 'editor-turndown', test: /node_modules[\\/](turndown|@mixmark-io)[\\/]/ },
                    ],
                },
            },
        },
    },
    server: {
        // `src/pages/legal/legalSources.ts` globs the served markdown from
        // `internal/api/handler/legal/`, which is outside this package root.
        // Without this the dev server refuses those reads.
        fs: { allow: [path.resolve(__dirname, '..')] },
        proxy: {
            '/api': 'http://localhost:8080',
            '/ws': { target: 'ws://localhost:8080', ws: true },
        },
    },
})
