# Visual WYSIWYG Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace plain textareas in the problem setter workspace with a TipTap WYSIWYG editor featuring a visual math insertion dialog.

**Architecture:** TipTap (ProseMirror-based) editor with custom math extension, toolbar with formatting buttons, and a math dialog with live KaTeX preview. HTML output is converted to Markdown for storage.

**Tech Stack:** React, TipTap, KaTeX, turndown (HTML→Markdown)

---

## File Structure

```
web/src/
├── components/
│   └── VisualEditor/
│       ├── index.tsx              # Main VisualEditor component
│       ├── Toolbar.tsx            # Editor toolbar
│       ├── MathDialog.tsx         # Math insertion dialog
│       └── MathExtension.ts       # Custom TipTap math node
├── pages/
│   ├── SetterTestPage.tsx         # Example page at /setter/test1
│   └── SetterProblemWorkspace.tsx # (minor modifications)
└── components/
    └── SetterWorkspace/
        └── StatementTab.tsx       # Replace textareas with VisualEditor

docs/superpowers/
└── specs/
    └── 2026-05-31-visual-editor-design.md  # Design spec (already created)
```

---

### Task 1: Install Dependencies

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install TipTap and related packages**

```bash
cd web
npm install @tiptap/react @tiptap/starter-kit @tiptap/extension-mathematics @tiptap/extension-code-block-lowlight @tiptap/extension-placeholder katex lowlight turndown
npm install -D @types/turndown
```

- [ ] **Step 2: Verify installation**

```bash
npm list @tiptap/react @tiptap/starter-kit @tiptap/extension-mathematics
```

Expected: All packages listed with versions

- [ ] **Step 3: Commit**

```bash
git add package.json package-lock.json
git commit -m "deps: add TipTap editor and math dependencies"
```

---

### Task 2: Create VisualEditor Component

**Files:**
- Create: `web/src/components/VisualEditor/index.tsx`

- [ ] **Step 1: Create VisualEditor component with basic TipTap setup**

```tsx
// web/src/components/VisualEditor/index.tsx
import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import Toolbar from './Toolbar'
import MathExtension from './MathExtension'
import './styles.css'

const lowlight = createLowlight(common)

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
      // Convert HTML to markdown and call onChange
      const html = editor.getHTML()
      const markdown = htmlToMarkdown(html)
      onChange(markdown)
    },
  })

  if (!editor) return null

  return (
    <div className="border border-gray-300 rounded-lg overflow-hidden">
      <Toolbar editor={editor} />
      <EditorContent editor={editor} className="prose prose-sm max-w-none p-4 min-h-[200px]" />
    </div>
  )
}

// Placeholder - will be implemented in Task 5
function htmlToMarkdown(html: string): string {
  return html
}
```

- [ ] **Step 2: Create styles file**

```css
/* web/src/components/VisualEditor/styles.css */
.tiptap {
  outline: none;
}

.tiptap p.is-editor-empty:first-child::before {
  color: #adb5bd;
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
}

.tiptap .math-node {
  display: inline-block;
  cursor: pointer;
  padding: 0 4px;
  border-radius: 4px;
  background: #f0f7ff;
  border: 1px solid #cce5ff;
}

.tiptap .math-node:hover {
  background: #cce5ff;
}

.tiptap .math-block {
  display: block;
  text-align: center;
  margin: 1em 0;
  padding: 8px;
}

.ProseMirror-selectednode .math-node {
  outline: 2px solid #2563eb;
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/VisualEditor/index.tsx web/src/components/VisualEditor/styles.css
git commit -m "feat: create VisualEditor component with TipTap setup"
```

---

### Task 3: Create Toolbar Component

**Files:**
- Create: `web/src/components/VisualEditor/Toolbar.tsx`

- [ ] **Step 1: Create Toolbar component**

