import type { Root } from 'mdast'

interface MdNode {
    type: string
    value?: string
    children?: MdNode[]
}

/**
 * Markdown folds a single newline into a space. Problem statements are written
 * line per line — a constraints block is one bound per line — so a lone newline
 * has to stay a line break instead of joining the block into one long line.
 *
 * Only `text` nodes are touched: code blocks and inline code keep their content
 * in `value` without children, so their whitespace survives untouched.
 */
export default function remarkSoftBreaks() {
    return (tree: Root) => {
        const root = tree as unknown as MdNode
        root.children = splitSoftBreaks(root.children ?? [])
    }
}

function splitSoftBreaks(nodes: MdNode[]): MdNode[] {
    return nodes.flatMap(node => {
        if (node.type === 'text') return splitLines(node as MdNode & { value: string })
        if (!node.children) return [node]
        return [{ ...node, children: splitSoftBreaks(node.children) }]
    })
}

function splitLines(node: MdNode & { value: string }): MdNode[] {
    const lines = node.value.split('\n')
    if (lines.length === 1) return [node]
    return lines.flatMap((line, i) => [
        ...(i > 0 ? [{ type: 'break' }] : []),
        ...(line === '' ? [] : [{ type: 'text', value: line }]),
    ])
}
