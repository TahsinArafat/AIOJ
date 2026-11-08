import { describe, it, expect } from 'vitest'
import { parseLegalDoc } from './legalMarkdown'

/**
 * The legal documents are hand-written markdown that cross-reference each other
 * with anchors (`/legal/terms#4-your-content-and-intellectual-property`). Those
 * links are invisible to TypeScript and to the router, so a renamed heading
 * would silently drop readers at the top of a page. This contract test reads the
 * real served documents and fails when an anchor no longer exists.
 *
 * The markdown is pulled through `import.meta.glob` rather than `node:fs`, so
 * this file typechecks under the browser tsconfig, which has no `@types/node`
 * — the same approach as `lib/languages.test.ts`.
 */
const sources = import.meta.glob('../../../../internal/api/handler/legal/*.md', {
    query: '?raw',
    import: 'default',
    eager: true,
}) as Record<string, string>

/** `/…/legal/privacy_policy.md` → `privacy` (the slug the links use). */
const SLUG_BY_FILE: Record<string, string> = {
    'privacy_policy.md': 'privacy',
    'terms_of_service.md': 'terms',
    'dmca.md': 'dmca',
}

const parsed = Object.fromEntries(
    Object.entries(sources).map(([path, md]) => {
        const file = path.replace(/^.*\//, '')
        const slug = SLUG_BY_FILE[file] ?? file.replace(/\.md$/, '')
        return [slug, { md, doc: parseLegalDoc(md) }]
    })
)

/** Markdown links to `/legal/<doc>#<anchor>` or to `#<anchor>` in the same doc. */
function anchors(md: string): { target: string; anchor: string }[] {
    const out: { target: string; anchor: string }[] = []
    const cross = /\]\(\/legal\/(terms|privacy|dmca)#([^)\s]+)\)/g
    const local = /\]\(#([^)\s]+)\)/g
    for (const m of md.matchAll(cross)) out.push({ target: m[1], anchor: m[2] })
    for (const m of md.matchAll(local)) out.push({ target: '', anchor: m[1] })
    return out
}

describe('legal document cross-references', () => {
    it('found all three served documents with a title and update date', () => {
        expect(Object.keys(parsed).sort()).toEqual(['dmca', 'privacy', 'terms'])
        for (const [slug, { doc }] of Object.entries(parsed)) {
            expect(doc.title, `${slug} title`).toMatch(/AIOJ/)
            expect(doc.updated, `${slug} last-updated`).toMatch(/^\d{4}-\d{2}-\d{2}$/)
            expect(doc.headings.length, `${slug} headings`).toBeGreaterThan(5)
        }
    })

    it('resolves every internal anchor to a real heading', () => {
        for (const [slug, { md, doc }] of Object.entries(parsed)) {
            const own = new Set(doc.headings.map((h) => h.id))
            for (const link of anchors(md)) {
                const targetSlug = link.target || slug
                const target = parsed[targetSlug]
                expect(target, `${slug} links to unknown document ${targetSlug}`).toBeDefined()
                const ids = new Set(target.doc.headings.map((h) => h.id))
                expect(
                    ids.has(link.anchor),
                    `${slug} → ${targetSlug}#${link.anchor} does not match a heading`
                ).toBe(true)
                if (!link.target) expect(own.has(link.anchor)).toBe(true)
            }
        }
    })

    it('keeps heading ids unique so deep links land on one section', () => {
        for (const [slug, { doc }] of Object.entries(parsed)) {
            const ids = doc.headings.map((h) => h.id)
            expect(new Set(ids).size, `${slug} has duplicate heading ids`).toBe(ids.length)
        }
    })
})
