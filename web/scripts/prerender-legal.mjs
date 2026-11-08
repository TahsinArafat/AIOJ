import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

/**
 * Writes the prerendered legal pages into `dist/`.
 *
 * Run by `npm run build`, after the client build (which produces the stylesheet)
 * and the SSR build (which produces the renderer). The output is what nginx
 * serves for `/legal/privacy`, `/legal/terms` and `/legal/dmca`, so the policy
 * text is in the HTML body even when no JavaScript runs.
 */

const scriptDir = dirname(fileURLToPath(import.meta.url))
const webRoot = join(scriptDir, '..')
const distDir = join(webRoot, 'dist')
const ssrDir = join(webRoot, 'dist-ssr')

async function main() {
    const ssrFiles = await readdir(ssrDir)
    const entryName = ssrFiles.find((f) => f.endsWith('.js'))
    if (!entryName) {
        throw new Error(`prerender: no built entry in ${ssrDir}; run the SSR build first`)
    }

    const assetsDir = join(distDir, 'assets')
    const cssHrefs = (await readdir(assetsDir))
        .filter((f) => f.endsWith('.css'))
        .map((f) => `/assets/${f}`)
    if (cssHrefs.length === 0) {
        throw new Error(`prerender: no stylesheet found in ${assetsDir}`)
    }

    const origin = (process.env.PUBLIC_ORIGIN ?? '').trim().replace(/\/$/, '')
    const mod = await import(pathToFileURL(join(ssrDir, entryName)).href)
    const pages = mod.renderLegalPages({ cssHrefs, origin })

    for (const page of pages) {
        // `legal/privacy.html`, not `legal/privacy/index.html`: nginx's
        // `try_files $uri $uri.html` serves it at the exact URL, whereas a
        // directory triggers a 301 to the trailing-slash path. A redirect on
        // the URL a reviewer was given is avoidable noise.
        const rel = page.path.replace(/^\//, '')
        const outFile = join(distDir, `${rel}.html`)
        await mkdir(dirname(outFile), { recursive: true })
        await writeFile(outFile, page.html, 'utf8')
        console.log(
            `prerender: ${page.path} → ${rel}.html ` +
                `(${page.html.length} bytes, ~${page.words} words)`
        )
    }
}

main().catch((err) => {
    console.error(err)
    process.exitCode = 1
})
