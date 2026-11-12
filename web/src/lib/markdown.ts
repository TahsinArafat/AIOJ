import { marked } from 'marked'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import TurndownService from 'turndown'
import remarkSoftBreaks from './remarkSoftBreaks'

interface MathToken {
    type: 'math'
    raw: string
    text: string
    displayMode: boolean
}

const escapeAttribute = (value: string) =>
    value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

/**
 * Math has to reach the editor as a real math node, not as `$...$` text: the
 * HTML-to-markdown pass escapes backslashes in text, so `\le` came back as
 * `\\le` and every saved bound rendered as "l e".
 */
const mathExtension = {
    name: 'math',
    level: 'inline' as const,
    start: (src: string) => src.search(/\$/),
    tokenizer(src: string): MathToken | undefined {
        const display = /^\$\$([\s\S]+?)\$\$/.exec(src)
        if (display) {
            return { type: 'math', raw: display[0], text: display[1], displayMode: true }
        }
        const inline = /^\$(?!\s)((?:\\\$|[^$\n])+?)(?<!\s)\$/.exec(src)
        if (inline) {
            return { type: 'math', raw: inline[0], text: inline[1], displayMode: false }
        }
    },
    renderer: (token: MathToken) =>
        `<math data-latex="${escapeAttribute(token.text)}" data-display-mode="${token.displayMode}"></math>`,
}

marked.use({ extensions: [mathExtension] })

/**
 * Markdown to HTML for seeding the WYSIWYG statement editor, which parses its
 * `content` as HTML: handing it markdown made every newline collapse into a
 * single line, and saving wrote that flattened text back.
 *
 * `breaks` keeps a lone newline as a real line break, which is how statements
 * are authored (one constraints bound per line).
 */
export const markdownToHtml = (markdown: string) =>
    marked.parse(markdown, { gfm: true, breaks: true, async: false })

/**
 * Remark plugins every statement, blog and editorial render shares. Soft breaks
 * run before math so the `$...$` chunks are still plain text and stay intact.
 */
export const richTextRemarkPlugins = [remarkGfm, remarkSoftBreaks, remarkMath]

// MathML keeps its lowercase name in the DOM, so comparing against `'MATH'`
// never matched a serialized math node.
const isMathNode = (node: { nodeName?: string; classList?: { contains: (name: string) => boolean } }) =>
    Boolean(node.classList?.contains('math-node')) || node.nodeName?.toLowerCase() === 'math'

/**
 * Turndown decides a node is "blank" when its textContent matches /^\s*$/, and
 * a blank node is rendered with blankReplacement BEFORE its children are
 * visited. A math node is `<math data-latex="x^2"></math>` — empty text by
 * design — so the math rule never ran and every formula was silently dropped.
 * Worse, the wrapping `<p>` is blank for the same reason, so even a paragraph
 * that held nothing but a formula collapsed to nothing.
 *
 * Seeding the element with a single character keeps both from reading as blank.
 * The math rule replaces the node wholesale, so the placeholder never reaches
 * the output.
 */
const seedMathText = (html: string) =>
    html.replace(/<math\b([^>]*)>\s*<\/math>/gi, '<math$1>x</math>')

const mathToMarkdown = (el: {
    getAttribute: (name: string) => string | null
    textContent?: string | null
    classList?: { contains: (name: string) => boolean }
}) => {
    const latex = el.getAttribute('data-latex') || el.textContent || ''
    const display = el.getAttribute('data-display-mode') === 'true' || el.classList?.contains('math-block')
    return display ? `$$${latex}$$` : `$${latex}$`
}

const turndownService = new TurndownService({
    headingStyle: 'atx',
    codeBlockStyle: 'fenced',
})

turndownService.addRule('math', {
    filter: isMathNode,
    replacement: (_content, node) =>
        mathToMarkdown(node as unknown as Parameters<typeof mathToMarkdown>[0]),
})

// Turndown writes `<br>` as a hard break (two trailing spaces), which would pad
// every saved line. A plain newline reads the same here — the renderer keeps
// single newlines — and leaves the stored statement unchanged on save.
turndownService.addRule('lineBreak', {
    filter: 'br',
    replacement: () => '\n',
})

/**
 * Images are uploaded media the statement links to, so the URL has to survive a
 * round trip verbatim. Turndown's built-in rule covers `![alt](src)` but drops
 * any title attribute, and the statement editor inserts alt text, so this keeps
 * all three parts.
 */
turndownService.addRule('image', {
    filter: 'img',
    replacement: (_content, node) => {
        const el = node as unknown as HTMLImageElement
        const src = el.getAttribute('src') || ''
        if (!src) return ''
        const alt = el.getAttribute('alt') || ''
        const title = el.getAttribute('title')
        return `![${alt}](${src}${title ? ` "${title}"` : ''})`
    },
})

/**
 * Tables. Turndown has no table rule, so a `<table>` collapsed into one
 * paragraph per cell — a problem statement with a table was destroyed the first
 * time anyone typed in the editor. GFM is what the renderer expects, so this
 * emits a pipe table, padding each row to the header's column count.
 */
turndownService.addRule('table', {
    filter: 'table',
    replacement: (_content, node) => {
        const table = node as unknown as HTMLTableElement
        const rows = Array.from(table.querySelectorAll('tr'))
        if (rows.length === 0) return ''

        const cellText = (cell: Element) => {
            // Math lives in an empty `<math data-latex>` node, so textContent is
            // blank for a cell holding only a formula. Read the same attribute
            // the math rule below emits, or `$n$` in a header is silently lost.
            const fromMath = Array.from(cell.querySelectorAll('math, .math-node'))
                .map(node => {
                    const latex = node.getAttribute('data-latex') || node.textContent || ''
                    const display =
                        node.getAttribute('data-display-mode') === 'true' ||
                        node.classList?.contains('math-block')
                    return display ? `$$${latex}$$` : `$${latex}$`
                })
                .join(' ')

            const text = (cell.textContent || '').trim()
            const inner = (fromMath || text).replace(/\|/g, '\\|')
            return inner.replace(/\s*\n\s*/g, ' ')
        }

        const grid = rows.map(row =>
            Array.from(row.querySelectorAll('th,td')).map(cellText),
        )
        const columns = grid.reduce((max, row) => Math.max(max, row.length), 1)
        const pad = (row: string[]) =>
            Array.from({ length: columns }, (_, i) => row[i] ?? '')

        const [header, ...body] = grid
        const lines = [
            `| ${pad(header).join(' | ')} |`,
            `| ${Array.from({ length: columns }, () => '---').join(' | ')} |`,
            ...body.map(row => `| ${pad(row).join(' | ')} |`),
        ]
        // Blank lines around the block so the renderer sees it as its own node
        // rather than folding it into the preceding paragraph.
        return `\n\n${lines.join('\n')}\n\n`
    },
})

/** The editor's serialized HTML back to the markdown that gets stored. */
export const htmlToMarkdown = (html: string) => turndownService.turndown(seedMathText(html))
