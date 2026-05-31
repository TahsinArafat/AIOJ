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
