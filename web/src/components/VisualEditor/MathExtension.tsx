import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer, type ReactNodeViewProps } from '@tiptap/react'
import { NodeViewWrapper } from '@tiptap/react'
import katex from 'katex'
import { useEffect, useRef } from 'react'

const MathComponent = ({ node }: ReactNodeViewProps) => {
  const ref = useRef<HTMLSpanElement>(null)

  useEffect(() => {
    if (ref.current) {
      try {
        katex.render(node.attrs.latex || '', ref.current, {
          displayMode: node.attrs.displayMode,
          throwOnError: false,
          trust: true,
        })
      } catch {
        if (ref.current) {
          ref.current.textContent = node.attrs.latex
        }
      }
    }
  }, [node.attrs.latex, node.attrs.displayMode])

  return (
    <NodeViewWrapper
      className={`math-node ${node.attrs.displayMode ? 'math-block' : ''}`}
      as={node.attrs.displayMode ? 'div' : 'span'}
    >
      <span ref={ref} />
    </NodeViewWrapper>
  )
}

const MathExtension = Node.create({
  name: 'math',
  group: 'inline',
  inline: true,
  atom: true,

  addAttributes() {
    return {
      latex: {
        default: '',
        parseHTML: (element: HTMLElement) => element.getAttribute('data-latex') ?? '',
        renderHTML: (attributes: { latex: string }) => ({ 'data-latex': attributes.latex }),
      },
      displayMode: {
        default: false,
        parseHTML: (element: HTMLElement) => element.getAttribute('data-display-mode') === 'true',
        renderHTML: (attributes: { displayMode: boolean }) => ({
          'data-display-mode': attributes.displayMode ? 'true' : 'false',
        }),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'math' }]
  },

  renderHTML({ node, HTMLAttributes }) {
    // The TeX is kept as text content as well as an attribute: an element with
    // no text at all reads as blank to the HTML-to-markdown pass, which drops it
    // before any rule can turn it back into `$...$`.
    return ['math', mergeAttributes(HTMLAttributes), node.attrs.latex]
  },

  addNodeView() {
    return ReactNodeViewRenderer(MathComponent)
  },
})

export default MathExtension
