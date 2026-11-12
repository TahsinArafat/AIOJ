import { useEffect, useRef } from 'react'
import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import Image from '@tiptap/extension-image'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableHeader } from '@tiptap/extension-table-header'
import { TableCell } from '@tiptap/extension-table-cell'
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
      // Images and tables are statement content the Markdown round-trip
      // supports, but without a node for them ProseMirror silently dropped the
      // element on parse. Inserting an image therefore appeared in the preview
      // pane and vanished from the editor, and the next keystroke saved a
      // statement with no image in it.
      Image.configure({
        // Statements link to uploaded media rather than embedding base64; the
        // markdown is stored and re-rendered, so the URL has to survive.
        inline: false,
        allowBase64: false,
      }),
      Table.configure({ resizable: false }),
      TableRow,
      TableHeader,
      TableCell,
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
