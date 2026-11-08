import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import LegalDoc from './LegalDoc'
import { parseLegalDoc, slugifyHeading } from './legalMarkdown'

function renderDoc(slug: 'privacy_policy' | 'terms_of_service' | 'dmca', title: string) {
    return render(
        <MemoryRouter>
            <LegalDoc slug={slug} title={title} />
        </MemoryRouter>
    )
}

afterEach(() => vi.unstubAllGlobals())

describe('slugifyHeading', () => {
    it('matches the anchors the markdown links use', () => {
        expect(slugifyHeading('18. If you run this software for other people')).toBe(
            '18-if-you-run-this-software-for-other-people'
        )
        expect(slugifyHeading('14. About the AIOJ software and self-hosted instances')).toBe(
            '14-about-the-aioj-software-and-self-hosted-instances'
        )
        expect(slugifyHeading('3. Acceptable use')).toBe('3-acceptable-use')
    })

    it('strips inline markdown and punctuation', () => {
        expect(slugifyHeading('**Bold** and `code`')).toBe('bold-and-code')
    })
})

describe('parseLegalDoc', () => {
    const MD = `# AIOJ Test Policy

**Last updated:** 2026-11-08

## 1. Scope

Body.

### 1.1 Detail

More.
`

    it('lifts the title, metadata, and heading outline out of the markdown', () => {
        const doc = parseLegalDoc(MD)

        expect(doc.title).toBe('AIOJ Test Policy')
        expect(doc.updated).toBe('2026-11-08')
        expect(doc.body.startsWith('## 1. Scope')).toBe(true)
        expect(doc.headings).toEqual([
            { id: '1-scope', level: 2, text: '1. Scope' },
            { id: '11-detail', level: 3, text: '1.1 Detail' },
        ])
        expect(doc.minutes).toBeGreaterThanOrEqual(1)
    })
})

describe('LegalDoc', () => {
    it('renders the bundled policy with no network request', () => {
        const fetchMock = vi.fn()
        vi.stubGlobal('fetch', fetchMock)

        renderDoc('privacy_policy', 'Privacy Policy')

        expect(
            screen.getByRole('heading', { level: 1, name: 'AIOJ Privacy Policy' })
        ).toBeInTheDocument()
        expect(fetchMock).not.toHaveBeenCalled()
    })

    it('renders the document title once, with anchored section headings', () => {
        renderDoc('privacy_policy', 'Privacy Policy')

        // The markdown H1 becomes the page header; it is never rendered twice.
        expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
        expect(
            screen.getByRole('heading', { level: 2, name: '2. What we collect' })
        ).toHaveAttribute('id', '2-what-we-collect')
        expect(screen.getByText('2026-11-08')).toBeInTheDocument()
    })

    it('lists every section in the table of contents with working anchors', () => {
        renderDoc('privacy_policy', 'Privacy Policy')

        // Desktop sidebar and the mobile disclosure each carry the outline.
        const links = screen.getAllByRole('link', { name: '6. Cookies and local storage' })
        expect(links.length).toBeGreaterThanOrEqual(2)
        expect(links[0]).toHaveAttribute('href', '#6-cookies-and-local-storage')
    })

    it('routes cross-document markdown links through the client router', () => {
        renderDoc('privacy_policy', 'Privacy Policy')

        const terms = screen.getAllByRole('link', { name: 'Terms of Service' })[0]
        expect(terms).toHaveAttribute('href', '/legal/terms')
    })

    it('renders the other two documents too, not just the privacy policy', () => {
        const terms = renderDoc('terms_of_service', 'Terms of Service')
        expect(
            within(terms.container).getAllByRole('heading', { level: 1 })[0]
        ).toHaveTextContent('AIOJ Terms of Service')
        terms.unmount()

        renderDoc('dmca', 'DMCA / Copyright Policy')
        expect(
            screen.getByRole('heading', { level: 1, name: /DMCA \/ Copyright Policy/ })
        ).toBeInTheDocument()
    })

    it('does not show placeholder or draft wording', () => {
        for (const slug of ['privacy_policy', 'terms_of_service', 'dmca'] as const) {
            const { container, unmount } = renderDoc(slug, slug)
            expect(container.textContent).not.toMatch(/placeholder|lawyer-reviewed|lorem ipsum/i)
            unmount()
        }
    })
})
