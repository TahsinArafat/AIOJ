import { useState, useEffect, useRef } from 'react'
import katex from 'katex'
import { Button } from '../ui/Button'
import { Field } from '../ui/Field'

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
  { label: 'Square root', latex: '\\sqrt{x}' },
  { label: 'Sum', latex: '\\sum_{i=1}^{n}' },
  { label: 'Integral', latex: '\\int_{a}^{b}' },
  { label: 'Matrix', latex: '\\begin{pmatrix} a & b \\\\ c & d \\end{pmatrix}' },
]

function SymbolRow({ label, symbols, onPick }: { label: string; symbols: string[]; onPick: (s: string) => void }) {
  return (
    <div className="flex flex-wrap items-center gap-1">
      <span className="w-14 shrink-0 text-xs text-muted">{label}</span>
      {symbols.map(s => (
        <button
          key={s}
          type="button"
          onClick={() => onPick(s)}
          aria-label={`Insert ${s}`}
          className="rounded-md border border-hairline bg-surface px-2 py-1 text-sm text-ink-soft transition-colors hover:border-hairline-strong hover:bg-surface-2"
        >
          {s}
        </button>
      ))}
    </div>
  )
}

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
      queueMicrotask(() => setError(''))
    } catch (e) {
      // deferred a microtask: error state may not be set synchronously in an effect
      queueMicrotask(() => setError((e instanceof Error ? e.message : '') || 'Invalid LaTeX'))
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

  const canInsert = Boolean(latex) && !error

  return (
    <div
      // `bg-opacity-50` is a Tailwind v3 utility that no longer exists in v4,
      // so the scrim was fully transparent and the page showed through.
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="math-dialog-title"
        className="w-full max-w-2xl overflow-hidden rounded-xl border border-hairline bg-surface shadow-xl"
        onClick={e => e.stopPropagation()}
      >
        <div className="border-b border-hairline px-4 py-3">
          <h3 id="math-dialog-title" className="text-sm font-semibold text-ink">Insert math formula</h3>
        </div>

        <div className="max-h-[70vh] space-y-4 overflow-y-auto p-4">
          <Field label="LaTeX" htmlFor="math-latex">
            <textarea
              id="math-latex"
              ref={inputRef}
              value={latex}
              onChange={e => setLatex(e.target.value)}
              onKeyDown={handleKeyDown}
              rows={3}
              placeholder="e.g. \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}"
              className="w-full rounded-lg border border-hairline-strong bg-surface px-3 py-2 font-mono text-sm text-ink placeholder:text-muted-soft transition-colors hover:border-muted-soft focus:border-brand focus:outline-none"
            />
          </Field>

          <div>
            <p className="mb-1.5 text-xs font-medium text-ink-soft">Preview</p>
            <div
              ref={previewRef}
              className={`flex min-h-[60px] items-center justify-center rounded-lg border p-4 ${error ? 'border-bad/30 bg-bad/10' : 'border-hairline bg-surface-2'
                }`}
            />
            {error && (
              // `role="alert"` so the invalid-LaTeX message is announced rather
              // than only coloured red.
              <p role="alert" className="mt-1.5 text-xs text-bad">{error}</p>
            )}
          </div>

          <div className="space-y-2">
            <p className="text-xs font-medium text-ink-soft">Quick insert</p>
            <SymbolRow label="Greek" symbols={GREEK_LETTERS} onPick={insertSymbol} />
            <SymbolRow label="Operators" symbols={OPERATORS} onPick={insertSymbol} />
            <SymbolRow label="Arrows" symbols={ARROWS} onPick={insertSymbol} />
          </div>

          <div>
            <p className="mb-2 text-xs font-medium text-ink-soft">Templates</p>
            <div className="flex flex-wrap gap-2">
              {TEMPLATES.map(t => (
                <button
                  key={t.label}
                  type="button"
                  onClick={() => insertTemplate(t.latex)}
                  className="rounded-lg border border-hairline bg-surface px-3 py-1 text-sm text-ink-soft transition-colors hover:border-brand/40 hover:bg-brand-soft hover:text-brand"
                >
                  {t.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap justify-end gap-2 border-t border-hairline px-4 py-3">
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="secondary" onClick={() => onInsert(latex, false)} disabled={!canInsert}>
            Insert inline
          </Button>
          <Button onClick={() => onInsert(latex, true)} disabled={!canInsert}>
            Insert block
          </Button>
        </div>
      </div>
    </div>
  )
}
