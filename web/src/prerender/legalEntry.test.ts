import { describe, it, expect } from 'vitest'
import { renderLegalPages } from './legalEntry'

/**
 * These assertions are the regression gate for the app-store rejection.
 *
 * The legal pages used to be client-rendered only: the server returned a 1.9 kB
 * SPA shell with an empty `<div id="root">`, so a crawler that does not run
 * JavaScript saw a policy page with zero content and rejected the listing as
 * "does not have sufficient content". The static output is now generated at
 * build time, and these tests check the properties that made it fail: real text
 * in the body, a meaningful word count, the topics a reviewer looks for, and no
 * leftover draft wording.
 */

const pages = renderLegalPages({ cssHrefs: ['/assets/index-test.css'], origin: 'https://aioj0.cyou' })
const bySlug = Object.fromEntries(pages.map((p) => [p.slug, p]))

/** Visible text of the body, with tags stripped. */
function visibleText(html: string): string {
    return html
        .replace(/<script[\s\S]*?<\/script>/g, ' ')
        .replace(/<style[\s\S]*?<\/style>/g, ' ')
        .replace(/<[^>]+>/g, ' ')
        .replace(/\s+/g, ' ')
        .trim()
}

describe('prerendered legal pages', () => {
    it('emits one page per document at the documented route', () => {
        expect(pages.map((p) => p.path)).toEqual([
            '/legal/terms',
            '/legal/privacy',
            '/legal/dmca',
        ])
    })

    it('serves the article as real text, not an empty app shell', () => {
        for (const page of pages) {
            expect(page.html.startsWith('<!doctype html>')).toBe(true)
            // The failure mode being guarded: nothing but <div id="root">.
            expect(page.html).not.toContain('id="root"')
            expect(visibleText(page.html).length).toBeGreaterThan(4000)
            expect(page.words, `${page.path} word count`).toBeGreaterThan(800)
        }
    })

    it('has the metadata a crawler and a link preview need', () => {
        for (const page of pages) {
            expect(page.html).toMatch(/<title>[^<]+ — AIOJ<\/title>/)
            expect(page.html).toMatch(/<meta name="description" content="[^"]{60,}"/)
            expect(page.html).toMatch(/<meta name="robots" content="index,follow"/)
            expect(page.html).toContain(
                `<link rel="canonical" href="https://aioj0.cyou${page.path}" />`
            )
            expect(page.html).toContain('<link rel="stylesheet" href="/assets/index-test.css" />')
            expect(page.html).toContain('<meta property="og:title"')
        }
    })

    it('does not duplicate the site name in the title', () => {
        expect(bySlug.privacy_policy.html).toContain('<title>Privacy Policy — AIOJ</title>')
        expect(bySlug.terms_of_service.html).toContain('<title>Terms of Service — AIOJ</title>')
    })

    it('renders one h1 and an anchored h2 outline', () => {
        for (const page of pages) {
            expect(page.html.match(/<h1[\s>]/g) ?? []).toHaveLength(1)
            const h2s = page.html.match(/<h2 id="[^"]+"/g) ?? []
            expect(h2s.length, `${page.path} h2 count`).toBeGreaterThan(5)
        }
        // Anchor ids must be the same ones the in-app table of contents links to.
        expect(bySlug.privacy_policy.html).toContain('id="2-what-we-collect"')
        expect(bySlug.privacy_policy.html).toContain('href="#2-what-we-collect"')
    })

    it('omits controls that need JavaScript', () => {
        // A dead <button> in a static document is worse than no button.
        for (const page of pages) {
            expect(page.html).not.toContain('<button')
        }
        // The only script is the theme bootstrap.
        expect(bySlug.privacy_policy.html.match(/<script/g) ?? []).toHaveLength(1)
    })

    it('covers the topics an app-store privacy review checks for', () => {
        const text = visibleText(bySlug.privacy_policy.html)
        const required = [
            'What we collect',
            'Why we use your data',
            'Device permissions',
            'When data leaves this Instance',
            'How long we keep data',
            'How we protect it',
            'Security incidents',
            'Your rights and choices',
            'Deleting your account',
            'Children',
            'Contact',
            'export',
            'delete your account',
            'bcrypt',
            'no third-party cookies',
        ]
        for (const topic of required) {
            expect(text.toLowerCase(), `privacy policy should cover: ${topic}`).toContain(
                topic.toLowerCase()
            )
        }
    })

    it('never ship draft or placeholder wording', () => {
        for (const page of pages) {
            expect(page.html).not.toMatch(/placeholder|lawyer-reviewed|lorem ipsum|TODO/i)
        }
    })

    it('omits canonical and og:url when no origin is configured', () => {
        const local = renderLegalPages({ cssHrefs: ['/a.css'] })
        for (const page of local) {
            expect(page.html).not.toContain('rel="canonical"')
            expect(page.html).not.toContain('og:url')
        }
    })
})
