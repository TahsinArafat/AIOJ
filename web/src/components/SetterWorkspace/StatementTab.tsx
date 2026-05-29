import type { ProblemFormState } from '../../types/problem-workspace'

interface StatementTabProps {
  formState: ProblemFormState
  saving: boolean
  onUpdate: <K extends keyof ProblemFormState>(key: K, value: ProblemFormState[K]) => void
  onSave: (e: React.FormEvent) => void
}

export default function StatementTab({ formState, saving, onUpdate, onSave }: StatementTabProps) {
  return (
    <form onSubmit={onSave} className="space-y-4">
      <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Edit Problem Statement</h2>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Title</label>
          <input
            type="text"
            value={formState.title}
            onChange={e => onUpdate('title', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Tags (comma-separated)</label>
          <input
            type="text"
            value={formState.tags}
            onChange={e => onUpdate('tags', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Difficulty</label>
          <select
            value={formState.difficulty}
            onChange={e => onUpdate('difficulty', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
          >
            <option value="easy">Easy</option>
            <option value="medium">Medium</option>
            <option value="hard">Hard</option>
          </select>
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Time Limit (ms)</label>
          <input
            type="number"
            value={formState.timeLimit}
            onChange={e => onUpdate('timeLimit', Number(e.target.value))}
            className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Memory Limit (KB)</label>
          <input
            type="number"
            value={formState.memoryLimit}
            onChange={e => onUpdate('memoryLimit', Number(e.target.value))}
            className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>

      <div>
        <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Description (Markdown Supported)</label>
        <textarea
          value={formState.description}
          onChange={e => onUpdate('description', e.target.value)}
          rows={8}
          className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
          required
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Input Format</label>
          <textarea
            value={formState.inputFormat}
            onChange={e => onUpdate('inputFormat', e.target.value)}
            rows={4}
            className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Output Format</label>
          <textarea
            value={formState.outputFormat}
            onChange={e => onUpdate('outputFormat', e.target.value)}
            rows={4}
            className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
          />
        </div>
      </div>

      <div>
        <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Hint</label>
        <textarea
          value={formState.hint}
          onChange={e => onUpdate('hint', e.target.value)}
          rows={2}
          className="w-full border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
        />
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold text-sm text-gray-700">Sample Cases</h3>
          <button
            type="button"
            onClick={() => onUpdate('sampleCases', [...formState.sampleCases, { input: '', output: '', explanation: '' }])}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700 transition-colors cursor-pointer"
          >
            + Add Sample
          </button>
        </div>

        {formState.sampleCases.map((sc, i) => (
          <div key={i} className="border border-gray-200 rounded-lg p-4 space-y-3 bg-white">
            <div className="flex items-center justify-between">
              <span className="font-medium text-sm text-gray-800">Sample {i + 1}</span>
              <button
                type="button"
                onClick={() => onUpdate('sampleCases', formState.sampleCases.filter((_, j) => j !== i))}
                className="text-red-600 hover:text-red-800 text-xs cursor-pointer"
              >
                Remove
              </button>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Input</label>
                <textarea
                  value={sc.input}
                  onChange={e => {
                    const updated = [...formState.sampleCases]
                    updated[i] = { ...updated[i], input: e.target.value }
                    onUpdate('sampleCases', updated)
                  }}
                  rows={3}
                  className="w-full font-mono text-xs border border-gray-300 rounded p-2 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Expected Output</label>
                <textarea
                  value={sc.output}
                  onChange={e => {
                    const updated = [...formState.sampleCases]
                    updated[i] = { ...updated[i], output: e.target.value }
                    onUpdate('sampleCases', updated)
                  }}
                  rows={3}
                  className="w-full font-mono text-xs border border-gray-300 rounded p-2 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
                />
              </div>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Explanation (optional)</label>
              <input
                type="text"
                value={sc.explanation}
                onChange={e => {
                  const updated = [...formState.sampleCases]
                  updated[i] = { ...updated[i], explanation: e.target.value }
                  onUpdate('sampleCases', updated)
                }}
                className="w-full text-sm border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
              />
            </div>
          </div>
        ))}
      </div>

      <button
        type="submit"
        disabled={saving}
        className="bg-blue-600 text-white px-5 py-2 rounded text-sm font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
      >
        {saving ? 'Saving...' : 'Save Statement'}
      </button>
    </form>
  )
}
