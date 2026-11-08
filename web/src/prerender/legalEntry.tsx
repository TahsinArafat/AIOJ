import { renderToStaticMarkup } from 'react-dom/server'
import LegalArticle from '../pages/legal/LegalArticle'
import {
    LEGAL_DOCS,
    parseLegalDoc,
    type LegalSlug,
} from '../pages/legal/legalMarkdown'
import { legalMarkdown } from '../pages/legal/legalSources'

/**
 * Build-time prerenderer for the legal pages.
 *
 * Why this exists: the app is a client-rendered SPA, so `/legal/privacy` was
 * served as a 1.9 kB shell with an empty `<div id="root">`. Anything that does
 * not execute JavaScript — including the crawler behind app-store compliance
 * checks — saw a page with no policy text and rejected it as insufficient.
 *
 * `npm run build` runs this through a small SSR build and writes
 * `dist/legal/{privacy,terms,dmca}/index.html`. nginx serves those files
 * directly (`try_files $uri $uri/`), so the full text is in the response body
 * with no JavaScript involved. The in-app route still renders `LegalArticle`
 * for client-side navigation, from the same markdown.
 */

export interface StaticLegalPage {
    path: string
    slug: LegalSlug
    html: string
    /** Approximate visible word count, used by the sufficiency test. */
    words: number
}

interface RenderOptions {
    /** Built stylesheet URLs, e.g. `/assets/index-abc123.css`. */
    cssHrefs?: string[]
    /** Absolute site origin, used for canonical and Open Graph URLs. */
    origin?: string
}

const PATH_BY_SLUG: Record<LegalSlug, string> = {
    privacy_policy: '/legal/privacy',
    terms_of_service: '/legal/terms',
    dmca: '/legal/dmca',
}

/**
 * Written for search results and link previews. Kept short and specific:
 * "Privacy Policy" alone is the least useful possible description.
 */
const DESCRIPTION_BY_SLUG: Record<LegalSlug, string> = {
    privacy_policy:
        'What AIOJ collects, how it is used and shared, how long it is kept, and how to export or delete your account. No data selling, no ad tracking, no model training on your code.',
    terms_of_service:
        'The rules for using AIOJ: you keep your code, contest integrity, virtual judge, availability, liability, and termination.',
    dmca: 'How to report copyright infringement on AIOJ, send a counter-notice, and how repeat infringement is handled.',
}

function escapeHtml(value: string): string {
    return value
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
}

/** Visible-text word count: enough signal to catch an accidentally empty page. */
function countWords(markdown: string): number {
    const text = markdown
        .replace(/```[\s\S]*?```/g, ' ')
        .replace(/[#*_`>|~-]/g, ' ')
        .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    return text.split(/\s+/).filter((w) => /[a-zA-Z0-9]/.test(w)).length
}

function pageShell({
    title,
    description,
    canonical,
    cssHrefs,
    body,
}: {
    title: string
    description: string
    canonical: string
    cssHrefs: string[]
    body: string
}): string {
    const css = cssHrefs
        .map((href) => `    <link rel="stylesheet" href="${escapeHtml(href)}" />`)
        .join('\n')
    const canonicalTag = canonical
        ? `    <link rel="canonical" href="${escapeHtml(canonical)}" />\n` +
          `    <meta property="og:url" content="${escapeHtml(canonical)}" />\n`
        : ''

    return `<!doctype html>
<html lang="en">
<head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=5" />
    <title>${escapeHtml(title)}</title>
    <meta name="description" content="${escapeHtml(description)}" />
    <meta name="robots" content="index,follow" />
${canonicalTag}    <meta property="og:type" content="article" />
    <meta property="og:site_name" content="AIOJ" />
    <meta property="og:title" content="${escapeHtml(title)}" />
    <meta property="og:description" content="${escapeHtml(description)}" />
    <meta name="twitter:card" content="summary" />
    <meta name="color-scheme" content="light dark" />
    <meta name="theme-color" content="#2563eb" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <link rel="manifest" href="/manifest.json" />
    <link rel="apple-touch-icon" href="/icons/icon-192.png" />
${css}
    <script>
      (function () {
        try {
          var saved = localStorage.getItem('theme');
          var theme = saved || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
          if (theme === 'dark') document.documentElement.classList.add('dark');
        } catch (e) {}
      })();
    </script>
</head>
<body class="min-h-screen bg-canvas text-ink antialiased">
    <a href="#top" class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded focus:bg-surface focus:px-3 focus:py-2 focus:text-sm focus:text-ink">
        Skip to content
    </a>
    <header class="border-b border-hairline bg-surface">
        <div class="mx-auto flex w-full max-w-[1400px] flex-wrap items-center gap-x-5 gap-y-2 px-4 py-3 sm:px-6">
            <a href="/" class="font-semibold tracking-tight text-ink">AIOJ</a>
            <span aria-hidden="true" class="text-hairline-strong">/</span>
            <span class="text-xs text-muted">Self-hostable competitive programming</span>
            <nav aria-label="Primary" class="ml-auto flex flex-wrap items-center gap-4 text-sm text-ink-soft">
                <a href="/problems" class="hover:text-ink">Problems</a>
                <a href="/contests" class="hover:text-ink">Contests</a>
                <a href="/blog" class="hover:text-ink">Blog</a>
                <a href="/login" class="rounded border border-hairline-strong px-3 py-1.5 hover:bg-surface-2">Sign in</a>
            </nav>
        </div>
    </header>
    <main>
${body}
    </main>
    <footer class="border-t border-hairline bg-surface">
        <div class="mx-auto flex w-full max-w-[1400px] flex-col gap-3 px-4 py-5 text-xs text-muted sm:flex-row sm:items-center sm:justify-between sm:px-6">
            <div class="flex flex-wrap items-center gap-2">
                <span class="font-semibold tracking-tight text-ink">AIOJ</span>
                <span aria-hidden="true" class="text-hairline-strong">/</span>
                <span>Self-hostable competitive programming</span>
            </div>
            <nav aria-label="Legal" class="flex flex-wrap items-center gap-4">
${LEGAL_DOCS.map(
    (d) => `                <a href="${d.to}" class="transition-colors hover:text-ink">${escapeHtml(d.label)}</a>`
).join('\n')}
            </nav>
        </div>
    </footer>
</body>
</html>
`
}

/** Render every legal document to a standalone HTML page. */
export function renderLegalPages(options: RenderOptions = {}): StaticLegalPage[] {
    const { cssHrefs = [], origin = '' } = options
    const base = origin.replace(/\/$/, '')

    return LEGAL_DOCS.map(({ slug, label }) => {
        const markdown = legalMarkdown(slug)
        const doc = parseLegalDoc(markdown)
        if (!doc.title) {
            throw new Error(`prerender: legal document ${slug} is missing or has no title`)
        }

        const body = renderToStaticMarkup(
            <LegalArticle doc={doc} slug={slug} title={label} interactive={false} />
        )
        const path = PATH_BY_SLUG[slug]
        // `<title>` uses the short label: the H1 already begins with "AIOJ",
        // so `${doc.title} — AIOJ` would read "AIOJ Privacy Policy — AIOJ".
        const title = `${label} — AIOJ`
        const description = DESCRIPTION_BY_SLUG[slug]

        return {
            path,
            slug,
            words: countWords(markdown),
            html: pageShell({
                title,
                description,
                canonical: base ? `${base}${path}` : '',
                cssHrefs,
                body,
            }),
        }
    })
}
