import { useRef } from 'react'
import type { TestCase } from '../../types/problem-workspace'

interface TestCasesTabProps {
  testcases: TestCase[]
  newTestCase: TestCase
  batchScore: number
  batchApplying: boolean
  onAdd: () => void
  onRemove: (index: number) => void
  onBatchSet: () => void
  onUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  onNewTestCaseChange: (tc: TestCase) => void
  onBatchScoreChange: (score: number) => void
}

export default function TestCasesTab({
  testcases,
  newTestCase,
  batchScore,
  batchApplying,
  onAdd,
  onRemove,
  onBatchSet,
  onUpload,
  onNewTestCaseChange,
  onBatchScoreChange,
}: TestCasesTabProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-bold text-ink border-b pb-2 mb-4">Upload Testcase Package (ZIP)</h2>
        <p className="text-xs text-muted-soft mb-3">
          Upload a ZIP package containing input and output testcases (e.g. `01.in` and `01.out`).
          Files will be parsed and loaded into the problem storage directory on the sandbox filesystem.
        </p>
        <div className="flex gap-3 items-center">
          <input
            type="file"
            ref={fileInputRef}
            onChange={onUpload}
            accept=".zip"
            className="hidden"
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            className="bg-brand text-on-brand px-4 py-2 rounded text-sm font-semibold hover:bg-brand-hover transition-colors cursor-pointer"
          >
            Choose ZIP file
          </button>
        </div>
      </div>

      <hr className="border-hairline" />

      <div>
        <h2 className="text-lg font-bold text-ink border-b pb-2 mb-4">Testcase Scores & Breakdown</h2>
        <div className="border border-hairline rounded-lg overflow-hidden mb-4">
          <table className="w-full text-sm text-left bg-surface">
            <thead className="bg-surface-2 text-muted text-xs uppercase font-semibold">
              <tr>
                <th className="px-4 py-2.5">Input File</th>
                <th className="px-4 py-2.5">Output File</th>
                <th className="px-4 py-2.5">Score</th>
                <th className="px-4 py-2.5 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-hairline">
              {testcases.map((tc, idx) => (
                <tr key={idx} className="hover:bg-surface-2">
                  <td className="px-4 py-2.5 font-mono text-xs text-ink">{tc.input_name}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-ink">{tc.output_name}</td>
                  <td className="px-4 py-2.5 text-ink">{tc.score} pts</td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => onRemove(idx)}
                      className="text-red-600 dark:text-red-400 hover:underline cursor-pointer"
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
              {testcases.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-6 text-center text-muted-soft text-xs">
                    No testcase scores registered. Add testcase scores below to match files.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {testcases.length > 0 && (
          <div className="bg-surface-2 rounded-lg p-4 border border-hairline mb-4 flex items-end justify-between">
            <div className="flex gap-4 items-end">
              <div>
                <label className="block text-xs text-muted mb-1">Set Score for All Test Cases</label>
                <input
                  type="number"
                  value={batchScore}
                  onChange={e => onBatchScoreChange(Number(e.target.value))}
                  className="border border-hairline-strong rounded px-3 py-1 text-xs w-28 focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
                />
              </div>
              <button
                onClick={onBatchSet}
                disabled={batchApplying}
                className="bg-brand text-on-brand px-3 py-1.5 rounded text-xs hover:bg-brand-hover disabled:opacity-50 transition-colors cursor-pointer"
              >
                {batchApplying ? 'Applying...' : 'Apply to All'}
              </button>
            </div>
            <span className="text-xs text-muted font-medium">
              Total Testcases: {testcases.length} | Total Points: {testcases.reduce((sum, tc) => sum + (tc.score || 0), 0)} pts
            </span>
          </div>
        )}

        <div className="bg-surface-2 rounded-lg p-4 border border-hairline space-y-4">
          <h4 className="font-semibold text-xs text-muted uppercase tracking-wider">Register TestCase File Matches</h4>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs text-muted mb-1">Input Name</label>
              <input
                type="text"
                placeholder="e.g. 01.in"
                value={newTestCase.input_name}
                onChange={e => onNewTestCaseChange({ ...newTestCase, input_name: e.target.value })}
                className="w-full border border-hairline-strong rounded px-3 py-1.5 text-xs font-mono focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
              />
            </div>
            <div>
              <label className="block text-xs text-muted mb-1">Output Name</label>
              <input
                type="text"
                placeholder="e.g. 01.out"
                value={newTestCase.output_name}
                onChange={e => onNewTestCaseChange({ ...newTestCase, output_name: e.target.value })}
                className="w-full border border-hairline-strong rounded px-3 py-1.5 text-xs font-mono focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
              />
            </div>
            <div>
              <label className="block text-xs text-muted mb-1">Score</label>
              <input
                type="number"
                value={newTestCase.score}
                onChange={e => onNewTestCaseChange({ ...newTestCase, score: Number(e.target.value) })}
                className="w-full border border-hairline-strong rounded px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
              />
            </div>
          </div>
          <button
            onClick={onAdd}
            className="bg-brand text-on-brand px-3 py-1.5 rounded text-xs hover:bg-brand-hover transition-colors cursor-pointer"
          >
            Add File Match
          </button>
        </div>
      </div>
    </div>
  )
}
