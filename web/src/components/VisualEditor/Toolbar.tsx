import { Editor } from '@tiptap/react'
import { useState, type ReactNode } from 'react'
import MathDialog from './MathDialog'

interface ToolbarProps {
  editor: Editor
}

/**
 * One button in the formatting toolbar.
 *
 * The active/inactive styling was repeated verbatim on all eleven buttons, so
 * a change to the "active" treatment had to be made eleven times and had
 * already drifted (`bg-blue-100 dark:bg-blue-900/30` had no token equivalent
 * and the math button had no active state at all). `aria-pressed` is what
 * actually tells assistive tech that Bold is on — the colour alone did not.
 */
function ToolButton({
  onClick,
  active = false,
  disabled = false,
  title,
  className = '',
  children,
}: {
  onClick: () => void
  active?: boolean
  disabled?: boolean
  title: string
  className?: string
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={title}
      aria-label={title}
      aria-pressed={active}
      className={`rounded px-2 py-1 text-sm transition-colors disabled:opacity-40 disabled:hover:bg-transparent ${
        active ? 'bg-brand-soft font-medium text-brand' : 'text-ink-soft hover:bg-surface-3'
      } ${className}`}
    >
      {children}
    </button>
  )
}

function Divider() {
  return <div className="mx-1 h-6 w-px self-center bg-hairline" aria-hidden="true" />
}

export default function Toolbar({ editor }: ToolbarProps) {
  const [showMathDialog, setShowMathDialog] = useState(false)

  const insertMath = (latex: string, displayMode: boolean) => {
    if (displayMode) {
      editor.chain().focus().insertContent(`$$${latex}$$`).run()
    } else {
      editor.chain().focus().insertContent(`$${latex}$`).run()
    }
    setShowMathDialog(false)
  }

  return (
    <>
      <div role="toolbar" aria-label="Formatting" className="flex flex-wrap items-center gap-1 border-b border-hairline bg-surface-2 p-2">
        <ToolButton
          onClick={() => editor.chain().focus().toggleBold().run()}
          active={editor.isActive('bold')}
          title="Bold"
          className="font-bold"
        >
          B
        </ToolButton>
        <ToolButton
          onClick={() => editor.chain().focus().toggleItalic().run()}
          active={editor.isActive('italic')}
          title="Italic"
          className="italic"
        >
          I
        </ToolButton>
        <ToolButton
          onClick={() => editor.chain().focus().toggleStrike().run()}
          active={editor.isActive('strike')}
          title="Strikethrough"
          className="line-through"
        >
          S
        </ToolButton>

        <Divider />

        {[1, 2, 3].map(level => (
          <ToolButton
            key={level}
            onClick={() => editor.chain().focus().toggleHeading({ level: level as 1 | 2 | 3 }).run()}
            active={editor.isActive('heading', { level })}
            title={`Heading ${level}`}
            className="font-bold"
          >
            H{level}
          </ToolButton>
        ))}

        <Divider />

        <ToolButton
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          active={editor.isActive('bulletList')}
          title="Bullet list"
        >
          •
        </ToolButton>
        <ToolButton
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          active={editor.isActive('orderedList')}
          title="Numbered list"
        >
          1.
        </ToolButton>

        <Divider />

        <ToolButton
          onClick={() => editor.chain().focus().toggleCode().run()}
          active={editor.isActive('code')}
          title="Inline code"
          className="font-mono"
        >
          {'</>'}
        </ToolButton>
        <ToolButton
          onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          active={editor.isActive('codeBlock')}
          title="Code block"
          className="font-mono"
        >
          {'{ }'}
        </ToolButton>

        <Divider />

        <ToolButton
          onClick={() => setShowMathDialog(true)}
          title="Insert math"
          className="font-bold text-brand"
        >
          ∑
        </ToolButton>

        <div className="flex-1" />

        <ToolButton
          onClick={() => editor.chain().focus().undo().run()}
          disabled={!editor.can().undo()}
          title="Undo"
        >
          ↶
        </ToolButton>
        <ToolButton
          onClick={() => editor.chain().focus().redo().run()}
          disabled={!editor.can().redo()}
          title="Redo"
        >
          ↷
        </ToolButton>
      </div>

      {showMathDialog && (
        <MathDialog
          onInsert={insertMath}
          onClose={() => setShowMathDialog(false)}
        />
      )}
    </>
  )
}
