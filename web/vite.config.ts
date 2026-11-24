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

/**
 * Drops KaTeX's `.woff` and `.ttf` fallbacks from its `@font-face` rules.
 *
 * Every face ships three times in one `src` list — woff2, then woff, then ttf —
 * and every browser this app targets (it ships ESM bundles) picks the first
 * entry, woff2. The other two were still emitted into `dist/assets`, so the
 * image carried 876 kB of fonts no request could ever ask for. Rewriting the
 * rules before Vite resolves the `url()`s means those files are never emitted
 * at all, which also keeps the emitted filenames' hashes honest — stripping
 * them from `dist/` after the build would leave CSS whose hash no longer
 * matched its content, and `immutable` caching turns that into a real bug.
 *
 * `scripts/copy-katex.mjs` applies the same rule to the booklet's own copy.
 */
function katexWoff2Only(): Plugin {
    return {
        name: 'aioj-katex-woff2-only',
        enforce: 'pre',
        transform(code, id) {
            if (!id.endsWith('.css') || !id.includes('/katex/')) return null
            return code.replace(/,\s*url\([^)]*\.(?:woff|ttf)\)\s*format\((["'])(?:woff|truetype)\1\)/g, '')
        },
    }
}

export default defineConfig({
    plugins: [siteOriginPlugin(), katexWoff2Only(), react(), tailwindcss()],
    resolve: {
        alias: { '@': path.resolve(__dirname, './src') },
    },
    build: {
        // The setter's VisualEditor (tiptap + prosemirror + lowlight) must stay
        // out of the public critical path, so its vendor stack is pinned to
        // named chunks that only the setter's lazy routes import.
        //
        // `priority` on the React group is load-bearing, not cosmetic. Without
        // it rolldown merged react, react-dom and use-sync-external-store into
        // the `editor-tiptap` group (they are reached through @tiptap/react),
        // which made the entry chunk import React *from* that chunk — and a
        // static import ships the whole chunk, so every visitor to the public
        // site downloaded 440 kB of tiptap + prosemirror that no public route
        // can reach. A higher-priority React group keeps React in a chunk the
        // entry genuinely needs and leaves the editor groups unimported.
        rolldownOptions: {
            output: {
                codeSplitting: {
                    groups: [
                        {
                            name: 'react',
                            test: /node_modules[\\/](react|react-dom|scheduler|use-sync-external-store)[\\/]/,
                            priority: 20,
                        },
                        { name: 'editor-highlight', test: /node_modules[\\/](highlight\.js|lowlight)[\\/]/, priority: 10 },
                        { name: 'editor-tiptap', test: /node_modules[\\/](@tiptap|prosemirror)[\\/]/, priority: 10 },
                        { name: 'editor-turndown', test: /node_modules[\\/](turndown|@mixmark-io)[\\/]/, priority: 10 },
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
