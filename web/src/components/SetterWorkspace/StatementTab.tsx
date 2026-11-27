import { useRef } from 'react'
import RichTextField, { RichTextPreview } from '../RichTextField'
import type { ProblemFormState } from '../../types/problem-workspace'

interface StatementTabProps {
  formState: ProblemFormState
  saving: boolean
  uploadingImage: boolean
  onUpdate: <K extends keyof ProblemFormState>(key: K, value: ProblemFormState[K]) => void
  onImageUpload: (file: File) => Promise<void>
  onSave: (e: React.FormEvent) => void
}

export default function StatementTab({
  formState,
  saving,
  uploadingImage,
  onUpdate,
  onImageUpload,
  onSave,
}: StatementTabProps) {
  const imageInputRef = useRef<HTMLInputElement>(null)

  const handleImageChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) return
    const file = e.target.files[0]
    await onImageUpload(file)
    if (imageInputRef.current) imageInputRef.current.value = ''
  }

  return (
    <form onSubmit={onSave} className="space-y-4">
      <h2 className="text-lg font-bold text-ink border-b pb-2 mb-4">Edit Problem Statement</h2>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Title</label>
          <input aria-label="Title"
            type="text"
            value={formState.title}
            onChange={e => onUpdate('title', e.target.value)}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Tags (comma-separated)</label>
          <input aria-label="Tags (comma-separated)"
            type="text"
            value={formState.tags}
            onChange={e => onUpdate('tags', e.target.value)}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          />
        </div>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Difficulty</label>
          <select aria-label="Difficulty"
            value={formState.difficulty}
            onChange={e => onUpdate('difficulty', e.target.value)}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          >
            <option value="easy">Easy</option>
            <option value="medium">Medium</option>
            <option value="hard">Hard</option>
          </select>
        </div>
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Rating</label>
          <input aria-label="Rating"
            type="number"
            min={0}
            max={3500}
            step={100}
            value={formState.rating}
            onChange={e => onUpdate('rating', Number(e.target.value))}
            placeholder="e.g. 1500"
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          />
          <p className="mt-1 text-xs text-muted-soft">0 = unrated. Filters on the problem list.</p>
        </div>
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Time Limit (ms)</label>
          <input aria-label="Time Limit (ms)"
            type="number"
            value={formState.timeLimit}
            onChange={e => onUpdate('timeLimit', Number(e.target.value))}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Memory Limit (KB)</label>
          <input aria-label="Memory Limit (KB)"
            type="number"
            min={1}
            max={2097152}
            value={formState.memoryLimit}
            onChange={e => onUpdate('memoryLimit', Number(e.target.value))}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          />
          <p className="mt-1 text-xs text-muted-soft">Max 2097152 KB (2 GiB). Multiplied per language at judge time — Java and Python count double.</p>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="space-y-4">
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="block text-xs font-semibold text-muted uppercase">Description (Markdown + LaTeX)</span>
              <input aria-label="Description (Markdown + LaTeX)"
                type="file"
                ref={imageInputRef}
                onChange={handleImageChange}
                accept=".png,.jpg,.jpeg,.gif"
                className="hidden"
              />
              <button
                type="button"
                onClick={() => imageInputRef.current?.click()}
                disabled={uploadingImage}
                className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand hover:text-brand disabled:opacity-50 transition-colors cursor-pointer"
              >
                <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
                {uploadingImage ? 'Uploading...' : 'Insert Image'}
              </button>
            </div>
            <RichTextField
              ariaLabel="Description (Markdown + LaTeX)"
              value={formState.description}
              onChange={(markdown) => onUpdate('description', markdown)}
              placeholder="Write your problem statement..."
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <RichTextField
                ariaLabel="Input Format"
                label="Input Format"
                value={formState.inputFormat}
                onChange={(markdown) => onUpdate('inputFormat', markdown)}
                placeholder="Describe the input format..."
                rows={4}
              />
            </div>
            <div>
              <RichTextField
                ariaLabel="Output Format"
                label="Output Format"
                value={formState.outputFormat}
                onChange={(markdown) => onUpdate('outputFormat', markdown)}
                placeholder="Describe the output format..."
                rows={4}
              />
            </div>
          </div>

          <div>
            <p className="mb-1 text-xs text-muted uppercase font-semibold">Constraints</p>
            <p className="mb-1 text-xs text-muted-soft">
              One bound per line, e.g. <code className="font-mono">$1 \le n \le 2\cdot10^5$</code>. Rendered as its own section on the problem page.
            </p>
            <RichTextField
              ariaLabel="Constraints"
              value={formState.constraints}
              onChange={(markdown) => onUpdate('constraints', markdown)}
              placeholder="e.g. $1 \le n \le 2\cdot10^5$"
              rows={3}
            />
          </div>

          <div>
            <RichTextField
              ariaLabel="Notes / Hint (optional)"
              label="Notes / Hint (optional)"
              value={formState.hint}
              onChange={(markdown) => onUpdate('hint', markdown)}
              placeholder="Optional clarification shown under Notes. Do not reveal the solution."
              rows={3}
            />
          </div>
        </div>

        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Live Preview</label>
          <div className="prose prose-sm max-w-none bg-surface-2 border border-hairline rounded-lg p-4 max-h-[600px] overflow-y-auto space-y-4">
            {formState.description || formState.inputFormat || formState.outputFormat || formState.constraints || formState.hint ? (
              <>
                <RichTextPreview label="Description" value={formState.description} />
                <RichTextPreview label="Input Format" value={formState.inputFormat} />
                <RichTextPreview label="Output Format" value={formState.outputFormat} />
                <RichTextPreview label="Constraints" value={formState.constraints} />
                <RichTextPreview label="Notes" value={formState.hint} />
              </>
            ) : (
              <p className="text-muted-soft text-sm italic">Start typing to see a live preview of your problem statement with rendered Markdown and LaTeX math formulas.</p>
            )}
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold text-sm text-ink-soft">Sample Cases</h3>
          <button
            type="button"
            onClick={() => onUpdate('sampleCases', [...formState.sampleCases, { input: '', output: '', explanation: '' }])}
            className="bg-brand text-on-brand px-3 py-1.5 rounded text-xs hover:bg-brand-hover transition-colors cursor-pointer"
          >
            + Add Sample
          </button>
        </div>

        {formState.sampleCases.map((sc, i) => (
          <div key={i} className="border border-hairline rounded-lg p-4 space-y-3 bg-surface">
            <div className="flex items-center justify-between">
              <span className="font-medium text-sm text-ink">Sample {i + 1}</span>
              <button
                type="button"
                onClick={() => onUpdate('sampleCases', formState.sampleCases.filter((_, j) => j !== i))}
                className="text-bad hover:text-bad/80 text-xs cursor-pointer"
              >
                Remove
              </button>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs font-medium text-muted mb-1">Input</label>
                <textarea aria-label="Input"
                  value={sc.input}
                  onChange={e => {
                    const updated = [...formState.sampleCases]
                    updated[i] = { ...updated[i], input: e.target.value }
                    onUpdate('sampleCases', updated)
                  }}
                  rows={3}
                  className="w-full font-mono text-xs border border-hairline-strong rounded p-2 focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-muted mb-1">Expected Output</label>
                <textarea aria-label="Expected Output"
                  value={sc.output}
                  onChange={e => {
                    const updated = [...formState.sampleCases]
                    updated[i] = { ...updated[i], output: e.target.value }
                    onUpdate('sampleCases', updated)
                  }}
                  rows={3}
                  className="w-full font-mono text-xs border border-hairline-strong rounded p-2 focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface"
                />
              </div>
            </div>
            <div>
              <p className="text-xs font-medium text-muted mb-1">Explanation (optional)</p>
              {/* Rendered as markdown on the problem page, so it takes the same
                  editor as the statement rather than a plain textarea. */}
              <RichTextField
                ariaLabel="Explanation (optional)"
                value={sc.explanation ?? ''}
                onChange={(markdown) => {
                  const updated = [...formState.sampleCases]
                  updated[i] = { ...updated[i], explanation: markdown }
                  onUpdate('sampleCases', updated)
                }}
                rows={2}
                minHeight="80px"
              />
            </div>
          </div>
        ))}
      </div>

      <button
        type="submit"
        disabled={saving}
        className="bg-brand text-on-brand px-5 py-2 rounded text-sm font-semibold hover:bg-brand-hover disabled:opacity-50 transition-colors cursor-pointer"
      >
        {saving ? 'Saving...' : 'Save Statement'}
      </button>
    </form>
  )
}
