import { lazy, Suspense, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import remarkGfm from 'remark-gfm'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'
import type { ProblemFormState } from '../../types/problem-workspace'

// The tiptap/lowlight/turndown stack is by far the heaviest part of the
// setter workspace. Lazy-loading it keeps StatementTab itself small and
// defers the editor bundle until React actually mounts the first field.
const VisualEditor = lazy(() => import('../VisualEditor'))

function VisualEditorField(props: { content: string; onChange: (markdown: string) => void; placeholder?: string }) {
  return (
    <Suspense
      fallback={
        <div
          aria-label="Loading editor"
          className="border border-hairline-strong rounded-lg min-h-[200px] animate-pulse bg-surface-2"
        />
      }
    >
      <VisualEditor {...props} />
    </Suspense>
  )
}

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
          <input
            type="text"
            value={formState.title}
            onChange={e => onUpdate('title', e.target.value)}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Tags (comma-separated)</label>
          <input
            type="text"
            value={formState.tags}
            onChange={e => onUpdate('tags', e.target.value)}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          />
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Difficulty</label>
          <select
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
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Time Limit (ms)</label>
          <input
            type="number"
            value={formState.timeLimit}
            onChange={e => onUpdate('timeLimit', Number(e.target.value))}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Memory Limit (KB)</label>
          <input
            type="number"
            value={formState.memoryLimit}
            onChange={e => onUpdate('memoryLimit', Number(e.target.value))}
            className="w-full border border-hairline-strong rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
          />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="space-y-4">
          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="block text-xs font-semibold text-muted uppercase">Description (Markdown + LaTeX)</label>
              <input
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
            <VisualEditorField
              content={formState.description}
              onChange={(markdown) => onUpdate('description', markdown)}
              placeholder="Write your problem statement..."
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-muted uppercase mb-1">Input Format</label>
              <VisualEditorField
                content={formState.inputFormat}
                onChange={(markdown) => onUpdate('inputFormat', markdown)}
                placeholder="Describe the input format..."
              />
            </div>
            <div>
              <label className="block text-xs font-semibold text-muted uppercase mb-1">Output Format</label>
              <VisualEditorField
                content={formState.outputFormat}
                onChange={(markdown) => onUpdate('outputFormat', markdown)}
                placeholder="Describe the output format..."
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted uppercase mb-1">Constraints</label>
            <VisualEditorField
              content={formState.hint}
              onChange={(markdown) => onUpdate('hint', markdown)}
              placeholder="Add constraints..."
            />
          </div>
        </div>

        <div>
          <label className="block text-xs font-semibold text-muted uppercase mb-1">Live Preview</label>
          <div className="prose prose-sm max-w-none bg-surface-2 border border-hairline rounded-lg p-4 max-h-[600px] overflow-y-auto">
            {formState.description || formState.inputFormat || formState.outputFormat || formState.hint ? (
              <div className="space-y-4">
                {formState.description && (
                  <div>
                    <h4 className="text-xs font-bold text-muted-soft uppercase tracking-wider mb-2">Description</h4>
                    <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                      {formState.description}
                    </ReactMarkdown>
                  </div>
                )}
                {formState.inputFormat && (
                  <div>
                    <h4 className="text-xs font-bold text-muted-soft uppercase tracking-wider mb-2">Input Format</h4>
                    <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                      {formState.inputFormat}
                    </ReactMarkdown>
                  </div>
                )}
                {formState.outputFormat && (
                  <div>
                    <h4 className="text-xs font-bold text-muted-soft uppercase tracking-wider mb-2">Output Format</h4>
                    <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                      {formState.outputFormat}
                    </ReactMarkdown>
                  </div>
                )}
                {formState.hint && (
                  <div>
                    <h4 className="text-xs font-bold text-muted-soft uppercase tracking-wider mb-2">Constraints</h4>
                    <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
                      {formState.hint}
                    </ReactMarkdown>
                  </div>
                )}
              </div>
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
                className="text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 text-xs cursor-pointer"
              >
                Remove
              </button>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs font-medium text-muted mb-1">Input</label>
                <textarea
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
                <textarea
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
              <label className="block text-xs font-medium text-muted mb-1">Explanation (optional)</label>
              <input
                type="text"
                value={sc.explanation ?? ''}
                onChange={e => {
                  const updated = [...formState.sampleCases]
                  updated[i] = { ...updated[i], explanation: e.target.value }
                  onUpdate('sampleCases', updated)
                }}
                className="w-full text-sm border border-hairline-strong rounded px-2 py-1.5 focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface"
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
