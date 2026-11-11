import { useEffect, useState } from 'react'
import { errorMessage } from '../lib/errors'
import { PageHeader } from '../components/ui/PageHeader'
import { buttonClass } from '../components/ui/buttonStyles'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useDocumentTitle } from '../lib/documentTitle'
import { api, getAccessToken } from '../lib/api'
import { useConfirm } from '../components/ConfirmDialog'
import type { ProblemFormState, TestCase, Collaborator } from '../types/problem-workspace'
import type { TestcaseFile, TestcaseFileContent } from '../types/api'
import StatementTab from '../components/SetterWorkspace/StatementTab'
import TestCasesTab from '../components/SetterWorkspace/TestCasesTab'
import CheckerTab from '../components/SetterWorkspace/CheckerTab'
import PermissionsTab from '../components/SetterWorkspace/PermissionsTab'
import SettingsTab from '../components/SetterWorkspace/SettingsTab'
import EditorialTab from '../components/SetterWorkspace/EditorialTab'
import TranslationsTab from '../components/SetterWorkspace/TranslationsTab'
import AIStudioTab from '../components/SetterWorkspace/AIStudioTab'
import SubmissionsTab from '../components/SetterWorkspace/SubmissionsTab'

function decodeRole(): string | null {
  const token = localStorage.getItem('access_token')
  if (!token) return null
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return payload.role ?? null
  } catch {
    return null
  }
}

export default function SetterProblemWorkspace() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const confirm = useConfirm()
  interface LoadedProblem {
    id: string
    slug: string
    title: string
    description: string
    /** Review workflow state; publishing is an admin decision. */
    status?: string
    review_note?: string
    visible?: boolean
}

