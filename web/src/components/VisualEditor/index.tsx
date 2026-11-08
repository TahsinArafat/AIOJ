import { useEffect, useRef } from 'react'
import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import Toolbar from './Toolbar'
import MathExtension from './MathExtension'
import { htmlToMarkdown, markdownToHtml } from '../../lib/markdown'
import './styles.css'

const lowlight = createLowlight(common)

interface VisualEditorProps {
  content: string
  onChange: (markdown: string) => void
  placeholder?: string
}

export default function VisualEditor({ content, onChange, placeholder }: VisualEditorProps) {
  // Last markdown this editor produced. Anything else arriving through
  // `content` was written from outside — an inserted image, an applied AI
  // draft — and has to be re-seeded instead of being ignored.
  const emitted = useRef(content)

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
    // The editor parses `content` as HTML, so markdown has to be converted
    // first — otherwise every newline collapses and the next save persists the
    // flattened statement.
    content: markdownToHtml(content),
    onUpdate: ({ editor }) => {
      const html = editor.getHTML()
      const markdown = htmlToMarkdown(html)
      emitted.current = markdown
      onChange(markdown)
    },
  })

  useEffect(() => {
    if (!editor || content === emitted.current) return
    emitted.current = content
    editor.commands.setContent(markdownToHtml(content), { emitUpdate: false })
  }, [content, editor])

  if (!editor) return null

  return (
    <div className="border border-hairline-strong rounded-lg overflow-hidden">
      <Toolbar editor={editor} />
      <EditorContent editor={editor} className="prose prose-sm dark:prose-invert max-w-none p-4 min-h-[200px]" />
    </div>
  )
}
