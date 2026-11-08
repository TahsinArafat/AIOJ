import { useEffect, useMemo, useState } from 'react'
import LegalArticle from './LegalArticle'
import { parseLegalDoc, type LegalSlug } from './legalMarkdown'
import { legalMarkdown } from './legalSources'

/**
 * The in-app legal route.
 *
 * The document is compiled into the bundle (see `legalSources.ts`) rather than
 * fetched, so this renders in the first paint and cannot be broken by an API
 * outage. The same markdown is prerendered to static HTML at build time for
 * crawlers — `src/prerender/legalEntry.tsx`.
 */
export default function LegalDoc({ slug, title }: { slug: LegalSlug; title: string }) {
    const md = useMemo(() => legalMarkdown(slug), [slug])
    const doc = useMemo(() => (md ? parseLegalDoc(md) : null), [md])
    const [activeId, setActiveId] = useState('')

    // Deep links: heading ids exist only after the markdown is parsed, so the
    // browser's own scroll-to-anchor cannot resolve them on a cold load.
    useEffect(() => {
        if (!doc || !window.location.hash) return
        const id = decodeURIComponent(window.location.hash.slice(1))
        document.getElementById(id)?.scrollIntoView({ block: 'start' })
    }, [doc])

    // Highlight the section being read.
    useEffect(() => {
        if (!doc || doc.headings.length === 0 || typeof IntersectionObserver === 'undefined') return
        const nodes = doc.headings
            .map((h) => document.getElementById(h.id))
            .filter((n): n is HTMLElement => n !== null)
        if (nodes.length === 0) return
        const observer = new IntersectionObserver(
            (entries) => {
                const visible = entries.filter((e) => e.isIntersecting)
                if (visible.length > 0) setActiveId(visible[0].target.id)
            },
            { rootMargin: '-96px 0px -70% 0px' }
        )
        nodes.forEach((n) => observer.observe(n))
        return () => observer.disconnect()
    }, [doc])

    if (!doc) {
        return (
            <div className="mx-auto mt-12 w-full max-w-3xl px-4">
                <div role="alert" className="rounded-lg border border-bad/30 bg-bad/10 p-6 text-bad">
                    <p className="font-medium">{title} is unavailable.</p>
                    <p className="mt-1 text-sm text-ink-soft">
                        The document source is missing from this build. Please contact the site
                        administrator.
                    </p>
                </div>
            </div>
        )
    }

    return <LegalArticle doc={doc} slug={slug} title={title} activeId={activeId} />
}