const [problem, setProblem] = useState<LoadedProblem | null>(null)
useDocumentTitle(problem ? `${problem.title} — setter` : null)
  type WorkspaceTab = 'statement' | 'testcases' | 'checker' | 'submissions' | 'permissions' | 'settings' | 'editorial' | 'translations' | 'ai'

  const [activeTab, setActiveTab] = useState<WorkspaceTab>(() => {
    const hash = window.location.hash.replace('#', '') as WorkspaceTab
    const validTabs: WorkspaceTab[] = ['statement', 'testcases', 'checker', 'submissions', 'permissions', 'settings', 'editorial', 'translations', 'ai']
    return validTabs.includes(hash) ? hash : 'statement'
  })
  const role = decodeRole()

  // Two-way hash sync — the initializer only reads the hash once, so a deep
  // link or a Back/Forward step that changes the hash alone left the workspace
  // on the previous tab. replaceState avoids stacking history per tab click.
  useEffect(() => {
    if (window.location.hash.replace('#', '') !== activeTab) {
      window.history.replaceState(null, '', `#${activeTab}`)
    }
    const onHashChange = () => {
      const next = window.location.hash.replace('#', '') as WorkspaceTab
      const valid: WorkspaceTab[] = ['statement', 'testcases', 'checker', 'submissions', 'permissions', 'settings', 'editorial', 'translations', 'ai']
      if (valid.includes(next)) setActiveTab(next)
    }
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [activeTab])

  const [saving, setSaving] = useState(false)
  const [uploadingImage, setUploadingImage] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // Consolidated form state
  const [formState, setFormState] = useState<ProblemFormState>({
    title: '',
    description: '',
    inputFormat: '',
    outputFormat: '',
    constraints: '',
    hint: '',
    timeLimit: 1000,
    memoryLimit: 262144,
    difficulty: 'easy',
    rating: 0,
    tags: '',
    sampleCases: [],
    testcases: [],
    checkerType: 'exact',
    floatEpsilon: 1e-6,
    spj: false,
    spjLanguage: 'cpp-gpp-64',
    spjSourceCode: '',
    interactive: false,
    interactorLanguage: 'cpp-gpp-64',
    interactorSourceCode: '',
  })

  // Test Cases supplementary state
  const [newTestCase, setNewTestCase] = useState<TestCase>({ input_name: '', output_name: '', score: 10 })
  const [batchScore, setBatchScore] = useState<number>(10)
  const [batchApplying, setBatchApplying] = useState(false)
  const [testcaseFiles, setTestcaseFiles] = useState<TestcaseFile[]>([])
  const [testcaseFilesLoading, setTestcaseFilesLoading] = useState(false)

  // Permissions supplementary state
  const [collaborators, setCollaborators] = useState<Collaborator[]>([])
  const [newUsername, setNewUsername] = useState('')
  const [newAccessLevel, setNewAccessLevel] = useState('co_author')

  const updateField = <K extends keyof ProblemFormState>(key: K, value: ProblemFormState[K]) => {
    setFormState(prev => ({ ...prev, [key]: value }))
  }

  const buildPayload = (overrides: Partial<ProblemFormState> = {}): Record<string, unknown> => {
    const merged = { ...formState, ...overrides }
    return {
      title: merged.title,
      description: merged.description,
      input_format: merged.inputFormat,
      output_format: merged.outputFormat,
      constraints: merged.constraints,
      hint: merged.hint,
      time_limit: Number(merged.timeLimit),
      memory_limit: Number(merged.memoryLimit),
      difficulty: merged.difficulty,
      rating: Number(merged.rating) || 0,
      tags: merged.tags.split(',').map(t => t.trim()).filter(t => t !== ''),
      sample_cases: merged.sampleCases,
      testcase_score: merged.testcases,
      spj: merged.checkerType === 'custom',
      spj_language: merged.spjLanguage,
      spj_source_code: merged.spjSourceCode,
      checker_type: merged.checkerType,
      float_epsilon: merged.floatEpsilon,
      interactive: merged.interactive,
      interactor_language: merged.interactorLanguage,
      interactor_source_code: merged.interactorSourceCode,
      // `visible` is deliberately NOT sent: the API derives it from the review
      // status and ignores a non-admin's value, so sending it only implied a
      // control the author does not have.
    }
  }

  const loadProblem = async () => {
    if (!slug) return
    try {
      const data = await api.problems.get(slug)
      setProblem(data)
      setFormState({
        title: data.title || '',
        description: data.description || '',
        inputFormat: data.input_format || '',
        outputFormat: data.output_format || '',
        constraints: data.constraints || '',
        hint: data.hint || '',
        timeLimit: data.time_limit || 1000,
        memoryLimit: data.memory_limit || 262144,
        difficulty: data.difficulty || 'easy',
        rating: data.rating ?? 0,
        tags: data.tags?.join(', ') || '',
        sampleCases: data.sample_cases || [],
        testcases: data.testcase_score || [],
        checkerType: data.checker_type || 'exact',
        floatEpsilon: data.float_epsilon ?? 1e-6,
        spj: data.spj || false,
        spjLanguage: data.spj_language || 'cpp-gpp-64',
        spjSourceCode: data.spj_source_code || '',
        interactive: data.interactive || false,
        interactorLanguage: data.interactor_language || 'cpp-gpp-64',
        interactorSourceCode: data.interactor_source_code || '',
      })

      const permData = await api.problems.getPermissions(data.slug)
      setCollaborators(permData.data || [])
    } catch (err) {
      setError(errorMessage(err) || 'Failed to load problem workspace')
    }
  }

  useEffect(() => {
    queueMicrotask(loadProblem)
  }, [slug]) // eslint-disable-line react-hooks/exhaustive-deps

  const loadTestcaseFiles = async (problemSlug: string) => {
    setTestcaseFilesLoading(true)
    try {
      const res = await api.problems.listTestcases(problemSlug)
      setTestcaseFiles(res.files || [])
    } catch (err) {
      setError(errorMessage(err) || 'Failed to load test data files')
    } finally {
      setTestcaseFilesLoading(false)
    }
  }

  // Loaded when the tab is shown rather than with the problem: every other tab
  // works without this listing, and it needs a round trip to the data dir.
  useEffect(() => {
    if (!slug || activeTab !== 'testcases') return
    queueMicrotask(() => loadTestcaseFiles(slug))
  }, [slug, activeTab])

  const handleSave = async (overrides?: Partial<ProblemFormState>) => {
    if (!problem) return
    setSaving(true)
    setError(null)
    setSuccess(null)
    try {
      const payload = buildPayload(overrides)
      await api.problems.update(problem.slug as string, payload)
      setSuccess('Problem saved successfully!')
      loadProblem()
    } catch (err) {
      setError(errorMessage(err) || 'Failed to save problem')
    } finally {
      setSaving(false)
    }
  }

  const handleSaveStatement = (e: React.FormEvent) => {
    e.preventDefault()
    handleSave()
  }

  const handleSaveChecker = () => {
    handleSave({ spj: formState.checkerType === 'custom' })
  }

  const handleAddTestCaseScore = async () => {
    if (!problem) return
    if (!newTestCase.input_name || !newTestCase.output_name) {
      setError('Input and Output file names are required')
      return
    }
    setError(null)
    setSuccess(null)
    const updated = [...formState.testcases, newTestCase]
    setFormState(prev => ({ ...prev, testcases: updated }))
    setNewTestCase({ input_name: '', output_name: '', score: 10 })
    try {
      await api.problems.update(problem.slug as string, buildPayload({ testcases: updated }))
      setSuccess('Testcase scores updated successfully!')
    } catch (err) {
      setError(errorMessage(err) || 'Failed to save testcase scores')
    }
  }

  const handleRemoveTestCaseScore = async (index: number) => {
    if (!problem) return
    setError(null)
    setSuccess(null)
    const updated = formState.testcases.filter((_, i) => i !== index)
    setFormState(prev => ({ ...prev, testcases: updated }))
    try {
      await api.problems.update(problem.slug as string, buildPayload({ testcases: updated }))
      setSuccess('Testcase score removed successfully!')
    } catch (err) {
      setError(errorMessage(err) || 'Failed to save testcase scores')
    }
  }

  const handleBatchSetScores = async () => {
    if (!problem) return
    if (formState.testcases.length === 0) {
      setError('No testcases registered to allocate scores')
      return
    }
    setError(null)
    setSuccess(null)
    setBatchApplying(true)
    const updated = formState.testcases.map(tc => ({ ...tc, score: batchScore }))
    setFormState(prev => ({ ...prev, testcases: updated }))
    try {
      await api.problems.update(problem.slug as string, buildPayload({ testcases: updated }))
      setSuccess(`All ${formState.testcases.length} testcase scores set to ${batchScore} points successfully!`)
    } catch (err) {
      setError(errorMessage(err) || 'Failed to update testcase scores')
    } finally {
      setBatchApplying(false)
    }
  }

  const handleUploadTestcases = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0 || !problem) return
    setError(null)
    setSuccess(null)
    const file = e.target.files[0]
    // Clear the input so re-uploading the same package fires another change event.
    e.target.value = ''
    try {
      await api.problems.uploadTestcases(problem.slug as string, file)
      setSuccess('Testcase package uploaded successfully!')
      loadProblem()
      loadTestcaseFiles(problem.slug as string)
    } catch (err) {
      setError(errorMessage(err) || 'Failed to upload testcase package')
    }
  }

  const handleLoadTestcaseFile = async (name: string): Promise<TestcaseFileContent | null> => {
    if (!problem) return null
    setError(null)
    setSuccess(null)
    try {
      return await api.problems.getTestcase(problem.slug as string, name)
    } catch (err) {
      setError(errorMessage(err) || `Failed to open ${name}`)
      return null
    }
  }

  const handleSaveTestcaseFile = async (name: string, content: string): Promise<boolean> => {
    if (!problem) return false
    setError(null)
    setSuccess(null)
    try {
      await api.problems.saveTestcase(problem.slug as string, name, content)
      setSuccess(`${name} saved.`)
      loadTestcaseFiles(problem.slug as string)
      return true
    } catch (err) {
      setError(errorMessage(err) || `Failed to save ${name}`)
      return false
    }
  }

  const handleCreateTestcasePair = async (pair: {
    stem: string
    input: string
    output: string
    score: number
  }): Promise<boolean> => {
    if (!problem) return false
    const stem = pair.stem.trim().replace(/\.(in|input|txt|out|output|ans|sol)$/i, '')
    if (!stem) {
      setError('Test case name is required')
      return false
    }
    const inputName = `${stem}.in`
    const outputName = `${stem}.out`
    setError(null)
    setSuccess(null)

    const updated = [
      ...formState.testcases.filter(tc => tc.input_name !== inputName && tc.output_name !== outputName),
      { input_name: inputName, output_name: outputName, score: Number(pair.score) || 0 },
    ]
    const slug = problem.slug as string
    try {
      await api.problems.saveTestcase(slug, inputName, pair.input)
      await api.problems.saveTestcase(slug, outputName, pair.output)
      setFormState(prev => ({ ...prev, testcases: updated }))
      await api.problems.update(slug, buildPayload({ testcases: updated }))
      setSuccess(`Test case ${stem} created with ${inputName} / ${outputName}.`)
      loadTestcaseFiles(slug)
      return true
    } catch (err) {
      setError(errorMessage(err) || 'Failed to create test case')
      return false
    }
  }

  const handleDeleteTestcaseFile = async (name: string) => {
    if (!problem) return
    if (!(await confirm({
      message: `Delete ${name} from the problem's test data? A score row that uses this file is removed too. This cannot be undone.`,
      confirmLabel: 'Delete file',
      variant: 'danger',
    }))) return
    setError(null)
    setSuccess(null)
    const slug = problem.slug as string
    const remaining = formState.testcases.filter(tc => tc.input_name !== name && tc.output_name !== name)
    try {
      await api.problems.deleteTestcase(slug, name)
      if (remaining.length !== formState.testcases.length) {
        setFormState(prev => ({ ...prev, testcases: remaining }))
        await api.problems.update(slug, buildPayload({ testcases: remaining }))
      }
      setSuccess(`${name} deleted.`)
      loadTestcaseFiles(slug)
    } catch (err) {
      setError(errorMessage(err) || `Failed to delete ${name}`)
    }
  }

  const handleDownloadTestcaseFile = async (name: string) => {
    if (!problem) return
    setError(null)
    try {
      await api.problems.downloadTestcase(problem.slug as string, name)
    } catch (err) {
      setError(errorMessage(err) || `Failed to download ${name}`)
    }
  }

  const handleDownloadAllTestcases = async () => {
    if (!problem) return
    setError(null)
    try {
      const resp = await fetch(api.problems.exportProblemUrl(problem.slug as string), {
        headers: { Authorization: `Bearer ${getAccessToken()}` },
      })
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const blob = await resp.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${problem.slug}.zip`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(errorMessage(err) || 'Failed to download test data')
    }
  }

  const handleUpdateTestCaseScore = async (index: number, score: number) => {
    if (!problem) return
    setError(null)
    setSuccess(null)
    const updated = formState.testcases.map((tc, i) => (i === index ? { ...tc, score } : tc))
    setFormState(prev => ({ ...prev, testcases: updated }))
    try {
      await api.problems.update(problem.slug as string, buildPayload({ testcases: updated }))
    } catch (err) {
      setError(errorMessage(err) || 'Failed to save testcase scores')
    }
  }

  const handleImageUpload = async (file: File) => {
    if (!problem) return
    setUploadingImage(true)
    setError(null)
    setSuccess(null)
    try {
      const formData = new FormData()
      formData.append('image', file)
      const headers: Record<string, string> = {}
      const token = getAccessToken()
      if (token) headers['Authorization'] = `Bearer ${token}`
      const res = await fetch(`/api/problems/${problem.slug}/media`, {
        method: 'POST',
        headers,
        body: formData,
      })
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `HTTP ${res.status}`)
      }
      const data = await res.json()
      const imageUrl = data.url as string
      setFormState(prev => ({
        ...prev,
        description: prev.description ? `${prev.description}\n\n![Image](${imageUrl})` : `![Image](${imageUrl})`
      }))
      setSuccess('Image uploaded and inserted into description!')
    } catch (err) {
      setError(errorMessage(err) || 'Failed to upload image')
    } finally {
      setUploadingImage(false)
    }
  }

  const handleAddCollaborator = async () => {
    if (!newUsername.trim() || !problem) return
    setError(null)
    setSuccess(null)
    try {
      await api.problems.addPermission(problem.slug as string, newUsername, newAccessLevel)
      setSuccess(`Collaborator ${newUsername} added successfully!`)
      setNewUsername('')
      loadProblem()
    } catch (err) {
      setError(errorMessage(err) || 'Failed to add collaborator')
    }
  }

  const handleRemoveCollaborator = async (userId: string) => {
    if (!problem) return
    setError(null)
    setSuccess(null)
    try {
      await api.problems.removePermission(problem.slug as string, userId)
      setSuccess('Collaborator removed successfully!')
      loadProblem()
    } catch (err) {
      setError(errorMessage(err) || 'Failed to remove collaborator')
    }
  }

  const handleDeleteProblem = async () => {
    if (!problem) return
    if (!(await confirm({
      message: 'Are you absolutely sure you want to delete this problem? This action cannot be undone.',
      confirmLabel: 'Delete Problem',
      variant: 'danger',
    }))) return
    setError(null)
    try {
      await api.problems.delete(problem.slug as string)
      navigate('/setter')
    } catch (err) {
      setError(errorMessage(err) || 'Failed to delete problem')
    }
  }

  // Publishing is an admin decision. The author's action is to submit for
  // review; the old handler saved `visible`, which the API now ignores for
  // non-admins, so the toggle silently did nothing.
  const handleSubmitForReview = async () => {
    if (!problem?.slug) return
    setSaving(true)
    setError(null)
    try {
      await api.problems.submitForReview(problem.slug as string)
      const data = await api.problems.get(problem.slug as string)
      setProblem(data)
    } catch (err) {
      setError(errorMessage(err) || 'Failed to submit for review')
    } finally {
      setSaving(false)
    }
  }

  if (!problem) {
    if (error) {
      return (
        <div className="text-center py-20 space-y-4">
          <p className="text-bad text-sm font-semibold">{error}</p>
          <button
            type="button"
            onClick={() => loadProblem()}
            className="bg-brand text-on-brand px-4 py-2 rounded text-sm font-semibold hover:bg-brand-hover transition-colors cursor-pointer"
          >
            Retry
          </button>
        </div>
      )
    }
    return <div className="text-center py-20 text-muted-soft">Loading problem workspace...</div>
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Problem workspace"
        title={problem.title as string}
        description="Statement, tests, checker, permissions and editorial for this problem."
        actions={
          <>
            <Link to={`/problems/${problem.slug as string}`} className={buttonClass({ variant: 'secondary', size: 'sm' })}>
              View public
            </Link>
            {/* `bg-surface-3 … hover:bg-surface-3` hovered to its own colour. */}
            <Link to="/setter" className={buttonClass({ variant: 'ghost', size: 'sm' })}>
              Back to workspace
            </Link>
          </>
        }
      />

      {/* Status messages */}
      {error && (
        <div role="alert" className="rounded-lg border border-bad/30 bg-bad/10 p-4 text-sm text-bad">
          {error}
        </div>
      )}
      {success && (
        <div className="bg-ok/10 border-l-4 border-ok/50 p-4 text-sm text-ok rounded-r">
          {success}
        </div>
      )}

      {/* Main content. Stacks under lg — a 14rem tab rail beside the panel left
          ~150px for the editor on a 390px phone. On mobile the rail scrolls
          horizontally so every tab stays reachable. */}
      <div className="flex flex-col lg:flex-row gap-6 items-start">
        {/* Tabs Sidebar */}
        <div className="w-full lg:w-56 lg:shrink-0 flex lg:flex-col flex-row overflow-x-auto lg:overflow-hidden border border-hairline rounded-lg bg-surface text-sm">
          <button
            onClick={() => setActiveTab('statement')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap border-r lg:border-r-0 lg:border-b border-hairline font-medium ${activeTab === 'statement' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Statement & Details
          </button>
          <button
            onClick={() => setActiveTab('testcases')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap border-r lg:border-r-0 lg:border-b border-hairline font-medium ${activeTab === 'testcases' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Test Cases / Data
          </button>
          <button
            onClick={() => setActiveTab('checker')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap border-r lg:border-r-0 lg:border-b border-hairline font-medium ${activeTab === 'checker' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Checker / Special Judge
          </button>
          <button
            onClick={() => setActiveTab('submissions')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap border-r lg:border-r-0 lg:border-b border-hairline font-medium ${activeTab === 'submissions' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Submissions
          </button>
          <button
            onClick={() => setActiveTab('permissions')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap border-r lg:border-r-0 lg:border-b border-hairline font-medium ${activeTab === 'permissions' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Collaborators
          </button>
          <button
            onClick={() => setActiveTab('settings')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap border-r lg:border-r-0 lg:border-b border-hairline font-medium ${activeTab === 'settings' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Workspace Settings
          </button>
          <button
            onClick={() => setActiveTab('editorial')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap font-medium ${activeTab === 'editorial' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Editorials
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('translations')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap font-medium ${activeTab === 'translations' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            Translations
          </button>
          {(role === 'admin' || role === 'setter') && <button
            type="button"
            onClick={() => setActiveTab('ai')}
            className={`px-4 py-3 text-left shrink-0 whitespace-nowrap font-medium ${activeTab === 'ai' ? 'bg-brand-soft text-brand border-l-4 border-l-blue-600' : 'text-muted hover:bg-surface-2 hover:text-ink dark:hover:text-white'}`}
          >
            AI Studio
          </button>}
        </div>

        {/* Tab Content Panels */}
        <div className="flex-1 bg-surface border border-hairline rounded-lg p-6 min-h-[500px]">
          {activeTab === 'statement' && (
            <StatementTab
              formState={formState}
              saving={saving}
              uploadingImage={uploadingImage}
              onUpdate={updateField}
              onImageUpload={handleImageUpload}
              onSave={handleSaveStatement}
            />
          )}

          {activeTab === 'testcases' && (
            <TestCasesTab
              testcases={formState.testcases}
              files={testcaseFiles}
              filesLoading={testcaseFilesLoading}
              newTestCase={newTestCase}
              batchScore={batchScore}
              batchApplying={batchApplying}
              onAdd={handleAddTestCaseScore}
              onRemove={handleRemoveTestCaseScore}
              onScoreChange={handleUpdateTestCaseScore}
              onBatchSet={handleBatchSetScores}
              onUpload={handleUploadTestcases}
              onNewTestCaseChange={setNewTestCase}
              onBatchScoreChange={setBatchScore}
              onReloadFiles={() => problem && loadTestcaseFiles(problem.slug as string)}
              onLoadFile={handleLoadTestcaseFile}
              onSaveFile={handleSaveTestcaseFile}
              onCreatePair={handleCreateTestcasePair}
              onDeleteFile={handleDeleteTestcaseFile}
              onDownloadFile={handleDownloadTestcaseFile}
              onDownloadAll={handleDownloadAllTestcases}
            />
          )}

          {activeTab === 'checker' && (
            <CheckerTab
              formState={formState}
              saving={saving}
              onUpdate={updateField}
              onSave={handleSaveChecker}
            />
          )}

          {activeTab === 'submissions' && (
            <SubmissionsTab problemSlug={problem.slug as string} />
          )}

          {activeTab === 'permissions' && (
            <PermissionsTab
              collaborators={collaborators}
              newUsername={newUsername}
              newAccessLevel={newAccessLevel}
              onAdd={handleAddCollaborator}
              onRemove={handleRemoveCollaborator}
              onNewUsernameChange={setNewUsername}
              onNewAccessLevelChange={setNewAccessLevel}
            />
          )}

          {activeTab === 'settings' && (
            <SettingsTab
              status={problem.status || 'draft'}
              reviewNote={problem.review_note}
              visible={problem.visible ?? false}
              saving={saving}
              onSubmitForReview={handleSubmitForReview}
              onDelete={handleDeleteProblem}
            />
          )}

          {activeTab === 'editorial' && (
            <EditorialTab
              problemId={problem.id}
              isUserAdmin={decodeRole() === 'admin'}
            />
          )}
          {activeTab === 'translations' && (
            <TranslationsTab
              problemId={problem.id}
              defaultTitle={problem.title as string}
              defaultDescription={problem.description as string}
            />
          )}
          {activeTab === 'ai' && (
            <AIStudioTab
              problemId={problem.id}
              onApplied={() => loadProblem()}
            />
          )}
        </div>
      </div>
    </div>
  )
}
