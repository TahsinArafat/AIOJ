import { useEffect, useState, useRef } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import CodeEditor from '../components/CodeEditor'

interface TestCase {
    input_name: string
    output_name: string
    score: number
}

interface Collaborator {
    problem_id: string
    user_id: string
    username: string
    access_level: string
}

export default function SetterProblemWorkspace() {
    const { slug } = useParams<{ slug: string }>()
    const navigate = useNavigate()
    const [problem, setProblem] = useState<any>(null)
    const [activeTab, setActiveTab] = useState<'statement' | 'testcases' | 'checker' | 'permissions' | 'settings'>('statement')
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState<string | null>(null)

    // Statement Form State
    const [title, setTitle] = useState('')
    const [description, setDescription] = useState('')
    const [inputFormat, setInputFormat] = useState('')
    const [outputFormat, setOutputFormat] = useState('')
    const [hint, setHint] = useState('')
    const [timeLimit, setTimeLimit] = useState(1000)
    const [memoryLimit, setMemoryLimit] = useState(262144)
    const [difficulty, setDifficulty] = useState('easy')
    const [tags, setTags] = useState('')

    // Sample Cases State
    const [sampleCases, setSampleCases] = useState<{input: string; output: string; explanation: string}[]>([])

    // Test Cases State
    const [testcases, setTestcases] = useState<TestCase[]>([])
    const [newTestCase, setNewTestCase] = useState<TestCase>({ input_name: '', output_name: '', score: 10 })
    const [batchScore, setBatchScore] = useState<number>(10)
    const [batchApplying, setBatchApplying] = useState(false)
    const fileInputRef = useRef<HTMLInputElement>(null)

    // Checker/SPJ State
    const [checkerType, setCheckerType] = useState('exact')
    const [floatEpsilon, setFloatEpsilon] = useState(1e-6)
    const [spj, setSpj] = useState(false)
    const [spjLanguage, setSpjLanguage] = useState('cpp-gpp-64')
    const [spjSourceCode, setSpjSourceCode] = useState('')

    // Interactive Problem State
    const [interactive, setInteractive] = useState(false)
    const [interactorLanguage, setInteractorLanguage] = useState('cpp-gpp-64')
    const [interactorSourceCode, setInteractorSourceCode] = useState('')

    // Permissions State
    const [collaborators, setCollaborators] = useState<Collaborator[]>([])
    const [newUsername, setNewUsername] = useState('')
    const [newAccessLevel, setNewAccessLevel] = useState('co_author')

    // Settings State
    const [visible, setVisible] = useState(true)

    const loadProblem = async () => {
        if (!slug) return
        try {
            const data = await api.problems.get(slug)
            setProblem(data)
            setTitle(data.title || '')
            setDescription(data.description || '')
            setInputFormat(data.input_format || '')
            setOutputFormat(data.output_format || '')
            setHint(data.hint || '')
            setTimeLimit(data.time_limit || 1000)
            setMemoryLimit(data.memory_limit || 262144)
            setDifficulty(data.difficulty || 'easy')
            setTags(data.tags?.join(', ') || '')
            setSampleCases(data.sample_cases || [])
            setTestcases(data.testcase_score || [])
            setCheckerType(data.checker_type || 'exact')
            setFloatEpsilon(data.float_epsilon || 1e-6)
            setSpj(data.spj || false)
            setSpjLanguage(data.spj_language || 'cpp-gpp-64')
            setSpjSourceCode(data.spj_source_code || '')
            setInteractive(data.interactive || false)
            setInteractorLanguage(data.interactor_language || 'cpp-gpp-64')
            setInteractorSourceCode(data.interactor_source_code || '')
            setVisible(data.visible !== false)

            // Load permissions
            const permData = await api.problems.getPermissions(data.slug)
            setCollaborators(permData.data || [])
        } catch (err: any) {
            setError(err.message || 'Failed to load problem workspace')
        }
    }

    useEffect(() => {
        loadProblem()
    }, [slug])

    const handleSaveStatement = async (e: React.FormEvent) => {
        e.preventDefault()
        setSaving(true)
        setError(null)
        setSuccess(null)
        try {
            const tagsArray = tags.split(',').map(t => t.trim()).filter(t => t !== '')
            const payload = {
                title,
                description,
                input_format: inputFormat,
                output_format: outputFormat,
                hint,
                time_limit: Number(timeLimit),
                memory_limit: Number(memoryLimit),
                difficulty,
                tags: tagsArray,
                sample_cases: sampleCases,
                testcase_score: testcases,
                spj,
                spj_language: spjLanguage,
                spj_source_code: spjSourceCode,
                checker_type: checkerType,
                float_epsilon: floatEpsilon,
                interactive,
                interactor_language: interactorLanguage,
                interactor_source_code: interactorSourceCode,
                visible
            }
            await api.problems.update(problem.slug, payload)
            setSuccess('Problem statement saved successfully!')
            loadProblem()
        } catch (err: any) {
            setError(err.message || 'Failed to save problem')
        } finally {
            setSaving(false)
        }
    }

    const handleUploadTestcases = async (e: React.ChangeEvent<HTMLInputElement>) => {
        if (!e.target.files || e.target.files.length === 0) return
        setError(null)
        setSuccess(null)
        const file = e.target.files[0]
        try {
            await api.problems.uploadTestcases(problem.slug, file)
            setSuccess('Testcase package uploaded successfully!')
            loadProblem()
        } catch (err: any) {
            setError(err.message || 'Failed to upload testcase package')
        }
    }

    const handleAddTestCaseScore = async () => {
        if (!newTestCase.input_name || !newTestCase.output_name) {
            setError('Input and Output file names are required')
            return
        }
        setError(null)
        setSuccess(null)
        const updated = [...testcases, newTestCase]
        setTestcases(updated)
        setNewTestCase({ input_name: '', output_name: '', score: 10 })
        try {
            const tagsArray = tags.split(',').map(t => t.trim()).filter(t => t !== '')
            const payload = {
                title,
                description,
                input_format: inputFormat,
                output_format: outputFormat,
                hint,
                time_limit: Number(timeLimit),
                memory_limit: Number(memoryLimit),
                difficulty,
                tags: tagsArray,
                sample_cases: sampleCases,
                testcase_score: updated,
                spj,
                spj_language: spjLanguage,
                spj_source_code: spjSourceCode,
                checker_type: checkerType,
                float_epsilon: floatEpsilon,
                interactive,
                interactor_language: interactorLanguage,
                interactor_source_code: interactorSourceCode,
                visible
            }
            await api.problems.update(problem.slug, payload)
            setSuccess('Testcase scores updated successfully!')
        } catch (err: any) {
            setError(err.message || 'Failed to save testcase scores')
        }
    }

    const handleRemoveTestCaseScore = async (index: number) => {
        setError(null)
        setSuccess(null)
        const updated = testcases.filter((_, i) => i !== index)
        setTestcases(updated)
        try {
            const tagsArray = tags.split(',').map(t => t.trim()).filter(t => t !== '')
            const payload = {
                title,
                description,
                input_format: inputFormat,
                output_format: outputFormat,
                hint,
                time_limit: Number(timeLimit),
                memory_limit: Number(memoryLimit),
                difficulty,
                tags: tagsArray,
                sample_cases: sampleCases,
                testcase_score: updated,
                spj,
                spj_language: spjLanguage,
                spj_source_code: spjSourceCode,
                checker_type: checkerType,
                float_epsilon: floatEpsilon,
                interactive,
                interactor_language: interactorLanguage,
                interactor_source_code: interactorSourceCode,
                visible
            }
            await api.problems.update(problem.slug, payload)
            setSuccess('Testcase score removed successfully!')
        } catch (err: any) {
            setError(err.message || 'Failed to save testcase scores')
        }
    }

    const handleBatchSetScores = async () => {
        if (testcases.length === 0) {
            setError('No testcases registered to allocate scores')
            return
        }
        setError(null)
        setSuccess(null)
        setBatchApplying(true)
        const updated = testcases.map(tc => ({ ...tc, score: batchScore }))
        setTestcases(updated)
        try {
            const tagsArray = tags.split(',').map(t => t.trim()).filter(t => t !== '')
            const payload = {
                title,
                description,
                input_format: inputFormat,
                output_format: outputFormat,
                hint,
                time_limit: Number(timeLimit),
                memory_limit: Number(memoryLimit),
                difficulty,
                tags: tagsArray,
                sample_cases: sampleCases,
                testcase_score: updated,
                spj,
                spj_language: spjLanguage,
                spj_source_code: spjSourceCode,
                checker_type: checkerType,
                float_epsilon: floatEpsilon,
                interactive,
                interactor_language: interactorLanguage,
                interactor_source_code: interactorSourceCode,
                visible
            }
            await api.problems.update(problem.slug, payload)
            setSuccess(`All ${testcases.length} testcase scores set to ${batchScore} points successfully!`)
        } catch (err: any) {
            setError(err.message || 'Failed to update testcase scores')
        } finally {
            setBatchApplying(false)
        }
    }

    const handleSaveChecker = async () => {
        setSaving(true)
        setError(null)
        setSuccess(null)
        try {
            const tagsArray = tags.split(',').map(t => t.trim()).filter(t => t !== '')
            const payload = {
                title,
                description,
                input_format: inputFormat,
                output_format: outputFormat,
                hint,
                time_limit: Number(timeLimit),
                memory_limit: Number(memoryLimit),
                difficulty,
                tags: tagsArray,
                sample_cases: sampleCases,
                testcase_score: testcases,
                spj: checkerType === 'custom',
                spj_language: spjLanguage,
                spj_source_code: spjSourceCode,
                checker_type: checkerType,
                float_epsilon: floatEpsilon,
                interactive,
                interactor_language: interactorLanguage,
                interactor_source_code: interactorSourceCode,
                visible
            }
            await api.problems.update(problem.slug, payload)
            setSuccess('Checker configuration saved successfully!')
            loadProblem()
        } catch (err: any) {
            setError(err.message || 'Failed to save checker configuration')
        } finally {
            setSaving(false)
        }
    }

    const handleAddCollaborator = async () => {
        if (!newUsername.trim()) return
        setError(null)
        setSuccess(null)
        try {
            // Find user ID by username or call endpoint
            await api.problems.addPermission(problem.slug, newUsername, newAccessLevel)
            setSuccess(`Collaborator ${newUsername} added successfully!`)
            setNewUsername('')
            loadProblem()
        } catch (err: any) {
            setError(err.message || 'Failed to add collaborator')
        }
    }

    const handleRemoveCollaborator = async (userId: string) => {
        setError(null)
        setSuccess(null)
        try {
            await api.problems.removePermission(problem.slug, userId)
            setSuccess('Collaborator removed successfully!')
            loadProblem()
        } catch (err: any) {
            setError(err.message || 'Failed to remove collaborator')
        }
    }

    const handleDeleteProblem = async () => {
        if (!window.confirm('Are you absolutely sure you want to delete this problem? This action CANNOT be undone.')) return
        setError(null)
        try {
            await api.problems.delete(problem.slug)
            navigate('/setter')
        } catch (err: any) {
            setError(err.message || 'Failed to delete problem')
        }
    }

    if (!problem) {
        return <div className="text-center py-20 text-gray-400">Loading problem workspace...</div>
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between border-b border-gray-200 pb-4">
                <div>
                    <span className="text-xs text-gray-400 uppercase tracking-wider">Polygon Workspace</span>
                    <h1 className="text-2xl font-bold text-gray-900">{problem.title}</h1>
                </div>
                <div className="flex gap-3">
                    <Link 
                        to={`/problems/${problem.slug}`} 
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
                        <form onSubmit={handleSaveStatement} className="space-y-4">
                            <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Edit Problem Statement</h2>
                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Title</label>
                                    <input 
                                        type="text" 
                                        value={title} 
                                        onChange={e => setTitle(e.target.value)} 
                                        className="w-full border rounded px-3 py-1.5 text-sm"
                                        required 
                                    />
                                </div>
                                <div>
                                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Tags (comma-separated)</label>
                                    <input 
                                        type="text" 
                                        value={tags} 
                                        onChange={e => setTags(e.target.value)} 
                                        className="w-full border rounded px-3 py-1.5 text-sm" 
                                    />
                                </div>
                            </div>

                            <div className="grid grid-cols-3 gap-4">
                                <div>
                                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Difficulty</label>
                                    <select 
                                        value={difficulty} 
                                        onChange={e => setDifficulty(e.target.value)} 
                                        className="w-full border rounded px-3 py-1.5 text-sm"
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
                                        value={timeLimit} 
                                        onChange={e => setTimeLimit(Number(e.target.value))} 
                                        className="w-full border rounded px-3 py-1.5 text-sm" 
                                    />
                                </div>
                                <div>
                                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Memory Limit (KB)</label>
                                    <input 
                                        type="number" 
                                        value={memoryLimit} 
                                        onChange={e => setMemoryLimit(Number(e.target.value))} 
                                        className="w-full border rounded px-3 py-1.5 text-sm" 
                                    />
                                </div>
                            </div>

                            <div>
                                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Description (Markdown Supported)</label>
                                <textarea 
                                    value={description} 
                                    onChange={e => setDescription(e.target.value)} 
                                    rows={8} 
                                    className="w-full border rounded px-3 py-1.5 text-sm font-mono"
                                    required
                                />
                            </div>

                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Input Format</label>
                                    <textarea 
                                        value={inputFormat} 
                                        onChange={e => setInputFormat(e.target.value)} 
                                        rows={4} 
                                        className="w-full border rounded px-3 py-1.5 text-sm" 
                                    />
                                </div>
                                <div>
                                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Output Format</label>
                                    <textarea 
                                        value={outputFormat} 
                                        onChange={e => setOutputFormat(e.target.value)} 
                                        rows={4} 
                                        className="w-full border rounded px-3 py-1.5 text-sm" 
                                    />
                                </div>
                            </div>

                            <div>
                                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Hint</label>
                                <textarea 
                                    value={hint} 
                                    onChange={e => setHint(e.target.value)} 
                                    rows={2} 
                                    className="w-full border rounded px-3 py-1.5 text-sm" 
                                />
                            </div>

                            <div className="space-y-4">
                                <div className="flex items-center justify-between">
                                    <h3 className="font-semibold text-sm text-gray-700">Sample Cases</h3>
                                    <button
                                        type="button"
                                        onClick={() => setSampleCases([...sampleCases, {input: '', output: '', explanation: ''}])}
                                        className="bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700 transition-colors cursor-pointer"
                                    >
                                        + Add Sample
                                    </button>
                                </div>

                                {sampleCases.map((sc, i) => (
                                    <div key={i} className="border border-gray-200 rounded-lg p-4 space-y-3">
                                        <div className="flex items-center justify-between">
                                            <span className="font-medium text-sm">Sample {i + 1}</span>
                                            <button
                                                type="button"
                                                onClick={() => setSampleCases(sampleCases.filter((_, j) => j !== i))}
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
                                                        const newCases = [...sampleCases]
                                                        newCases[i].input = e.target.value
                                                        setSampleCases(newCases)
                                                    }}
                                                    rows={3}
                                                    className="w-full font-mono text-xs border border-gray-300 rounded p-2"
                                                />
                                            </div>
                                            <div>
                                                <label className="block text-xs font-medium text-gray-500 mb-1">Expected Output</label>
                                                <textarea
                                                    value={sc.output}
                                                    onChange={e => {
                                                        const newCases = [...sampleCases]
                                                        newCases[i].output = e.target.value
                                                        setSampleCases(newCases)
                                                    }}
                                                    rows={3}
                                                    className="w-full font-mono text-xs border border-gray-300 rounded p-2"
                                                />
                                            </div>
                                        </div>
                                        <div>
                                            <label className="block text-xs font-medium text-gray-500 mb-1">Explanation (optional)</label>
                                            <input
                                                type="text"
                                                value={sc.explanation}
                                                onChange={e => {
                                                    const newCases = [...sampleCases]
                                                    newCases[i].explanation = e.target.value
                                                    setSampleCases(newCases)
                                                }}
                                                className="w-full text-sm border border-gray-300 rounded px-2 py-1.5"
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
                    )}

                    {activeTab === 'testcases' && (
                        <div className="space-y-6">
                            <div>
                                <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Upload Testcase Package (ZIP)</h2>
                                <p className="text-xs text-gray-400 mb-3">Upload a ZIP package containing input and output testcases (e.g. `01.in` and `01.out`). Files will be parsed and loaded into the problem storage directory on the sandbox filesystem.</p>
                                <div className="flex gap-3 items-center">
                                    <input 
                                        type="file" 
                                        ref={fileInputRef} 
                                        onChange={handleUploadTestcases} 
                                        accept=".zip" 
                                        className="hidden" 
                                    />
                                    <button 
                                        onClick={() => fileInputRef.current?.click()} 
                                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors cursor-pointer"
                                    >
                                        Choose ZIP file
                                    </button>
                                </div>
                            </div>

                            <hr className="border-gray-200" />

                            <div>
                                <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Testcase Scores & Breakdown</h2>
                                <div className="border border-gray-200 rounded-lg overflow-hidden mb-4">
                                    <table className="w-full text-sm text-left">
                                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase font-semibold">
                                            <tr>
                                                <th className="px-4 py-2.5">Input File</th>
                                                <th className="px-4 py-2.5">Output File</th>
                                                <th className="px-4 py-2.5">Score</th>
                                                <th className="px-4 py-2.5 text-right">Action</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-gray-100">
                                            {testcases.map((tc, idx) => (
                                                <tr key={idx}>
                                                    <td className="px-4 py-2.5 font-mono text-xs">{tc.input_name}</td>
                                                    <td className="px-4 py-2.5 font-mono text-xs">{tc.output_name}</td>
                                                    <td className="px-4 py-2.5">{tc.score} pts</td>
                                                    <td className="px-4 py-2.5 text-right">
                                                        <button 
                                                            onClick={() => handleRemoveTestCaseScore(idx)} 
                                                            className="text-red-600 hover:underline cursor-pointer"
                                                        >
                                                            Remove
                                                        </button>
                                                    </td>
                                                </tr>
                                            ))}
                                            {testcases.length === 0 && (
                                                <tr>
                                                    <td colSpan={4} className="px-4 py-6 text-center text-gray-400 text-xs">
                                                        No testcase scores registered. Add testcase scores below to match files.
                                                    </td>
                                                </tr>
                                            )}
                                        </tbody>
                                    </table>
                                </div>

                                {testcases.length > 0 && (
                                    <div className="bg-gray-50 rounded-lg p-4 border border-gray-200 mb-4 flex items-end justify-between">
                                        <div className="flex gap-4 items-end">
                                            <div>
                                                <label className="block text-xs text-gray-500 mb-1">Set Score for All Test Cases</label>
                                                <input 
                                                    type="number" 
                                                    value={batchScore} 
                                                    onChange={e => setBatchScore(Number(e.target.value))} 
                                                    className="border rounded px-3 py-1 text-xs w-28" 
                                                />
                                            </div>
                                            <button 
                                                onClick={handleBatchSetScores}
                                                disabled={batchApplying}
                                                className="bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
                                            >
                                                {batchApplying ? 'Applying...' : 'Apply to All'}
                                            </button>
                                        </div>
                                        <span className="text-xs text-gray-400 font-medium">
                                            Total Testcases: {testcases.length} | Total Points: {testcases.reduce((sum, tc) => sum + (tc.score || 0), 0)} pts
                                        </span>
                                    </div>
                                )}

                                <div className="bg-gray-50 rounded-lg p-4 border border-gray-200 space-y-4">
                                    <h4 className="font-semibold text-xs text-gray-500 uppercase tracking-wider">Register TestCase File Matches</h4>
                                    <div className="grid grid-cols-3 gap-3">
                                        <div>
                                            <label className="block text-xs text-gray-500 mb-1">Input Name</label>
                                            <input 
                                                type="text" 
                                                placeholder="e.g. 01.in" 
                                                value={newTestCase.input_name} 
                                                onChange={e => setNewTestCase({...newTestCase, input_name: e.target.value})} 
                                                className="w-full border rounded px-3 py-1.5 text-xs font-mono" 
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-xs text-gray-500 mb-1">Output Name</label>
                                            <input 
                                                type="text" 
                                                placeholder="e.g. 01.out" 
                                                value={newTestCase.output_name} 
                                                onChange={e => setNewTestCase({...newTestCase, output_name: e.target.value})} 
                                                className="w-full border rounded px-3 py-1.5 text-xs font-mono" 
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-xs text-gray-500 mb-1">Score</label>
                                            <input 
                                                type="number" 
                                                value={newTestCase.score} 
                                                onChange={e => setNewTestCase({...newTestCase, score: Number(e.target.value)})} 
                                                className="w-full border rounded px-3 py-1.5 text-xs" 
                                            />
                                        </div>
                                    </div>
                                    <button 
                                        onClick={handleAddTestCaseScore} 
                                        className="bg-gray-800 text-white px-3 py-1.5 rounded text-xs hover:bg-black transition-colors cursor-pointer"
                                    >
                                        Add File Match
                                    </button>
                                </div>
                            </div>
                        </div>
                    )}

                    {activeTab === 'checker' && (
                        <div className="space-y-4">
                            <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Checker & Special Judge Configuration</h2>
                            <div>
                                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Checker Method</label>
                                <select 
                                    value={checkerType} 
                                    onChange={e => setCheckerType(e.target.value)} 
                                    className="border rounded px-3 py-1.5 text-sm"
                                >
                                    <option value="exact">Exact Bytes Match (Standard)</option>
                                    <option value="lines">Lines Differences (Ignore Trailing Spaces)</option>
                                    <option value="float">Float Tolerance Precision</option>
                                    <option value="float_absolute">Floating Point (Absolute Epsilon)</option>
                                    <option value="float_relative">Floating Point (Relative Epsilon)</option>
                                    <option value="custom">Custom Special Judge (SPJ)</option>
                                </select>
                            </div>

                            {(checkerType === 'float' || checkerType === 'float_absolute' || checkerType === 'float_relative') && (
                                <div>
                                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Float Epsilon (Precision Tolerance)</label>
                                    <input 
                                        type="number" 
                                        step="any" 
                                        value={floatEpsilon} 
                                        onChange={e => setFloatEpsilon(Number(e.target.value))} 
                                        className="border rounded px-3 py-1.5 text-sm w-48 font-mono" 
                                    />
                                </div>
                            )}

                            {checkerType === 'custom' && (
                                <div className="space-y-4">
                                    <div className="bg-gray-900 border border-gray-800 rounded-lg p-5 text-gray-300 shadow-md space-y-3">
                                        <div className="flex items-center space-x-2 text-yellow-400">
                                            <span className="w-2.5 h-2.5 rounded-full bg-yellow-400 animate-pulse"></span>
                                            <p className="font-bold text-xs uppercase tracking-wider">Special Judge (SPJ) Sandbox Environment Protocol</p>
                                        </div>
                                        <p className="text-xs leading-relaxed text-gray-400">
                                            Your compiled C++ checker binary runs inside a secure isolated sandbox with execution parameters:
                                        </p>
                                        <div className="bg-black/40 border border-gray-800 rounded p-2.5 font-mono text-[11px] text-emerald-400 select-all">
                                            ./spj input.txt user.txt answer.txt
                                        </div>
                                        <div className="grid grid-cols-2 gap-4 pt-2 text-[11px]">
                                            <div className="space-y-1.5">
                                                <span className="font-semibold block text-gray-400 uppercase tracking-wider text-[10px]">Compilation Outputs</span>
                                                <ul className="list-disc pl-4 space-y-0.5 text-gray-500">
                                                    <li>Standard streams are fully captured</li>
                                                    <li>Errors output on <span className="font-semibold text-gray-400">stderr</span> as details</li>
                                                </ul>
                                            </div>
                                            <div className="space-y-1.5">
                                                <span className="font-semibold block text-gray-400 uppercase tracking-wider text-[10px]">Return Status Codes</span>
                                                <ul className="list-disc pl-4 space-y-0.5 text-gray-500">
                                                    <li><span className="font-semibold text-emerald-400">exit status 0</span>: Accepted (AC)</li>
                                                    <li><span className="font-semibold text-rose-400">exit status 1</span>: Wrong Answer (WA)</li>
                                                </ul>
                                            </div>
                                        </div>
                                    </div>

                                    <div className="bg-white border rounded-lg p-4 shadow-sm space-y-4">
                                        <div className="flex justify-between items-center border-b pb-3">
                                            <div>
                                                <span className="text-xs text-gray-600 font-bold block uppercase tracking-wider">Advanced SPJ Presets</span>
                                                <span className="text-[11px] text-gray-400">Populate boilerplate templates directly into the editor</span>
                                            </div>
                                            <div className="flex gap-2">
                                                <button
                                                    type="button"
                                                    onClick={() => setSpjSourceCode(`#include <iostream>
#include <fstream>
#include <cmath>

using namespace std;

int main(int argc, char* argv[]) {
    if (argc < 4) {
        cerr << "Usage: spj <input> <user> <answer>" << endl;
        return 2;
    }
    
    ifstream fin(argv[1]);
    ifstream fuser(argv[2]);
    ifstream fans(argv[3]);
    
    double userVal, ansVal;
    if (!(fuser >> userVal)) {
        cerr << "Wrong Answer: Failed to read user float token" << endl;
        return 1;
    }
    if (!(fans >> ansVal)) {
        cerr << "System Error: Failed to read expected answer float token" << endl;
        return 2;
    }
    
    double diff = abs(userVal - ansVal);
    if (diff > 1e-9 && diff / max(1.0, abs(ansVal)) > 1e-9) {
        cerr << "Wrong Answer: Difference too large! Expected " << ansVal << ", got " << userVal << " (diff: " << diff << ")" << endl;
        return 1;
    }
    
    cout << "OK: Floats match within 1e-9" << endl;
    return 0;
}`)}
                                                    className="bg-gray-50 border border-gray-200 text-gray-700 px-3 py-1.5 rounded-md text-xs hover:bg-gray-100 hover:text-black transition-colors font-semibold cursor-pointer"
                                                >
                                                    Precision Float Presets
                                                </button>
                                                <button
                                                    type="button"
                                                    onClick={() => setSpjSourceCode(`#include <iostream>
#include <fstream>
#include <vector>

using namespace std;

int main(int argc, char* argv[]) {
    ifstream fin(argv[1]);   // Input case
    ifstream fuser(argv[2]); // User stdout
    ifstream fans(argv[3]);  // Expected output
    
    int n;
    fin >> n;
    
    vector<int> userArr(n);
    for (int i = 0; i < n; i++) {
        if (!(fuser >> userArr[i])) {
            cerr << "Wrong Answer: Insufficient numbers of tokens" << endl;
            return 1;
        }
    }
    
    for (int i = 1; i < n; i++) {
        if (userArr[i] < userArr[i-1]) {
            cerr << "Wrong Answer: Array is not sorted at index " << i << endl;
            return 1;
        }
    }
    
    cout << "OK: Sorted output verified" << endl;
    return 0;
}`)}
                                                    className="bg-gray-50 border border-gray-200 text-gray-700 px-3 py-1.5 rounded-md text-xs hover:bg-gray-100 hover:text-black transition-colors font-semibold cursor-pointer"
                                                >
                                                    Array/Graph Presets
                                                </button>
                                            </div>
                                        </div>

                                        <div className="grid grid-cols-2 gap-4">
                                            <div>
                                                <label className="block text-[10px] font-bold text-gray-400 uppercase tracking-wider mb-1">SPJ Sandbox Language</label>
                                                <select 
                                                    value={spjLanguage} 
                                                    onChange={e => setSpjLanguage(e.target.value)} 
                                                    className="border rounded px-3 py-1.5 text-sm"
                                                >
                                                    <option value="cpp-gpp-64">C++ (G++ 64-bit)</option>
                                                </select>
                                            </div>
                                        </div>

                                        <div>
                                            <label className="block text-[10px] font-bold text-gray-400 uppercase tracking-wider mb-1">SPJ Source Code Editor</label>
                                            <div className="border rounded overflow-hidden shadow-inner bg-gray-50">
                                                <CodeEditor 
                                                    language={spjLanguage}
                                                    value={spjSourceCode}
                                                    onChange={setSpjSourceCode}
                                                    height="350px"
                                                />
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            )}

                            <hr className="border-gray-200" />

                            <div className="space-y-3">
                                <h3 className="text-sm font-bold text-gray-700 uppercase tracking-wider">Interactive Problem</h3>
                                <div className="flex items-center gap-2">
                                    <input
                                        type="checkbox"
                                        id="interactive"
                                        checked={interactive}
                                        onChange={e => setInteractive(e.target.checked)}
                                        className="rounded"
                                    />
                                    <label htmlFor="interactive" className="text-sm font-medium text-gray-700">
                                        Interactive Problem (Judge communicates via stdin/stdout)
                                    </label>
                                </div>

                                {interactive && (
                                    <div className="space-y-3 pl-6 border-l-2 border-blue-200">
                                        <div>
                                            <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Language</label>
                                            <select
                                                value={interactorLanguage}
                                                onChange={e => setInteractorLanguage(e.target.value)}
                                                className="border border-gray-300 rounded px-2 py-1.5 text-sm"
                                            >
                                                <option value="cpp-gpp-64">C++ (G++ 64-bit)</option>
                                                <option value="python">Python 3</option>
                                            </select>
                                        </div>
                                        <div>
                                            <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Source Code</label>
                                            <div className="border rounded overflow-hidden shadow-inner bg-gray-50">
                                                <CodeEditor
                                                    language={interactorLanguage}
                                                    value={interactorSourceCode}
                                                    onChange={setInteractorSourceCode}
                                                    height="200px"
                                                />
                                            </div>
                                        </div>
                                    </div>
                                )}
                            </div>

                            <button 
                                onClick={handleSaveChecker} 
                                disabled={saving} 
                                className="bg-blue-600 text-white px-5 py-2 rounded text-sm font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
                            >
                                {saving ? 'Saving...' : 'Save Checker Configuration'}
                            </button>
                        </div>
                    )}

                    {activeTab === 'permissions' && (
                        <div className="space-y-6">
                            <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Problem Collaborators</h2>
                            <div className="border border-gray-200 rounded-lg overflow-hidden mb-4">
                                <table className="w-full text-sm text-left">
                                    <thead className="bg-gray-50 text-gray-500 text-xs uppercase font-semibold">
                                        <tr>
                                            <th className="px-4 py-2.5">User</th>
                                            <th className="px-4 py-2.5">Access Level</th>
                                            <th className="px-4 py-2.5 text-right">Action</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-gray-100">
                                        {collaborators.map((c, idx) => (
                                            <tr key={idx}>
                                                <td className="px-4 py-2.5 font-medium">{c.username}</td>
                                                <td className="px-4 py-2.5">
                                                    <span className={`px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wider ${
                                                        c.access_level === 'owner' ? 'bg-purple-100 text-purple-800' :
                                                        c.access_level === 'co_author' ? 'bg-blue-100 text-blue-800' : 'bg-gray-100 text-gray-800'
                                                    }`}>
                                                        {c.access_level}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-2.5 text-right">
                                                    {c.access_level !== 'owner' ? (
                                                        <button 
                                                            onClick={() => handleRemoveCollaborator(c.user_id)} 
                                                            className="text-red-600 hover:underline cursor-pointer"
                                                        >
                                                            Remove
                                                        </button>
                                                    ) : (
                                                        <span className="text-gray-400 text-xs">Primary Owner</span>
                                                    )}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>

                            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-4">
                                <h4 className="font-semibold text-xs text-gray-500 uppercase tracking-wider">Add Collaborator</h4>
                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-xs text-gray-500 mb-1">Username</label>
                                        <input 
                                            type="text" 
                                            placeholder="Enter username" 
                                            value={newUsername} 
                                            onChange={e => setNewUsername(e.target.value)} 
                                            className="w-full border rounded px-3 py-1.5 text-xs" 
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-xs text-gray-500 mb-1">Access Level</label>
                                        <select 
                                            value={newAccessLevel} 
                                            onChange={e => setNewAccessLevel(e.target.value)} 
                                            className="w-full border rounded px-3 py-1.5 text-xs bg-white"
                                        >
                                            <option value="co_author">Co-Author (Full Edit Permissions)</option>
                                            <option value="tester">Tester (Read & Submit Private Problem)</option>
                                        </select>
                                    </div>
                                </div>
                                <button 
                                    onClick={handleAddCollaborator} 
                                    className="bg-gray-800 text-white px-3 py-1.5 rounded text-xs hover:bg-black transition-colors cursor-pointer"
                                >
                                    Share Permissions
                                </button>
                            </div>
                        </div>
                    )}

                    {activeTab === 'settings' && (
                        <div className="space-y-6">
                            <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Workspace Settings</h2>
                            <div className="space-y-4">
                                <div className="flex items-center gap-3">
                                    <input 
                                        type="checkbox" 
                                        id="visible" 
                                        checked={visible} 
                                        onChange={e => setVisible(e.target.checked)} 
                                        className="h-4 w-4 border-gray-300 rounded text-blue-600 focus:ring-blue-500" 
                                    />
                                    <label htmlFor="visible" className="text-sm font-medium text-gray-700">
                                        Problem Visibility (Visible to Solvers)
                                    </label>
                                </div>
                                <p className="text-xs text-gray-400">If unchecked, the problem statement, statistics, and editorials will only be visible to authors and testers. Useful for preparing problems before contests.</p>

                                <button 
                                    onClick={handleSaveStatement} 
                                    disabled={saving} 
                                    className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors cursor-pointer"
                                >
                                    Update Visibility
                                </button>
                            </div>

                            <hr className="border-gray-200" />

                            <div className="space-y-2">
                                <h3 className="text-sm font-bold text-red-600 uppercase tracking-wider">Danger Zone</h3>
                                <p className="text-xs text-gray-400">Deleting the problem will discard all statements, submissions history, and testcase files from the server sandbox. This operation cannot be undone.</p>
                                <button 
                                    onClick={handleDeleteProblem} 
                                    className="bg-red-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-red-700 transition-colors cursor-pointer"
                                >
                                    Delete Problem
                                </button>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}
