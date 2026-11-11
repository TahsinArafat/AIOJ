/**
 * Copy KaTeX's runtime into the built site so the server-rendered contest
 * booklet can typeset math without a CDN.
 *
 * Why this exists: the booklet HTML is produced by the Go API
 * (internal/pdf/generator.go) and served from the API origin. It loaded KaTeX
 * from jsdelivr, but the site's Content-Security-Policy is
 * `script-src 'self' blob:` — the CDN script was blocked outright, so
 * renderMathInElement never ran. Math stayed as literal `$...$`, and because the
 * print dialog was triggered from that same blocked script's onload, printing
 * never auto-opened either.
 *
 * Self-hosting removes both the blocked script AND the need to widen the CSP to
 * an external origin. nginx serves real files under dist/ before the SPA
 * fallback, so /katex/... resolves to these.
 */
import { cp, mkdir, rm } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const webRoot = resolve(here, '..')
const dist = resolve(webRoot, 'dist')
const src = resolve(webRoot, 'node_modules/katex/dist')
const dest = resolve(dist, 'katex')

if (!existsSync(src)) {
  console.error('copy-katex: katex is not installed; run npm ci first')
  process.exit(1)
}
if (!existsSync(dist)) {
  console.error('copy-katex: dist/ does not exist; run vite build first')
  process.exit(1)
}

await rm(dest, { recursive: true, force: true })
await mkdir(dest, { recursive: true })

// Only the runtime the booklet needs: the stylesheet, the renderer, the
// auto-render extension that converts $...$ in place, and the fonts the CSS
// references. Shipping the whole package would include source maps and the
// mjs/ESM builds nothing here uses.
await cp(resolve(src, 'katex.min.css'), resolve(dest, 'katex.min.css'))
await cp(resolve(src, 'katex.min.js'), resolve(dest, 'katex.min.js'))
await cp(resolve(src, 'contrib/auto-render.min.js'), resolve(dest, 'auto-render.min.js'))
await cp(resolve(src, 'fonts'), resolve(dest, 'fonts'), { recursive: true })

console.log('copy-katex: wrote dist/katex (css, js, auto-render, fonts)')
