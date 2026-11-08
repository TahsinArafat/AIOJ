import { useMemo, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Clock3, Printer } from 'lucide-react'
import {
    LEGAL_DOCS,
    plainText,
    slugifyHeading,
    type Heading,
    type LegalSlug,
    type ParsedDoc,
} from './legalMarkdown'

/**
 * The legal article itself — everything inside the page, with no data loading.
 *
 * It is rendered twice: by `LegalDoc` in the running app, and by the build-time
 * prerenderer in `src/prerender/legalEntry.tsx`, which writes static HTML so
 * crawlers (and anyone with JavaScript disabled) see the full policy. That is
 * why every control has a non-JavaScript form:
 *
 *   - navigation uses `<Link>` in the app and a real `<a href>` in the static
 *     copy, so it works without the router;
 *   - "Back to top" is an anchor, not a click handler;
 *   - the print button is only rendered when `interactive`, because it would be
 *     a dead button in a static document.
 *
 * Both renderers feed it the same markdown and the same parser, so the text,
 * heading ids and cross-document links cannot drift apart.
 */

interface LegalArticleProps {
    doc: ParsedDoc
    slug: LegalSlug
    title: string
    /** False when rendering HTML with no JavaScript behind it. */
    interactive?: boolean
    /** Section currently in view; only meaningful when interactive. */
    activeId?: string
}

function DocLink({
    href,
    interactive,
    className,
    current,
    children,
}: {
    href: string
    interactive: boolean
    className?: string
    current?: boolean
    children: ReactNode
}) {
    const props = {
        className,
        'aria-current': current ? ('page' as const) : undefined,
    }
    if (interactive) {
        return (
            <Link to={href} {...props}>
                {children}
            </Link>
        )
    }
    return (
        <a href={href} {...props}>
            {children}
        </a>
    )
}

function TableOfContents({
    headings,
    activeId,
    interactive,
}: {
    headings: Heading[]
    activeId: string
    interactive: boolean
}) {
    if (headings.length === 0) return null
    return (
        <nav aria-label="On this page">
            <p className="text-[11px] font-semibold tracking-widest text-muted uppercase">
                On this page
            </p>
            <ul className="mt-3 space-y-0.5 border-l border-hairline text-sm">
                {headings.map((h) => {
                    const active = interactive && h.id === activeId
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

export default function LegalArticle({
    doc,
    slug,
    title,
    interactive = true,
    activeId = '',
}: LegalArticleProps) {
    const components = useMemo<Components>(() => {
        const headingId = (children: ReactNode): string => {
            const key = slugifyHeading(plainText(children))
            return doc.ids.get(key) ?? key
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
                if (!href) return <a>{children}</a>
                if (href.startsWith('/')) {
                    return <DocLink href={href} interactive={interactive}>{children}</DocLink>
                }
                if (/^https?:\/\//.test(href)) {
                    return (
                        <a href={href} target="_blank" rel="noopener noreferrer">
                            {children}
                        </a>
                    )
                }
                return <a href={href}>{children}</a>
            },
        }
    }, [doc, interactive])

    return (
        <div id="top" className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">
            <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_15rem] lg:gap-14">
                <article className="min-w-0 max-w-3xl">
                    <nav aria-label="Legal documents" className="flex flex-wrap gap-2">
                        {LEGAL_DOCS.map((d) => {
                            const current = d.slug === slug
                            return (
                                <DocLink
                                    key={d.slug}
                                    href={d.to}
                                    current={current}
                                    interactive={interactive}
                                    className={[
                                        'rounded-full border px-3 py-1 text-xs font-medium transition-colors',
                                        current
                                            ? 'border-brand bg-brand/10 text-brand'
                                            : 'border-hairline text-ink-soft hover:border-hairline-strong hover:text-ink',
                                    ].join(' ')}
                                >
                                    {d.label}
                                </DocLink>
                            )
                        })}
                    </nav>

                    <header className="mt-6 border-b border-hairline pb-6">
                        <p className="text-[11px] font-semibold tracking-widest text-muted uppercase">
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
                            {interactive && (
                                <button
                                    type="button"
                                    onClick={() => window.print()}
                                    className="ml-auto inline-flex items-center gap-1.5 rounded border border-hairline px-2.5 py-1 text-xs text-ink-soft transition-colors hover:border-hairline-strong hover:text-ink print:hidden"
                                >
                                    <Printer className="h-3.5 w-3.5" aria-hidden="true" />
                                    Print
                                </button>
                            )}
                        </div>
                    </header>

                    <details className="mt-6 rounded-lg border border-hairline bg-surface-2/40 p-4 print:hidden lg:hidden">
                        <summary className="cursor-pointer text-sm font-medium text-ink">
                            On this page
                        </summary>
                        <div className="mt-3">
                            <TableOfContents
                                headings={doc.headings}
                                activeId={activeId}
                                interactive={interactive}
                            />
                        </div>
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
                                    <DocLink
                                        href={d.to}
                                        interactive={interactive}
                                        className="text-brand hover:underline"
                                    >
                                        {d.label}
                                    </DocLink>
                                    {i < all.length - 1 ? (i === all.length - 2 ? ' and ' : ', ') : ''}
                                </span>
                            ))}
                            .
                        </p>
                        <a href="#top" className="inline-flex items-center gap-1.5 hover:text-ink">
                            Back to top
                        </a>
                    </div>
                </article>

                <aside className="hidden lg:block print:hidden">
                    <div className="sticky top-24 max-h-[calc(100vh-8rem)] overflow-y-auto pr-2">
                        <TableOfContents
                            headings={doc.headings}
                            activeId={activeId}
                            interactive={interactive}
                        />
                    </div>
                </aside>
            </div>
        </div>
    )
}
