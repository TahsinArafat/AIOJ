import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import Toolbar from './Toolbar'
import MathExtension from './MathExtension'
import TurndownService from 'turndown'
import './styles.css'

const lowlight = createLowlight(common)

const turndownService = new TurndownService({
  headingStyle: 'atx',
  codeBlockStyle: 'fenced',
})

turndownService.addRule('math', {
  filter: (node) => {
    return node.classList?.contains('math-node') || node.nodeName === 'MATH'
  },
  replacement: (_content, node) => {
    const latex = node.getAttribute('data-latex') || node.textContent
    const displayMode = node.classList?.contains('math-block')
    return displayMode ? `$$${latex}$$` : `$${latex}$`
  },
})

function htmlToMarkdown(html: string): string {
  return turndownService.turndown(html)
}

interface VisualEditorProps {
  content: string
  onChange: (markdown: string) => void
  placeholder?: string
}

export default function VisualEditor({ content, onChange, placeholder }: VisualEditorProps) {
  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        codeBlock: false,
      }),
      Placeholder.configure({
        placeholder: placeholder || 'Start writing...',
      }),
      CodeBlockLowlight.configure({
        lowlight,
      }),
      MathExtension,
    ],
    content: content,
    onUpdate: ({ editor }) => {
      const html = editor.getHTML()
      const markdown = htmlToMarkdown(html)
      onChange(markdown)
    },
  })

  if (!editor) return null

  return (
    <div className="border border-gray-300 dark:border-gray-600 rounded-lg overflow-hidden">
      <Toolbar editor={editor} />
      <EditorContent editor={editor} className="prose prose-sm dark:prose-invert max-w-none p-4 min-h-[200px]" />
    </div>
  )
}