```tsx
// web/src/components/VisualEditor/Toolbar.tsx
import { Editor } from '@tiptap/react'
import { useState } from 'react'
import MathDialog from './MathDialog'

interface ToolbarProps {
  editor: Editor
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
      <div className="flex flex-wrap gap-1 p-2 border-b border-gray-200 bg-gray-50">
        <button
          onClick={() => editor.chain().focus().toggleBold().run()}
          className={`px-2 py-1 rounded text-sm font-bold ${editor.isActive('bold') ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Bold"
        >
          B
        </button>
        <button
          onClick={() => editor.chain().focus().toggleItalic().run()}
          className={`px-2 py-1 rounded text-sm italic ${editor.isActive('italic') ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Italic"
        >
          I
        </button>
        <button
          onClick={() => editor.chain().focus().toggleStrike().run()}
          className={`px-2 py-1 rounded text-sm line-through ${editor.isActive('strike') ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Strikethrough"
        >
          S
        </button>

        <div className="w-px h-6 bg-gray-300 mx-1 self-center" />

        <button
          onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
          className={`px-2 py-1 rounded text-sm font-bold ${editor.isActive('heading', { level: 1 }) ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Heading 1"
        >
          H1
        </button>
        <button
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          className={`px-2 py-1 rounded text-sm font-bold ${editor.isActive('heading', { level: 2 }) ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Heading 2"
        >
          H2
        </button>
        <button
          onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
          className={`px-2 py-1 rounded text-sm font-bold ${editor.isActive('heading', { level: 3 }) ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Heading 3"
        >
          H3
        </button>

        <div className="w-px h-6 bg-gray-300 mx-1 self-center" />

        <button
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          className={`px-2 py-1 rounded text-sm ${editor.isActive('bulletList') ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Bullet List"
        >
          •
        </button>
        <button
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          className={`px-2 py-1 rounded text-sm ${editor.isActive('orderedList') ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Numbered List"
        >
          1.
        </button>

        <div className="w-px h-6 bg-gray-300 mx-1 self-center" />

        <button
          onClick={() => editor.chain().focus().toggleCode().run()}
          className={`px-2 py-1 rounded text-sm font-mono ${editor.isActive('code') ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Inline Code"
        >
          {'</>'}
        </button>
        <button
          onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          className={`px-2 py-1 rounded text-sm font-mono ${editor.isActive('codeBlock') ? 'bg-blue-100 text-blue-700' : 'hover:bg-gray-200'}`}
          title="Code Block"
        >
          {'{ }'}
        </button>

        <div className="w-px h-6 bg-gray-300 mx-1 self-center" />

        <button
          onClick={() => setShowMathDialog(true)}
          className="px-2 py-1 rounded text-sm font-bold hover:bg-gray-200 text-blue-600"
          title="Insert Math"
        >
          ∑
        </button>

        <div className="flex-1" />

        <button
          onClick={() => editor.chain().focus().undo().run()}
          disabled={!editor.can().undo()}
          className="px-2 py-1 rounded text-sm hover:bg-gray-200 disabled:opacity-50"
          title="Undo"
        >
          ↶
        </button>
        <button
          onClick={() => editor.chain().focus().redo().run()}
          disabled={!editor.can().redo()}
          className="px-2 py-1 rounded text-sm hover:bg-gray-200 disabled:opacity-50"
          title="Redo"
        >
          ↷
        </button>
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
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/VisualEditor/Toolbar.tsx
git commit -m "feat: create editor toolbar with formatting buttons"
```

---

### Task 4: Create MathDialog Component

**Files:**
- Create: `web/src/components/VisualEditor/MathDialog.tsx`

- [ ] **Step 1: Create MathDialog component**

```tsx
// web/src/components/VisualEditor/MathDialog.tsx
import { useState, useEffect, useRef } from 'react'
import katex from 'katex'

interface MathDialogProps {
  onInsert: (latex: string, displayMode: boolean) => void
  onClose: () => void
}

const GREEK_LETTERS = ['α', 'β', 'γ', 'δ', 'ε', 'ζ', 'η', 'θ', 'ι', 'κ', 'λ', 'μ', 'ν', 'ξ', 'π', 'ρ', 'σ', 'τ', 'υ', 'φ', 'χ', 'ψ', 'ω']
const OPERATORS = ['∑', '∫', '√', '∞', '±', '×', '÷', '≠', '≈', '≤', '≥']
const ARROWS = ['→', '←', '↑', '↓', '⇒', '⇐']
const SYMBOLS_MAP: Record<string, string> = {
  'α': '\\alpha', 'β': '\\beta', 'γ': '\\gamma', 'δ': '\\delta',
  'ε': '\\epsilon', 'ζ': '\\zeta', 'η': '\\eta', 'θ': '\\theta',
  'ι': '\\iota', 'κ': '\\kappa', 'λ': '\\lambda', 'μ': '\\mu',
  'ν': '\\nu', 'ξ': '\\xi', 'π': '\\pi', 'ρ': '\\rho',
  'σ': '\\sigma', 'τ': '\\tau', 'υ': '\\upsilon', 'φ': '\\phi',
  'χ': '\\chi', 'ψ': '\\psi', 'ω': '\\omega',
  '∑': '\\sum', '∫': '\\int', '√': '\\sqrt{}', '∞': '\\infty',
  '±': '\\pm', '×': '\\times', '÷': '\\div', '≠': '\\neq',
  '≈': '\\approx', '≤': '\\leq', '≥': '\\geq',
  '→': '\\rightarrow', '←': '\\leftarrow', '↑': '\\uparrow',
  '↓': '\\downarrow', '⇒': '\\Rightarrow', '⇐': '\\Leftarrow',
}

const TEMPLATES = [
  { label: 'Fraction', latex: '\\frac{a}{b}' },
  { label: 'Square Root', latex: '\\sqrt{x}' },
  { label: 'Sum', latex: '\\sum_{i=1}^{n}' },
  { label: 'Integral', latex: '\\int_{a}^{b}' },
  { label: 'Matrix', latex: '\\begin{pmatrix} a & b \\\\ c & d \\end{pmatrix}' },
]

export default function MathDialog({ onInsert, onClose }: MathDialogProps) {
  const [latex, setLatex] = useState('')
  const [error, setError] = useState('')
  const previewRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  useEffect(() => {
    if (!previewRef.current) return
    try {
      katex.render(latex || '\\text{Preview}', previewRef.current, {
        displayMode: true,
        throwOnError: true,
        trust: true,
      })
      setError('')
    } catch (e: any) {
      setError(e.message || 'Invalid LaTeX')
    }
  }, [latex])

  const insertSymbol = (symbol: string) => {
    const latexCmd = SYMBOLS_MAP[symbol] || symbol
    const textarea = inputRef.current
    if (textarea) {
      const start = textarea.selectionStart
      const end = textarea.selectionEnd
      const newLatex = latex.substring(0, start) + latexCmd + latex.substring(end)
      setLatex(newLatex)
      setTimeout(() => {
        textarea.focus()
        textarea.setSelectionRange(start + latexCmd.length, start + latexCmd.length)
      }, 0)
    } else {
      setLatex(prev => prev + latexCmd)
    }
  }

  const insertTemplate = (template: string) => {
    setLatex(template)
    inputRef.current?.focus()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (latex && !error) {
        onInsert(latex, false)
      }
    }
    if (e.key === 'Escape') {
      onClose()
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4" onClick={e => e.stopPropagation()}>
        <div className="p-4 border-b">
          <h3 className="text-lg font-semibold">Insert Math Formula</h3>
        </div>

        <div className="p-4 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">LaTeX Input</label>
            <textarea
              ref={inputRef}
              value={latex}
              onChange={e => setLatex(e.target.value)}
              onKeyDown={handleKeyDown}
              className="w-full border border-gray-300 rounded px-3 py-2 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              rows={3}
              placeholder="e.g., \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Preview</label>
            <div
              ref={previewRef}
              className={`border rounded p-4 min-h-[60px] flex items-center justify-center ${error ? 'border-red-300 bg-red-50' : 'border-gray-200 bg-gray-50'}`}
            />
            {error && <p className="text-red-500 text-xs mt-1">{error}</p>}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Quick Insert</label>
            <div className="space-y-2">
              <div className="flex flex-wrap gap-1">
                <span className="text-xs text-gray-500 w-12">Greek:</span>
                {GREEK_LETTERS.map(l => (
                  <button key={l} onClick={() => insertSymbol(l)} className="px-2 py-1 text-sm border rounded hover:bg-gray-100">{l}</button>
                ))}
              </div>
              <div className="flex flex-wrap gap-1">
                <span className="text-xs text-gray-500 w-12">Ops:</span>
                {OPERATORS.map(o => (
                  <button key={o} onClick={() => insertSymbol(o)} className="px-2 py-1 text-sm border rounded hover:bg-gray-100">{o}</button>
                ))}
              </div>
              <div className="flex flex-wrap gap-1">
                <span className="text-xs text-gray-500 w-12">Arrows:</span>
                {ARROWS.map(a => (
                  <button key={a} onClick={() => insertSymbol(a)} className="px-2 py-1 text-sm border rounded hover:bg-gray-100">{a}</button>
                ))}
              </div>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Templates</label>
            <div className="flex flex-wrap gap-2">
              {TEMPLATES.map(t => (
                <button
                  key={t.label}
                  onClick={() => insertTemplate(t.latex)}
                  className="px-3 py-1 text-sm border rounded hover:bg-blue-50 hover:border-blue-300"
                >
                  {t.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="p-4 border-t flex justify-end gap-2">
          <button onClick={onClose} className="px-4 py-2 text-sm border rounded hover:bg-gray-50">
            Cancel
          </button>
          <button
            onClick={() => onInsert(latex, false)}
            disabled={!latex || !!error}
            className="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
          >
            Insert Inline
          </button>
          <button
            onClick={() => onInsert(latex, true)}
            disabled={!latex || !!error}
            className="px-4 py-2 text-sm bg-blue-700 text-white rounded hover:bg-blue-800 disabled:opacity-50"
          >
            Insert Block
          </button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/VisualEditor/MathDialog.tsx
git commit -m "feat: create math dialog with LaTeX input and preview"
```

---

### Task 5: Create MathExtension for TipTap

**Files:**
- Create: `web/src/components/VisualEditor/MathExtension.ts`

- [ ] **Step 1: Create custom math extension**

```typescript
// web/src/components/VisualEditor/MathExtension.ts
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
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/VisualEditor/MathExtension.ts
git commit -m "feat: create TipTap math extension with KaTeX rendering"
```

---

### Task 6: Implement HTML to Markdown Conversion

**Files:**
- Modify: `web/src/components/VisualEditor/index.tsx`

- [ ] **Step 1: Add turndown and implement conversion**

```tsx
// Add to top of web/src/components/VisualEditor/index.tsx
import TurndownService from 'turndown'

const turndownService = new TurndownService({
  headingStyle: 'atx',
  codeBlockStyle: 'fenced',
})

// Custom rule for math
turndownService.addRule('math', {
  filter: (node) => {
    return node.classList?.contains('math-node') || node.nodeName === 'MATH'
  },
  replacement: (content, node: any) => {
    const latex = node.getAttribute('data-latex') || node.textContent
    const displayMode = node.classList?.contains('math-block')
    return displayMode ? `$$${latex}$$` : `$${latex}$`
  },
})

function htmlToMarkdown(html: string): string {
  return turndownService.turndown(html)
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/VisualEditor/index.tsx
git commit -m "feat: implement HTML to Markdown conversion with math support"
```

---

### Task 7: Integrate VisualEditor into StatementTab

**Files:**
- Modify: `web/src/components/SetterWorkspace/StatementTab.tsx`

- [ ] **Step 1: Import VisualEditor**

Add import at top:
```tsx
import VisualEditor from '../VisualEditor'
```

- [ ] **Step 2: Replace description textarea with VisualEditor**

Find the description textarea (around line 94-101) and replace with:
```tsx
<div>
  <label className="block text-xs font-medium text-gray-600 mb-1">Description (Markdown + LaTeX)</label>
  <VisualEditor
    content={formState.description}
    onChange={(markdown) => onUpdate('description', markdown)}
    placeholder="Write your problem statement..."
  />
</div>
```

- [ ] **Step 3: Replace inputFormat textarea with VisualEditor**

Find the inputFormat textarea and replace with:
```tsx
<div>
  <label className="block text-xs font-medium text-gray-600 mb-1">Input Format</label>
  <VisualEditor
    content={formState.inputFormat}
    onChange={(markdown) => onUpdate('inputFormat', markdown)}
    placeholder="Describe the input format..."
  />
</div>
```

- [ ] **Step 4: Replace outputFormat textarea with VisualEditor**

Find the outputFormat textarea and replace with:
```tsx
<div>
  <label className="block text-xs font-medium text-gray-600 mb-1">Output Format</label>
  <VisualEditor
    content={formState.outputFormat}
    onChange={(markdown) => onUpdate('outputFormat', markdown)}
    placeholder="Describe the output format..."
  />
</div>
```

- [ ] **Step 5: Replace hint textarea with VisualEditor**

Find the hint textarea and replace with:
```tsx
<div>
  <label className="block text-xs font-medium text-gray-600 mb-1">Hint (optional)</label>
  <VisualEditor
    content={formState.hint}
    onChange={(markdown) => onUpdate('hint', markdown)}
    placeholder="Add a hint..."
  />
</div>
```

- [ ] **Step 6: Commit**

```bash
git add web/src/components/SetterWorkspace/StatementTab.tsx
git commit -m "feat: integrate VisualEditor into StatementTab"
```

---

### Task 8: Create Test Page

**Files:**
- Create: `web/src/pages/SetterTestPage.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create SetterTestPage component**

```tsx
// web/src/pages/SetterTestPage.tsx
import { useState } from 'react'
import VisualEditor from '../components/VisualEditor'

const SAMPLE_MARKDOWN = `# Weird Algorithm

Consider an algorithm that takes as input a positive integer $n$. If $n$ is even, the algorithm divides it by two, and if $n$ is odd, the algorithm multiplies it by three and adds one. The algorithm repeats this, until $n$ is one. For example, the sequence for $n=3$ is as follows:

$$3 \\rightarrow 10 \\rightarrow 5 \\rightarrow 16 \\rightarrow 8 \\rightarrow 4 \\rightarrow 2 \\rightarrow 1$$

Your task is to simulate the execution of the algorithm for a given value of $n$.

## Input

The only input line contains an integer $n$.

## Output

Print a line that contains all values of $n$ during the algorithm.

## Constraints

- $1 \\le n \\le 10^6$

## Example

**Input:**
\`\`\`
3
\`\`\`

**Output:**
\`\`\`
3 10 5 16 8 4 2 1
\`\`\`
`

export default function SetterTestPage() {
  const [markdown, setMarkdown] = useState(SAMPLE_MARKDOWN)
  const [showMarkdown, setShowMarkdown] = useState(false)

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-4xl mx-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold">Visual Editor Test Page</h1>
          <div className="flex gap-2">
            <button
              onClick={() => setShowMarkdown(!showMarkdown)}
              className="px-4 py-2 text-sm border rounded hover:bg-gray-100"
            >
              {showMarkdown ? 'Hide' : 'Show'} Markdown
            </button>
            <button
              onClick={() => {
                navigator.clipboard.writeText(markdown)
                alert('Markdown copied!')
              }}
              className="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700"
            >
              Copy Markdown
            </button>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4">Editor</h2>
          <VisualEditor
            content={markdown}
            onChange={setMarkdown}
            placeholder="Start writing your problem statement..."
          />
        </div>

        {showMarkdown && (
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold mb-4">Generated Markdown</h2>
            <pre className="bg-gray-100 rounded p-4 text-sm font-mono overflow-auto max-h-[400px]">
              {markdown}
            </pre>
          </div>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Add route to App.tsx**

Find the setter routes section and add:
```tsx
<Route path="/setter/test1" element={<SetterTestPage />} />
```

Add import:
```tsx
import SetterTestPage from './pages/SetterTestPage'
```

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/SetterTestPage.tsx web/src/App.tsx
git commit -m "feat: add test page at /setter/test1"
```

---

### Task 9: Test and Verify

- [ ] **Step 1: Run TypeScript check**

```bash
cd web && npx tsc --noEmit
```

Expected: No type errors

- [ ] **Step 2: Run dev server**

```bash
cd web && npm run dev
```

Expected: Server starts without errors

- [ ] **Step 3: Test in browser**

1. Open http://localhost:5173/setter/test1
2. Verify toolbar buttons work (bold, italic, headings, lists, code)
3. Click ∑ button to open math dialog
4. Type LaTeX formula, verify preview renders
5. Click "Insert Inline" or "Insert Block"
6. Verify math appears in editor
7. Click "Show Markdown" to verify output format
8. Click "Copy Markdown" to verify clipboard

- [ ] **Step 4: Test in real workspace**

1. Open http://localhost:5173/setter/create or an existing problem
2. Verify VisualEditor appears for all text fields
3. Type content, save, verify it persists
4. View problem on public page, verify math renders

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete visual editor with math support"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Install dependencies | package.json |
| 2 | Create VisualEditor component | VisualEditor/index.tsx, styles.css |
| 3 | Create Toolbar component | VisualEditor/Toolbar.tsx |
| 4 | Create MathDialog component | VisualEditor/MathDialog.tsx |
| 5 | Create MathExtension | VisualEditor/MathExtension.ts |
| 6 | Implement HTML→Markdown | VisualEditor/index.tsx |
| 7 | Integrate into StatementTab | StatementTab.tsx |
| 8 | Create test page | SetterTestPage.tsx, App.tsx |
| 9 | Test and verify | - |
