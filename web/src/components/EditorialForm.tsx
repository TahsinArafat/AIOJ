import { useState } from 'react'
import { api } from '../lib/api'

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
        <input
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
        <textarea
          value={approach}
          onChange={e => setApproach(e.target.value)}
          rows={3}
          className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700"
          placeholder="Briefly summarize the logic/approach..."
        />
      </div>

      <div>
        <label className="block font-medium text-ink-soft mb-1">Editorial Content (Markdown Supported) *</label>
        <textarea
          value={content}
          onChange={e => setContent(e.target.value)}
          rows={6}
          required
          className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700 font-mono"
          placeholder="Detailed explanation, proofs, cases..."
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block font-medium text-ink-soft mb-1">Time Complexity</label>
          <input
            type="text"
            value={timeComplexity}
            onChange={e => setTimeComplexity(e.target.value)}
            className="w-full border rounded px-3 py-2 bg-surface dark:border-gray-700"
            placeholder="e.g. O(N log N)"
          />
        </div>
        <div>
          <label className="block font-medium text-ink-soft mb-1">Space Complexity</label>
          <input
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
          <select
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
        <textarea
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
