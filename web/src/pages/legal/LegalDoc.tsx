import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { ArrowUp, Clock3, Printer, RotateCw } from 'lucide-react'
import {
    LEGAL_DOCS,
    parseLegalDoc,
    plainText,
    slugifyHeading,
    type Heading,
    type LegalSlug,
} from './legalMarkdown'

function TableOfContents({
    headings,
    activeId,
    className = '',
}: {
    headings: Heading[]
    activeId: string
    className?: string
}) {
    if (headings.length === 0) return null
    return (
        <nav aria-label="On this page" className={className}>
            <p className="text-[11px] font-semibold uppercase tracking-widest text-muted">On this page</p>
            <ul className="mt-3 space-y-0.5 border-l border-hairline text-sm">
                {headings.map((h) => {
                    const active = h.id === activeId
                    return (
                        <li key={h.id}>
                            <a
                                href={`#${h.id}`}
                                aria-current={active ? 'location' : undefined}
                                className={[
                                    '-ml-px block border-l-2 py-1 pr-2 leading-snug transition-colors',
                                    h.level === 3 ? 'pl-6' : 'pl-3',
                                    active
                                        ? 'border-brand font-medium text-ink'
                                        : 'border-transparent text-ink-soft hover:border-hairline-strong hover:text-ink',
                                ].join(' ')}
                            >
                                {h.text}
                            </a>
                        </li>
                    )
                })}
            </ul>
        </nav>
    )
}

