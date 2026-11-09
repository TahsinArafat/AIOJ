import type { Components } from 'react-markdown'
import ReactMarkdown from 'react-markdown'
import rehypeKatex from 'rehype-katex'
import { richTextRemarkPlugins } from '../lib/markdown'

/**
 * Statement rendering shared by the practice page and the contest page.
 *
 * The two pages had drifted: the contest copy used its own heading scale,
 * wrapped every section in a bordered box, and styled inline code differently,
 * so the same problem looked like a different problem depending on whether it
 * was opened from the problem list or from a contest. Both now render through
 * this module.
 */

/** Body wrapper for the statement's prose. */
const STATEMENT_PROSE = 'prose prose-sm max-w-none text-ink text-[14px] leading-[1.6]'

const statementMarkdownComponents: Components = {
    code(props) {
        const { children, className } = props
        if (className?.includes('language-')) {
            return <code className={className}>{children}</code>
        }
        return <code className="bg-surface-3 px-1.5 py-0.5 rounded text-sm font-mono text-ink">{children}</code>
    },
}

/** The problem statement itself: markdown, GFM tables and KaTeX math. */
export function StatementBody({ body, className = STATEMENT_PROSE }: { body?: string | null; className?: string }) {
    if (!body) return null
    return (
        <div className={className}>
            <ReactMarkdown
                remarkPlugins={richTextRemarkPlugins}
                rehypePlugins={[rehypeKatex]}
                components={statementMarkdownComponents}
            >
                {body}
            </ReactMarkdown>
        </div>
    )
}

/** One statement sub-section: heading plus KaTeX/GFM-rendered body. */
export function StatementSection({ title, body }: { title: string; body: string }) {
    return (
        <section>
            <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted">{title}</h3>
            <StatementBody body={body} className="prose prose-sm max-w-none text-[14px] leading-[1.65] text-ink-soft" />
        </section>
    )
}
