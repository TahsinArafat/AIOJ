import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import type { ProblemFormState, TestCase, Collaborator } from '../types/problem-workspace'
import StatementTab from '../components/SetterWorkspace/StatementTab'
import TestCasesTab from '../components/SetterWorkspace/TestCasesTab'
import CheckerTab from '../components/SetterWorkspace/CheckerTab'
import PermissionsTab from '../components/SetterWorkspace/PermissionsTab'
import SettingsTab from '../components/SetterWorkspace/SettingsTab'

export default function SetterProblemWorkspace() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const [problem, setProblem] = useState<any>(null)
  const [activeTab, setActiveTab] = useState<'statement' | 'testcases' | 'checker' | 'permissions' | 'settings'>('statement')
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
    hint: '',
    timeLimit: 1000,
    memoryLimit: 262144,
    difficulty: 'easy',
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
    visible: true,
  })

  // Test Cases supplementary state
  const [newTestCase, setNewTestCase] = useState<TestCase>({ input_name: '', output_name: '', score: 10 })
  const [batchScore, setBatchScore] = useState<number>(10)
  const [batchApplying, setBatchApplying] = useState(false)

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
      hint: merged.hint,
      time_limit: Number(merged.timeLimit),
      memory_limit: Number(merged.memoryLimit),
      difficulty: merged.difficulty,
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
      visible: merged.visible,
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
        hint: data.hint || '',
        timeLimit: data.time_limit || 1000,
        memoryLimit: data.memory_limit || 262144,
        difficulty: data.difficulty || 'easy',
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
        visible: data.visible !== false,
      })

      const permData = await api.problems.getPermissions(data.slug)
      setCollaborators(permData.data || [])
    } catch (err: any) {
      setError(err.message || 'Failed to load problem workspace')
    }
  }

  useEffect(() => {
    loadProblem()
  }, [slug])

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
    } catch (err: any) {
      setError(err.message || 'Failed to save problem')
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
    } catch (err: any) {
      setError(err.message || 'Failed to save testcase scores')
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
    } catch (err: any) {
      setError(err.message || 'Failed to save testcase scores')
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
    } catch (err: any) {
      setError(err.message || 'Failed to update testcase scores')
    } finally {
      setBatchApplying(false)
    }
  }

  const handleUploadTestcases = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0 || !problem) return
    setError(null)
    setSuccess(null)
    const file = e.target.files[0]
    try {
      await api.problems.uploadTestcases(problem.slug as string, file)
      setSuccess('Testcase package uploaded successfully!')
      loadProblem()
    } catch (err: any) {
      setError(err.message || 'Failed to upload testcase package')
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
    } catch (err: any) {
      setError(err.message || 'Failed to upload image')
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
    } catch (err: any) {
      setError(err.message || 'Failed to add collaborator')
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
    } catch (err: any) {
      setError(err.message || 'Failed to remove collaborator')
    }
  }

  const handleDeleteProblem = async () => {
    if (!problem) return
    if (!window.confirm('Are you absolutely sure you want to delete this problem? This action CANNOT be undone.')) return
    setError(null)
    try {
      await api.problems.delete(problem.slug as string)
      navigate('/setter')
    } catch (err: any) {
      setError(err.message || 'Failed to delete problem')
    }
  }

  const handleUpdateVisibility = () => {
    handleSave({ visible: formState.visible })
  }

  if (!problem) {
    return <div className="text-center py-20 text-gray-400">Loading problem workspace...</div>
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-gray-200 pb-4">
        <div>
          <span className="text-xs text-gray-400 uppercase tracking-wider font-semibold">Polygon Workspace</span>
          <h1 className="text-2xl font-bold text-gray-900">{problem.title as string}</h1>
        </div>
        <div className="flex gap-3">
          <Link
            to={`/problems/${problem.slug as string}`}
            className="border border-gray-300 text-gray-700 px-4 py-2 rounded text-sm hover:bg-gray-50 transition-colors"
          >
            View Public
          </Link>
          <Link
            to="/setter"
            className="bg-gray-100 border border-gray-200 text-gray-700 px-4 py-2 rounded text-sm hover:bg-gray-200 transition-colors"
          >
            Back to Setter Workspace
          </Link>
        </div>
      </div>

      {/* Status messages */}
      {error && (
        <div className="bg-red-50 border-l-4 border-red-500 p-4 text-sm text-red-700 rounded-r">
          {error}
        </div>
      )}
      {success && (
        <div className="bg-green-50 border-l-4 border-green-500 p-4 text-sm text-green-700 rounded-r">
          {success}
        </div>
      )}

      {/* Main content */}
      <div className="flex gap-6 items-start">
        {/* Tabs Sidebar */}
        <div className="w-56 shrink-0 flex flex-col border border-gray-200 rounded-lg bg-white overflow-hidden text-sm">
          <button
            onClick={() => setActiveTab('statement')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'statement' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Statement & Details
          </button>
          <button
            onClick={() => setActiveTab('testcases')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'testcases' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Test Cases / Data
          </button>
          <button
            onClick={() => setActiveTab('checker')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'checker' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Checker / Special Judge
          </button>
          <button
            onClick={() => setActiveTab('permissions')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'permissions' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Collaborators
          </button>
          <button
            onClick={() => setActiveTab('settings')}
            className={`px-4 py-3 text-left font-medium ${activeTab === 'settings' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Workspace Settings
          </button>
        </div>

        {/* Tab Content Panels */}
        <div className="flex-1 bg-white border border-gray-200 rounded-lg p-6 min-h-[500px]">
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
              newTestCase={newTestCase}
              batchScore={batchScore}
              batchApplying={batchApplying}
              onAdd={handleAddTestCaseScore}
              onRemove={handleRemoveTestCaseScore}
              onBatchSet={handleBatchSetScores}
              onUpload={handleUploadTestcases}
              onNewTestCaseChange={setNewTestCase}
              onBatchScoreChange={setBatchScore}
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
              visible={formState.visible}
              saving={saving}
              onUpdateVisibility={handleUpdateVisibility}
              onToggleVisibility={(v) => updateField('visible', v)}
              onDelete={handleDeleteProblem}
            />
          )}
        </div>
      </div>
    </div>
  )
}
