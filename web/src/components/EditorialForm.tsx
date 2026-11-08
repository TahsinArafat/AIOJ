import { useState } from 'react'
import { api } from '../lib/api'
import MarkdownContent from './MarkdownContent'

interface EditorialFormProps {
  problemId: string
  isUserAdmin: boolean
  onSuccess: () => void
  onCancel?: () => void
}

export default function EditorialForm({ problemId, isUserAdmin, onSuccess, onCancel }: EditorialFormProps) {
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [approach, setApproach] = useState('')
  const [solutionCode, setSolutionCode] = useState('')
  const [solutionLanguage, setSolutionLanguage] = useState('cpp')
  const [timeComplexity, setTimeComplexity] = useState('')
  const [spaceComplexity, setSpaceComplexity] = useState('')
  const [isOfficial, setIsOfficial] = useState(false)
  const [showPreview, setShowPreview] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !content.trim()) {
      setError('Title and Content are required.')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      await api.editorials.create({
        problem_id: problemId,
        title: title.trim(),
        content: content.trim(),
        approach: approach.trim() || undefined,
        solution_code: solutionCode.trim() || undefined,
        solution_language: solutionLanguage || undefined,
        time_complexity: timeComplexity.trim() || undefined,
        space_complexity: spaceComplexity.trim() || undefined,
        is_official: isUserAdmin ? isOfficial : false,
      })
      onSuccess()
    } catch (err) {
      setError((err instanceof Error ? err.message : '') || 'Failed to submit editorial.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 text-sm text-ink">
      {error && <div className="p-3 bg-bad/10 text-bad rounded border border-bad/30">{error}</div>}
      
      <div>
        <label className="block font-medium text-ink-soft mb-1">Title *</label>
        <input aria-label="Title"
          type="text"
          value={title}
          onChange={e => setTitle(e.target.value)}
          required
          className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700"
          placeholder="e.g. Optimal Greedy Approach"
        />
      </div>

      <div>
        <label className="block font-medium text-ink-soft mb-1">Approach Description</label>
        <textarea aria-label="Approach Description"
          value={approach}
          onChange={e => setApproach(e.target.value)}
          rows={3}
          className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700"
          placeholder="Briefly summarize the logic/approach..."
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-1">
          <label className="block font-medium text-ink-soft">Editorial Content (Markdown + LaTeX) *</label>
          <button type="button" onClick={() => setShowPreview(v => !v)} className="text-xs text-brand hover:underline">
            {showPreview ? 'Write' : 'Preview'}
          </button>
        </div>
        {showPreview ? (
          <div aria-label="Editorial content preview" className="prose max-w-none text-ink border rounded p-3 bg-surface dark:border-gray-700 min-h-32">
            {content.trim()
              ? <MarkdownContent content={content} />
              : <p className="text-muted-soft italic">Nothing to preview yet.</p>}
          </div>
        ) : (
          <textarea aria-label="Editorial Content (Markdown + LaTeX)"
            value={content}
            onChange={e => setContent(e.target.value)}
            rows={6}
            required
            className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700 font-mono"
            placeholder="Detailed explanation, proofs, cases... Use $...$ for inline math and $$...$$ for display math."
          />
        )}
        <p className="mt-1 text-xs text-muted">Markdown and LaTeX math are rendered when the editorial is viewed.</p>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block font-medium text-ink-soft mb-1">Time Complexity</label>
          <input aria-label="Time Complexity"
            type="text"
            value={timeComplexity}
            onChange={e => setTimeComplexity(e.target.value)}
            className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700"
            placeholder="e.g. O(N log N)"
          />
        </div>
        <div>
          <label className="block font-medium text-ink-soft mb-1">Space Complexity</label>
          <input aria-label="Space Complexity"
            type="text"
            value={spaceComplexity}
            onChange={e => setSpaceComplexity(e.target.value)}
            className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700"
            placeholder="e.g. O(N)"
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block font-medium text-ink-soft mb-1">Solution Language</label>
          <select aria-label="Solution Language"
            value={solutionLanguage}
            onChange={e => setSolutionLanguage(e.target.value)}
            className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700"
          >
            <option value="cpp">C++</option>
            <option value="python">Python</option>
            <option value="java">Java</option>
            <option value="go">Go</option>
            <option value="rust">Rust</option>
            <option value="javascript">JavaScript</option>
          </select>
        </div>
      </div>

      <div>
        <label className="block font-medium text-ink-soft mb-1">Solution Code</label>
        <textarea aria-label="Solution Code"
          value={solutionCode}
          onChange={e => setSolutionCode(e.target.value)}
          rows={6}
          className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700 font-mono"
          placeholder="Paste clean solution code here..."
        />
      </div>

      {isUserAdmin && (
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="isOfficial"
            checked={isOfficial}
            onChange={e => setIsOfficial(e.target.checked)}
            className="rounded border-hairline-strong"
          />
          <label htmlFor="isOfficial" className="font-medium text-ink-soft">Mark as Official Editorial</label>
        </div>
      )}

      <div className="flex gap-2 justify-end pt-4">
        {onCancel && (
          <button
            type="button"
            onClick={onCancel}
            disabled={submitting}
            className="border px-4 py-2 rounded text-ink-soft hover:bg-surface-2"
          >
            Cancel
          </button>
        )}
        <button
          type="submit"
          disabled={submitting}
          className="bg-brand hover:bg-brand-hover text-on-brand px-4 py-2 rounded font-medium disabled:opacity-50"
        >
          {submitting ? 'Submitting...' : 'Save Editorial'}
        </button>
      </div>
    </form>
  )
}
