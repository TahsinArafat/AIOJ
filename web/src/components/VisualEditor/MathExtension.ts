import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { NodeViewWrapper } from '@tiptap/react'
import katex from 'katex'
import { useEffect, useRef } from 'react'

const MathComponent = ({ node, updateAttributes }: any) => {
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
      latex: { default: '' },
      displayMode: { default: false },
    }
  },

  parseHTML() {
    return [{ tag: 'math' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['math', mergeAttributes(HTMLAttributes)]
  },

  addNodeView() {
    return ReactNodeViewRenderer(MathComponent)
  },
})

export default MathExtension
