import ReactMarkdown, { type Components } from 'react-markdown'
import remarkMath from 'remark-math'
import remarkGfm from 'remark-gfm'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'

interface MarkdownContentProps {
    /** Markdown source. `$...$` / `$$...$$` chunks are rendered as KaTeX math. */
    content?: string | null
    className?: string
    /**
     * Render without a wrapping paragraph. Use for short single-line fields
     * (complexity, approach blurbs) so the output stays inline in its container.
     */
    inline?: boolean
}

const inlineComponents: Components = {
    // ReactMarkdown always wraps text in a paragraph; flatten it for inline use.
    p: ({ children }) => <>{children}</>,
}

/**
 * Single source of truth for rendering user-written rich text. Problem
 * statements, blogs and editorials all use this so Markdown + LaTeX behave
 * identically everywhere.
 */
export default function MarkdownContent({ content, className, inline = false }: MarkdownContentProps) {
    if (!content) return null

    const markdown = (
        <ReactMarkdown
            remarkPlugins={[remarkGfm, remarkMath]}
            rehypePlugins={[rehypeKatex]}
            components={inline ? inlineComponents : undefined}
        >
            {content}
        </ReactMarkdown>
    )

    return inline ? <span className={className}>{markdown}</span> : <div className={className}>{markdown}</div>
}