/** Shared shell for ToS / Privacy / DMCA pages. */
export default function LegalDoc({
    slug,
    title,
}: {
    slug: LegalSlug
    title: string
}) {
    const [attempt, setAttempt] = useState(0)
    const [activeId, setActiveId] = useState('')
    // One settled result per request, tagged with the request it answers. A
    // slug change or a retry therefore reads as "loading" without an effect
    // having to reset state synchronously.
    const [loaded, setLoaded] = useState<{ key: string; md?: string; err?: string }>({ key: '' })

    const requestKey = `${slug}:${attempt}`

    useEffect(() => {
        const controller = new AbortController()
        fetch(`/api/legal/${slug}`, { signal: controller.signal })
            .then((r) => {
                if (!r.ok) throw new Error(`HTTP ${r.status}`)
                return r.text()
            })
            .then((md) => {
                if (!controller.signal.aborted) setLoaded({ key: requestKey, md })
            })
            .catch((e: unknown) => {
                if (controller.signal.aborted) return
                setLoaded({
                    key: requestKey,
                    err: e instanceof Error ? e.message : 'failed to load',
                })
            })
        return () => controller.abort()
    }, [slug, requestKey])

    const settled = loaded.key === requestKey ? loaded : null
    const error = settled?.err ?? ''
    const markdown = settled?.md ?? null

    const doc = useMemo(() => (markdown === null ? null : parseLegalDoc(markdown)), [markdown])

    const retry = useCallback(() => setAttempt((n) => n + 1), [])

    // Deep links: the markdown arrives from a fetch, so the browser cannot
    // scroll to `#section` on its own. Do it once the body is on screen.
    useEffect(() => {
        if (!doc || !window.location.hash) return
        const id = decodeURIComponent(window.location.hash.slice(1))
        const target = document.getElementById(id)
        if (target) target.scrollIntoView({ block: 'start' })
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

    const components = useMemo<Components>(() => {
        const headingId = (children: ReactNode): string => {
            const key = slugifyHeading(plainText(children))
            return doc?.ids.get(key) ?? key
        }
        return {
            h2({ children }) {
                return (
                    <h2 id={headingId(children)} className="scroll-mt-24">
                        {children}
                    </h2>
                )
            },
            h3({ children }) {
                return (
                    <h3 id={headingId(children)} className="scroll-mt-24">
                        {children}
                    </h3>
                )
            },
            // Tables get their own scroll container: a wide table must never
            // make the whole page scroll sideways on a phone.
            table({ children }) {
                return (
                    <div className="my-6 w-full overflow-x-auto">
                        <table className="my-0">{children}</table>
                    </div>
                )
            },
            a({ href, children }) {
                if (href && href.startsWith('/')) {
                    return <Link to={href}>{children}</Link>
                }
                if (href && /^https?:\/\//.test(href)) {
                    return (
                        <a href={href} target="_blank" rel="noopener noreferrer">
                            {children}
                        </a>
                    )
                }
                return <a href={href}>{children}</a>
            },
        }
    }, [doc])

    if (error) {
        return (
            <div className="mx-auto mt-12 w-full max-w-3xl px-4">
                <div role="alert" className="rounded-lg border border-bad/30 bg-bad/10 p-6 text-bad">
                    <p className="font-medium">
                        Failed to load {title}: {error}
                    </p>
                    <p className="mt-1 text-sm text-ink-soft">
                        This document is served by the API at <code>/api/legal/{slug}</code>. Check
                        your connection and try again.
                    </p>
                    <button
                        type="button"
                        onClick={retry}
                        className="mt-4 inline-flex items-center gap-2 rounded border border-hairline-strong bg-surface px-3 py-1.5 text-sm text-ink hover:bg-surface-2"
                    >
                        <RotateCw className="h-4 w-4" aria-hidden="true" />
                        Try again
                    </button>
                </div>
            </div>
        )
    }

    if (!doc) {
        return (
            <div className="mx-auto mt-12 w-full max-w-3xl px-4" aria-busy="true">
                <div className="h-8 w-64 max-w-full animate-pulse rounded bg-surface-2" />
                <div className="mt-3 h-3 w-40 animate-pulse rounded bg-surface-2" />
                <div className="mt-8 space-y-3">
                    {[0, 1, 2, 3, 4, 5].map((i) => (
                        <div
                            key={i}
                            className="h-3 animate-pulse rounded bg-surface-2"
                            style={{ width: `${92 - i * 7}%` }}
                        />
                    ))}
                </div>
                <span className="sr-only">Loading {title}…</span>
            </div>
        )
    }

    return (
        <div className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">
            <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_15rem] lg:gap-14">
                <article className="min-w-0 max-w-3xl">
                    <nav aria-label="Legal documents" className="flex flex-wrap gap-2">
                        {LEGAL_DOCS.map((d) => {
                            const current = d.slug === slug
                            return (
                                <Link
                                    key={d.slug}
                                    to={d.to}
                                    aria-current={current ? 'page' : undefined}
                                    className={[
                                        'rounded-full border px-3 py-1 text-xs font-medium transition-colors',
                                        current
                                            ? 'border-brand bg-brand/10 text-brand'
                                            : 'border-hairline text-ink-soft hover:border-hairline-strong hover:text-ink',
                                    ].join(' ')}
                                >
                                    {d.label}
                                </Link>
                            )
                        })}
                    </nav>

                    <header className="mt-6 border-b border-hairline pb-6">
                        <p className="text-[11px] font-semibold uppercase tracking-widest text-muted">
                            Legal
                        </p>
                        <h1 className="mt-1 text-3xl font-bold break-words text-ink">
                            {doc.title || title}
                        </h1>
                        <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted">
                            {doc.updated && (
                                <span>
                                    Last updated{' '}
                                    <time dateTime={doc.updated} className="text-ink-soft">
                                        {doc.updated}
                                    </time>
                                </span>
                            )}
                            <span className="inline-flex items-center gap-1">
                                <Clock3 className="h-3.5 w-3.5" aria-hidden="true" />
                                {doc.minutes} min read
                            </span>
                            <button
                                type="button"
                                onClick={() => window.print()}
                                className="ml-auto inline-flex items-center gap-1.5 rounded border border-hairline px-2.5 py-1 text-xs text-ink-soft transition-colors hover:border-hairline-strong hover:text-ink print:hidden"
                            >
                                <Printer className="h-3.5 w-3.5" aria-hidden="true" />
                                Print
                            </button>
                        </div>
                    </header>

                    <details className="mt-6 rounded-lg border border-hairline bg-surface-2/40 p-4 print:hidden lg:hidden">
                        <summary className="cursor-pointer text-sm font-medium text-ink">
                            On this page
                        </summary>
                        <TableOfContents
                            headings={doc.headings}
                            activeId={activeId}
                            className="mt-3"
                        />
                    </details>

                    <div className="prose prose-sm mt-6 max-w-none break-words dark:prose-invert">
                        <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
                            {doc.body}
                        </ReactMarkdown>
                    </div>

                    <div className="mt-10 flex flex-wrap items-center justify-between gap-3 border-t border-hairline pt-6 text-xs text-muted print:hidden">
                        <p>
                            See also{' '}
                            {LEGAL_DOCS.filter((d) => d.slug !== slug).map((d, i, all) => (
                                <span key={d.slug}>
                                    <Link to={d.to} className="text-brand hover:underline">
                                        {d.label}
                                    </Link>
                                    {i < all.length - 1 ? (i === all.length - 2 ? ' and ' : ', ') : ''}
                                </span>
                            ))}
                            .
                        </p>
                        <button
                            type="button"
                            onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
                            className="inline-flex items-center gap-1.5 hover:text-ink"
                        >
                            <ArrowUp className="h-3.5 w-3.5" aria-hidden="true" />
                            Back to top
                        </button>
                    </div>
                </article>

                <aside className="hidden lg:block print:hidden">
                    <div className="sticky top-24 max-h-[calc(100vh-8rem)] overflow-y-auto pr-2">
                        <TableOfContents headings={doc.headings} activeId={activeId} />
                    </div>
                </aside>
            </div>
        </div>
    )
}
