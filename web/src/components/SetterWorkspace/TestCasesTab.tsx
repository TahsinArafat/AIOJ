import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { TestcaseFile, TestcaseFileContent } from '../../types/api'
import type { TestCase } from '../../types/problem-workspace'
import { Badge } from '../ui/Badge'
import { Button } from '../ui/Button'
import { Table, TableShell, THead, Th, Td, Tr } from '../ui/Table'

/** The server serves files up to 8 MiB; the textarea is useless past that. */
export const MAX_EDITABLE_FILE_BYTES = 8 * 1024 * 1024

interface TestCasesTabProps {
  testcases: TestCase[]
  files: TestcaseFile[]
  filesLoading: boolean
  newTestCase: TestCase
  batchScore: number
  batchApplying: boolean
  onAdd: () => void
  onRemove: (index: number) => void
  onScoreChange: (index: number, score: number) => void
  onBatchSet: () => void
  onUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  onNewTestCaseChange: (tc: TestCase) => void
  onBatchScoreChange: (score: number) => void
  onReloadFiles: () => void
  onLoadFile: (name: string) => Promise<TestcaseFileContent | null>
  onSaveFile: (name: string, content: string) => Promise<boolean>
  onCreatePair: (pair: { stem: string; input: string; output: string; score: number }) => Promise<boolean>
  onDeleteFile: (name: string) => void
  onDownloadFile: (name: string) => void
  onDownloadAll: () => void
}

interface EditorState {
  mode: 'view' | 'edit'
  name: string
  content: string
  size: number
  binary: boolean
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const ROLE_TONE = { input: 'brand', output: 'accent', other: 'neutral' } as const

function FileEditor({
  editor,
  saving,
  onChange,
  onClose,
  onSave,
  onDownload,
}: {
  editor: EditorState
  saving: boolean
  onChange: (content: string) => void
  onClose: () => void
  onSave: () => void
  onDownload: () => void
}) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  useEffect(() => {
    textareaRef.current?.focus()
  }, [])

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={`${editor.mode === 'edit' ? 'Edit' : 'View'} ${editor.name}`}
        className="flex max-h-[85vh] w-full max-w-3xl flex-col rounded-xl border border-hairline bg-surface shadow-xl"
      >
        <div className="flex items-center justify-between gap-3 border-b border-hairline px-5 py-3">
          <div className="min-w-0">
            <h3 className="truncate font-mono text-sm font-semibold text-ink">{editor.name}</h3>
            <p className="text-xs text-muted">
              {formatBytes(editor.size)}
              {editor.mode === 'view' && ' — read-only'}
            </p>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>

