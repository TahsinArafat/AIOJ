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

const turndownService = new TurndownService({
    headingStyle: 'atx',
    codeBlockStyle: 'fenced',
})

// MathML keeps its lowercase name in the DOM, so comparing against `'MATH'`
// never matched a serialized math node.
const isMathNode = (node: { nodeName?: string; classList?: { contains: (name: string) => boolean } }) =>
    Boolean(node.classList?.contains('math-node')) || node.nodeName?.toLowerCase() === 'math'

turndownService.addRule('math', {
    filter: isMathNode,
    replacement: (_content, node) => {
        const latex = node.getAttribute('data-latex') || node.textContent
        const displayMode =
            node.getAttribute('data-display-mode') === 'true' || node.classList?.contains('math-block')
        return displayMode ? `$$${latex}$$` : `$${latex}$`
    },
})

// Turndown writes `<br>` as a hard break (two trailing spaces), which would pad
// every saved line. A plain newline reads the same here — the renderer keeps
// single newlines — and leaves the stored statement unchanged on save.
turndownService.addRule('lineBreak', {
    filter: 'br',
    replacement: () => '\n',
})

/** The editor's serialized HTML back to the markdown that gets stored. */
export const htmlToMarkdown = (html: string) => turndownService.turndown(html)
