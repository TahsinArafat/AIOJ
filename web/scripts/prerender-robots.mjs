import { readFile, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

/**
 * Fills `%SITE_ORIGIN%` in the emitted `dist/robots.txt`.
 *
 * Run by `npm run build`, after the client build has copied `web/public/` into
 * `dist/`. Vite copies `public/` verbatim, so without this the file shipped the
 * placeholder — and before that it shipped a hardcoded
 * `http://localhost:8081/sitemap.xml`, which told every crawler that reads
 * robots.txt to look for the sitemap on localhost. Search Console submissions
 * kept working, which is why nobody noticed.
 *
 * The Sitemap directive needs an absolute URL. When PUBLIC_ORIGIN is unset (a
 * local `npm run dev` build) the line is dropped instead of emitted as a
 * relative path, which the spec does not allow and crawlers ignore.
 */

const scriptDir = dirname(fileURLToPath(import.meta.url))
const webRoot = join(scriptDir, '..')
const robotsFile = join(webRoot, 'dist', 'robots.txt')

async function main() {
    const raw = await readFile(robotsFile, 'utf8')
    const origin = (process.env.PUBLIC_ORIGIN ?? '').trim().replace(/\/+$/, '')

    const filled = raw.replaceAll('%SITE_ORIGIN%', origin)
    const out = origin
        ? filled
        : filled
              .split('\n')
              .filter((line) => !line.startsWith('Sitemap:'))
              .join('\n')

    await writeFile(robotsFile, out, 'utf8')
    console.log(
        origin
            ? `prerender: robots.txt sitemap → ${origin}/sitemap.xml`
            : 'prerender: robots.txt sitemap line dropped (PUBLIC_ORIGIN unset)'
    )
}

main().catch((err) => {
    console.error(err)
    process.exitCode = 1
})