        <div className="min-h-0 flex-1 overflow-auto px-5 py-4">
          {editor.binary ? (
            <p className="text-sm text-muted">
              This file is not valid UTF-8, so it cannot be shown or edited here.
              Download it to inspect the bytes.
            </p>
          ) : (
            <textarea
              ref={textareaRef}
              aria-label="Test case content"
              value={editor.content}
              readOnly={editor.mode === 'view'}
              onChange={e => onChange(e.target.value)}
              spellCheck={false}
              className="h-[50vh] w-full resize-none rounded-lg border border-hairline-strong bg-surface-2 p-3 font-mono text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
            />
          )}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-hairline px-5 py-3">
          <Button variant="secondary" size="sm" onClick={onDownload}>
            Download
          </Button>
          {editor.mode === 'edit' && !editor.binary && (
            <Button size="sm" onClick={onSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save file'}
            </Button>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

export default function TestCasesTab({
  testcases,
  files,
  filesLoading,
  newTestCase,
  batchScore,
  batchApplying,
  onAdd,
  onRemove,
  onScoreChange,
  onBatchSet,
  onUpload,
  onNewTestCaseChange,
  onBatchScoreChange,
  onReloadFiles,
  onLoadFile,
  onSaveFile,
  onCreatePair,
  onDeleteFile,
  onDownloadFile,
  onDownloadAll,
}: TestCasesTabProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  const [editor, setEditor] = useState<EditorState | null>(null)
  const [openingFile, setOpeningFile] = useState<string | null>(null)
  const [savingFile, setSavingFile] = useState(false)
  const [newPair, setNewPair] = useState({ stem: '', input: '', output: '', score: 10 })
  const [creating, setCreating] = useState(false)

  const openFile = async (name: string, mode: 'view' | 'edit') => {
    setOpeningFile(name)
    const data = await onLoadFile(name)
    setOpeningFile(null)
    if (!data) return
    setEditor({ mode, name: data.name, content: data.content, size: data.size, binary: data.binary })
  }

  const saveEditor = async () => {
    if (!editor) return
    setSavingFile(true)
    const saved = await onSaveFile(editor.name, editor.content)
    setSavingFile(false)
    if (saved) setEditor(null)
  }

  const createPair = async () => {
    if (!newPair.stem.trim()) return
    setCreating(true)
    const created = await onCreatePair(newPair)
    setCreating(false)
    if (created) setNewPair({ stem: '', input: '', output: '', score: 10 })
  }

  const canEdit = (file: TestcaseFile) => file.size <= MAX_EDITABLE_FILE_BYTES

  return (
    <div className="space-y-8">
      {/* ── Files ─────────────────────────────────────────────────────── */}
      <section>
        <div className="mb-4 flex flex-wrap items-end justify-between gap-3 border-b pb-2">
          <div>
            <h2 className="text-lg font-bold text-ink">Test Data Files</h2>
            <p className="text-xs text-muted-soft">
              Every file the judge reads for this problem. View or edit text cases inline, or
              download individual files.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <input type="file" ref={fileInputRef} onChange={onUpload} accept=".zip" className="hidden" />
            <Button variant="secondary" size="sm" onClick={() => fileInputRef.current?.click()}>
              Upload ZIP
            </Button>
            <Button variant="secondary" size="sm" onClick={onDownloadAll} disabled={files.length === 0}>
              Download all
            </Button>
            <Button variant="ghost" size="sm" onClick={onReloadFiles} disabled={filesLoading}>
              {filesLoading ? 'Refreshing...' : 'Refresh'}
            </Button>
          </div>
        </div>

        {files.length === 0 ? (
          <p className="rounded-lg border border-dashed border-hairline-strong bg-surface-2 px-4 py-6 text-center text-xs text-muted-soft">
            No test data files yet. Upload a ZIP of <code>01.in</code> / <code>01.out</code> pairs, or
            write a new test case below.
          </p>
        ) : (
          <TableShell>
            <Table>
              <THead>
                <tr>
                  <Th>File</Th>
                  <Th>Type</Th>
                  <Th>Size</Th>
                  <Th className="text-right">Actions</Th>
                </tr>
              </THead>
              <tbody>
                {files.map(file => (
                  <Tr key={file.name}>
                    <Td className="font-mono text-xs text-ink">{file.name}</Td>
                    <Td>
                      <div className="flex items-center gap-1.5">
                        <Badge tone={ROLE_TONE[file.role]}>{file.role}</Badge>
                        {file.role !== 'other' && !file.pair && <Badge tone="warn">unpaired</Badge>}
                      </div>
                    </Td>
                    <Td className="text-xs text-muted">{formatBytes(file.size)}</Td>
                    <Td>
                      <div className="flex justify-end gap-3 text-xs">
                        <button
                          type="button"
                          onClick={() => openFile(file.name, 'view')}
                          disabled={!canEdit(file) || openingFile === file.name}
                          className="cursor-pointer text-brand hover:underline disabled:cursor-not-allowed disabled:text-muted-soft disabled:no-underline"
                        >
                          View
                        </button>
                        <button
                          type="button"
                          onClick={() => openFile(file.name, 'edit')}
                          disabled={!canEdit(file) || openingFile === file.name}
                          className="cursor-pointer text-brand hover:underline disabled:cursor-not-allowed disabled:text-muted-soft disabled:no-underline"
                        >
                          Edit
                        </button>
                        <button
                          type="button"
                          onClick={() => onDownloadFile(file.name)}
                          className="cursor-pointer text-brand hover:underline"
                        >
                          Download
                        </button>
                        <button
                          type="button"
                          onClick={() => onDeleteFile(file.name)}
                          className="cursor-pointer text-bad hover:underline"
                        >
                          Delete
                        </button>
                      </div>
                    </Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          </TableShell>
        )}
      </section>

      {/* ── Scores ────────────────────────────────────────────────────── */}
      <section>
        <div className="mb-4 border-b pb-2">
          <h2 className="text-lg font-bold text-ink">Testcase Scores &amp; Breakdown</h2>
          <p className="text-xs text-muted-soft">
            Which input/output pair is judged, and how many points each pair is worth.
          </p>
        </div>

        <TableShell className="mb-4">
          <Table>
            <THead>
              <tr>
                <Th>Input File</Th>
                <Th>Output File</Th>
                <Th>Score</Th>
                <Th className="text-right">Action</Th>
              </tr>
            </THead>
            <tbody>
              {testcases.map((tc, idx) => (
                <Tr key={`${tc.input_name}-${tc.output_name}-${idx}`}>
                  <Td className="font-mono text-xs text-ink">{tc.input_name}</Td>
                  <Td className="font-mono text-xs text-ink">{tc.output_name}</Td>
                  <Td>
                    <div className="flex items-center gap-1">
                      <input
                        aria-label={`Score for ${tc.input_name}`}
                        type="number"
                        min={0}
                        defaultValue={tc.score}
                        onBlur={e => {
                          const next = Number(e.target.value)
                          if (Number.isFinite(next) && next !== tc.score) onScoreChange(idx, next)
                        }}
                        className="w-20 rounded border border-hairline-strong bg-surface px-2 py-1 text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                      />
                      <span className="text-xs text-muted-soft">pts</span>
                    </div>
                  </Td>
                  <Td>
                    <div className="text-right">
                      <button
                        type="button"
                        onClick={() => onRemove(idx)}
                        className="cursor-pointer text-xs text-bad hover:underline"
                      >
                        Remove
                      </button>
                    </div>
                  </Td>
                </Tr>
              ))}
              {testcases.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-6 text-center text-xs text-muted-soft">
                    No testcase scores registered. Add a pair below, or upload a ZIP to auto-discover
                    pairs.
                  </td>
                </tr>
              )}
            </tbody>
          </Table>
        </TableShell>

        {testcases.length > 0 && (
          <div className="mb-4 flex flex-wrap items-end justify-between gap-3 rounded-lg border border-hairline bg-surface-2 p-4">
            <div className="flex items-end gap-3">
              <div>
                <label htmlFor="batch-score" className="mb-1 block text-xs text-muted">
                  Set Score for All Test Cases
                </label>
                <input
                  id="batch-score"
                  aria-label="Set Score for All Test Cases"
                  type="number"
                  value={batchScore}
                  onChange={e => onBatchScoreChange(Number(e.target.value))}
                  className="w-28 rounded border border-hairline-strong bg-surface px-3 py-1 text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
              </div>
              <Button size="sm" onClick={onBatchSet} disabled={batchApplying}>
                {batchApplying ? 'Applying...' : 'Apply to All'}
              </Button>
            </div>
            <span className="text-xs font-medium text-muted">
              Total Testcases: {testcases.length} | Total Points:{' '}
              {testcases.reduce((sum, tc) => sum + (tc.score || 0), 0)} pts
            </span>
          </div>
        )}
      </section>

      {/* ── Add ───────────────────────────────────────────────────────── */}
      <section>
        <div className="mb-4 border-b pb-2">
          <h2 className="text-lg font-bold text-ink">Add a Test Case</h2>
          <p className="text-xs text-muted-soft">
            Write the input and expected output and both files are created as{' '}
            <span className="font-mono">&lt;name&gt;.in</span> /{' '}
            <span className="font-mono">&lt;name&gt;.out</span>.
          </p>
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <div className="space-y-3 rounded-lg border border-hairline bg-surface-2 p-4">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted">
              Write input &amp; output
            </h3>
            <div className="flex items-end gap-3">
              <div className="flex-1">
                <label htmlFor="new-pair-stem" className="mb-1 block text-xs text-muted">
                  Test case name
                </label>
                <input
                  id="new-pair-stem"
                  type="text"
                  placeholder="e.g. 03"
                  value={newPair.stem}
                  onChange={e => setNewPair({ ...newPair, stem: e.target.value })}
                  className="w-full rounded border border-hairline-strong bg-surface px-3 py-1.5 font-mono text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
              </div>
              <div>
                <label htmlFor="new-pair-score" className="mb-1 block text-xs text-muted">
                  Score
                </label>
                <input
                  id="new-pair-score"
                  type="number"
                  min={0}
                  value={newPair.score}
                  onChange={e => setNewPair({ ...newPair, score: Number(e.target.value) })}
                  className="w-20 rounded border border-hairline-strong bg-surface px-3 py-1.5 text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
              </div>
            </div>
            <div>
              <label htmlFor="new-pair-input" className="mb-1 block text-xs text-muted">
                Input
              </label>
              <textarea
                id="new-pair-input"
                value={newPair.input}
                onChange={e => setNewPair({ ...newPair, input: e.target.value })}
                spellCheck={false}
                placeholder="1 2"
                className="h-24 w-full resize-y rounded border border-hairline-strong bg-surface p-2 font-mono text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
              />
            </div>
            <div>
              <label htmlFor="new-pair-output" className="mb-1 block text-xs text-muted">
                Expected output
              </label>
              <textarea
                id="new-pair-output"
                value={newPair.output}
                onChange={e => setNewPair({ ...newPair, output: e.target.value })}
                spellCheck={false}
                placeholder="3"
                className="h-24 w-full resize-y rounded border border-hairline-strong bg-surface p-2 font-mono text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
              />
            </div>
            <Button size="sm" onClick={createPair} disabled={creating || !newPair.stem.trim()}>
              {creating ? 'Creating...' : 'Create test case'}
            </Button>
          </div>

          <div className="space-y-3 rounded-lg border border-hairline bg-surface-2 p-4">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted">
              Register existing files
            </h3>
            <p className="text-xs text-muted-soft">
              Match two files that are already in the directory (for example after a ZIP upload with
              non-standard extensions) and give the pair a score.
            </p>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="register-input" className="mb-1 block text-xs text-muted">
                  Input file
                </label>
                <input
                  id="register-input"
                  aria-label="Input Name"
                  type="text"
                  list="testcase-file-names"
                  placeholder="e.g. 01.in"
                  value={newTestCase.input_name}
                  onChange={e => onNewTestCaseChange({ ...newTestCase, input_name: e.target.value })}
                  className="w-full rounded border border-hairline-strong bg-surface px-3 py-1.5 font-mono text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
              </div>
              <div>
                <label htmlFor="register-output" className="mb-1 block text-xs text-muted">
                  Output file
                </label>
                <input
                  id="register-output"
                  aria-label="Output Name"
                  type="text"
                  list="testcase-file-names"
                  placeholder="e.g. 01.out"
                  value={newTestCase.output_name}
                  onChange={e => onNewTestCaseChange({ ...newTestCase, output_name: e.target.value })}
                  className="w-full rounded border border-hairline-strong bg-surface px-3 py-1.5 font-mono text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
              </div>
            </div>
            <div>
              <label htmlFor="register-score" className="mb-1 block text-xs text-muted">
                Score
              </label>
              <input
                id="register-score"
                aria-label="Score"
                type="number"
                min={0}
                value={newTestCase.score}
                onChange={e => onNewTestCaseChange({ ...newTestCase, score: Number(e.target.value) })}
                className="w-24 rounded border border-hairline-strong bg-surface px-3 py-1.5 text-xs text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
              />
            </div>
            <Button size="sm" onClick={onAdd}>
              Add file match
            </Button>
          </div>
        </div>

        <datalist id="testcase-file-names">
          {files.map(file => (
            <option key={file.name} value={file.name} />
          ))}
        </datalist>
      </section>

      {editor && (
        <FileEditor
          editor={editor}
          saving={savingFile}
          onChange={content => setEditor({ ...editor, content })}
          onClose={() => setEditor(null)}
          onSave={saveEditor}
          onDownload={() => onDownloadFile(editor.name)}
        />
      )}
    </div>
  )
}
