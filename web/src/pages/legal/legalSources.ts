import type { LegalSlug } from './legalMarkdown'

/**
 * The legal documents, compiled into the app.
 *
 * The same markdown files are embedded by the Go handler and served at
 * `GET /api/legal/{slug}`. Reading them here instead of fetching that endpoint
 * means the page works with the API down, renders in the first paint instead of
 * showing a skeleton, and — through the prerenderer in `src/prerender` — is
 * emitted as static HTML that a crawler can read without running JavaScript.
 *
 * `import.meta.glob` is resolved at build time, so a missing file is a build
 * error rather than a blank page. `legalLinks.test.ts` also asserts that every
 * slug resolves and that cross-document anchors still exist.
 */
const sources = import.meta.glob('../../../../internal/api/handler/legal/*.md', {
    query: '?raw',
    import: 'default',
    eager: true,
}) as Record<string, string>

const FILE_BY_SLUG: Record<LegalSlug, string> = {
    terms_of_service: 'terms_of_service.md',
    privacy_policy: 'privacy_policy.md',
    dmca: 'dmca.md',
}

/**
 * Raw markdown for one document, or an empty string if the file is missing.
 *
 * Empty rather than throwing: a render-time throw in the routed page would take
 * out the whole app shell, while the page can show a clear message for one
 * document. The unit tests fail loudly if a document disappears.
 */
export function legalMarkdown(slug: LegalSlug): string {
    const file = FILE_BY_SLUG[slug]
    for (const [path, body] of Object.entries(sources)) {
        if (path.endsWith(`/${file}`)) return body
    }
    return ''
}
