import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import LegalDoc from './LegalDoc'
import { parseLegalDoc, slugifyHeading } from './legalMarkdown'

const MD = `# AIOJ Test Policy

**Last updated:** 2026-11-08

## 1. Scope

This policy covers the test instance.

## 2. Cookies and local storage

### 2.1 Essential cookies

Nothing here.

See [the terms](/legal/terms#4-your-content-and-intellectual-property).
`

function okResponse(body: string) {
    return {
        ok: true,
        status: 200,
        text: async () => body,
    } as unknown as Response
}

function renderDoc(slug: 'privacy_policy' = 'privacy_policy') {
    return render(
        <MemoryRouter>
            <LegalDoc slug={slug} title="Privacy Policy" />
        </MemoryRouter>
    )
}

describe('slugifyHeading', () => {
    it('matches the anchors the markdown links use', () => {
        expect(slugifyHeading('14. About the AIOJ software and self-hosted instances')).toBe(
            '14-about-the-aioj-software-and-self-hosted-instances'
        )
        expect(slugifyHeading('4. Your content and intellectual property')).toBe(
            '4-your-content-and-intellectual-property'
        )
        expect(slugifyHeading('3. Acceptable use')).toBe('3-acceptable-use')
    })

    it('strips inline markdown and punctuation', () => {
        expect(slugifyHeading('**Bold** and `code`')).toBe('bold-and-code')
    })
})

describe('parseLegalDoc', () => {
    it('lifts the title, metadata, and heading outline out of the body', () => {
        const doc = parseLegalDoc(MD)

        expect(doc.title).toBe('AIOJ Test Policy')
        expect(doc.updated).toBe('2026-11-08')
        expect(doc.body).not.toContain('**Last updated:**')
        expect(doc.body.startsWith('## 1. Scope')).toBe(true)
        expect(doc.headings).toEqual([
            { id: '1-scope', level: 2, text: '1. Scope' },
            { id: '2-cookies-and-local-storage', level: 2, text: '2. Cookies and local storage' },
            { id: '21-essential-cookies', level: 3, text: '2.1 Essential cookies' },
        ])
        expect(doc.minutes).toBeGreaterThanOrEqual(1)
    })
})

describe('LegalDoc', () => {
    beforeEach(() => {
        vi.stubGlobal('fetch', vi.fn(async () => okResponse(MD)))
    })

    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('renders the served document once, with metadata and anchored headings', async () => {
        renderDoc()

        expect(await screen.findByRole('heading', { level: 1, name: 'AIOJ Test Policy' })).toBeInTheDocument()
        // The markdown H1 is lifted into the page header, never rendered twice.
        expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
        expect(screen.getByText('2026-11-08')).toBeInTheDocument()
        expect(screen.getByRole('heading', { level: 2, name: '1. Scope' })).toHaveAttribute(
            'id',
            '1-scope'
        )
        expect(screen.getByRole('heading', { level: 3, name: '2.1 Essential cookies' })).toHaveAttribute(
            'id',
            '21-essential-cookies'
        )
    })

    it('lists every section in the table of contents with working anchors', async () => {
        renderDoc()
        await screen.findByRole('heading', { level: 2, name: '1. Scope' })

        // Desktop sidebar and the mobile disclosure each carry the outline.
        const scopeLinks = screen.getAllByRole('link', { name: '1. Scope' })
        expect(scopeLinks.length).toBeGreaterThanOrEqual(2)
        expect(scopeLinks[0]).toHaveAttribute('href', '#1-scope')
        expect(screen.getAllByRole('link', { name: '2.1 Essential cookies' })[0]).toHaveAttribute(
            'href',
            '#21-essential-cookies'
        )
    })

    it('turns cross-document markdown links into client-side routes', async () => {
        renderDoc()
        const link = await screen.findByRole('link', {
            name: 'the terms',
        })
        expect(link).toHaveAttribute('href', '/legal/terms#4-your-content-and-intellectual-property')
    })

    it('offers a retry when the document fails to load', async () => {
        const fetchMock = vi
            .fn()
            .mockResolvedValueOnce({ ok: false, status: 503, text: async () => '' })
            .mockResolvedValueOnce(okResponse(MD))
        vi.stubGlobal('fetch', fetchMock)

        renderDoc()

        expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load Privacy Policy')
        fireEvent.click(screen.getByRole('button', { name: /try again/i }))

        await waitFor(() =>
            expect(screen.getByRole('heading', { level: 2, name: '1. Scope' })).toBeInTheDocument()
        )
        expect(fetchMock).toHaveBeenCalledTimes(2)
    })
})
