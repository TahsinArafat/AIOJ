/**
 * Markdown plumbing for the legal pages.
 *
 * Kept out of `LegalDoc.tsx` so the page module exports only a component — the
 * Fast Refresh lint rule requires that — and so the parser can be unit-tested
 * without mounting React.
 */

import type { ReactNode } from 'react'

export type LegalSlug = 'terms_of_service' | 'privacy_policy' | 'dmca'

/** Every legal document served by `/api/legal/{slug}`, in navigation order. */
export const LEGAL_DOCS: { slug: LegalSlug; label: string; to: string }[] = [
    { slug: 'terms_of_service', label: 'Terms of Service', to: '/legal/terms' },
    { slug: 'privacy_policy', label: 'Privacy Policy', to: '/legal/privacy' },
    { slug: 'dmca', label: 'DMCA / Copyright Policy', to: '/legal/dmca' },
]

export interface Heading {
    id: string
    level: 2 | 3
    text: string
}

export interface ParsedDoc {
    title: string
    updated: string
    body: string
    headings: Heading[]
    /** Slugified heading text → element id, so render and TOC agree. */
    ids: Map<string, string>
    minutes: number
}

/**
 * GitHub-compatible heading slug.
 *
 * The cross-document links in the markdown are written with these anchors
 * (`/legal/terms#4-your-content-and-intellectual-property`), so the renderer has
 * to produce exactly the same ids or a deep link silently lands at the top.
 */
export function slugifyHeading(text: string): string {
    return text
        .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1') // links → their label
        .replace(/[`*_~]/g, '') // inline emphasis/code markers
        .toLowerCase()
        .replace(/[^\p{L}\p{N}\s-]/gu, '')
        .trim()
        .replace(/\s+/g, '-')
}

/** Flatten rendered heading children back to plain text. */
export function plainText(node: ReactNode): string {
    if (node === null || node === undefined || typeof node === 'boolean') return ''
    if (typeof node === 'string' || typeof node === 'number') return String(node)
    if (Array.isArray(node)) return node.map(plainText).join('')
    if (typeof node === 'object' && 'props' in node) {
        return plainText((node as { props?: { children?: ReactNode } }).props?.children)
    }
    return ''
}

/** Split the served markdown into header metadata, body, and a heading outline. */
export function parseLegalDoc(md: string): ParsedDoc {
    const lines = md.split('\n')
    const bodyLines: string[] = []
    const headings: Heading[] = []
    const ids = new Map<string, string>()
    const used = new Map<string, number>()

    let title = ''
    let updated = ''
    let inHeader = true
    let inFence = false

    for (const line of lines) {
        if (/^\s*(```|~~~)/.test(line)) inFence = !inFence

        if (inHeader && !inFence) {
            if (/^#\s+/.test(line)) {
                title = line.replace(/^#\s+/, '').trim()
                continue
            }
            const meta = line.match(/^\*\*([^*]+):\*\*\s*(.+)$/)
            if (meta) {
                if (/last updated|effective/i.test(meta[1])) updated = meta[2].trim()
                continue
            }
            if (/^#{2,}\s+/.test(line)) inHeader = false
            else if (line.trim() === '---') continue
        }

        bodyLines.push(line)

        if (!inFence) {
            const heading = line.match(/^(#{2,3})\s+(.+?)\s*$/)
            if (heading) {
                const level = heading[1].length as 2 | 3
                const text = heading[2]
                const key = slugifyHeading(text)
                const seen = used.get(key) ?? 0
                used.set(key, seen + 1)
                const id = seen === 0 ? key : `${key}-${seen}`
                headings.push({ id, level, text })
                ids.set(key, id)
            }
        }
    }

    const body = bodyLines.join('\n').trim()
    const words = body ? body.split(/\s+/).length : 0

    return {
        title,
        updated,
        body,
        headings,
        ids,
        minutes: Math.max(1, Math.round(words / 220)),
    }
}
