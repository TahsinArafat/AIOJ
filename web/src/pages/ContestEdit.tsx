import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function ContestEdit() {
    const { id } = useParams<{ id: string }>()
    const nav = useNavigate()
    const [contest, setContest] = useState<any>(null)
    const [problems, setProblems] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState('')

    const [form, setForm] = useState({
        title: '',
        description: '',
        start_time: '',
        end_time: '',
        freeze_time: '',
        password: '',
        pdf_enabled: true,
        statement_hidden: false,
        upsolving_enabled: true,
        virtual_contest_enabled: true,
    })

    const [searchQuery, setSearchQuery] = useState('')
    const [searchResults, setSearchResults] = useState<any[]>([])
    const [searching, setSearching] = useState(false)

    useEffect(() => {
        if (!id) return
        loadData()
    }, [id])

    async function loadData() {
        try {
            const data = await api.contests.get(id!)
            setContest(data.contest)
            setProblems(data.problems || [])
            setForm({
                title: data.contest.title || '',
                description: data.contest.description || '',
                start_time: data.contest.start_time ? new Date(data.contest.start_time).toISOString().slice(0, 16) : '',
                end_time: data.contest.end_time ? new Date(data.contest.end_time).toISOString().slice(0, 16) : '',
                freeze_time: data.contest.freeze_time ? new Date(data.contest.freeze_time).toISOString().slice(0, 16) : '',
                password: '',
                pdf_enabled: data.contest.pdf_enabled ?? true,
                statement_hidden: data.contest.statement_hidden ?? false,
                upsolving_enabled: data.contest.upsolving_enabled ?? true,
                virtual_contest_enabled: data.contest.virtual_contest_enabled ?? true,
            })
        } catch (err: any) {
            setError(err.message || 'Failed to load contest')
        } finally {
            setLoading(false)
        }
    }

    const handleSearch = useCallback(async (query: string) => {
        if (query.length < 2) {
            setSearchResults([])
            return
        }
        setSearching(true)
        try {
            const res = await api.problems.list(0, 20, { search: query })
            setSearchResults(res.data || [])
        } catch {
            setSearchResults([])
        } finally {
            setSearching(false)
        }
    }, [])

    useEffect(() => {
        const timer = setTimeout(() => handleSearch(searchQuery), 300)
        return () => clearTimeout(timer)
    }, [searchQuery, handleSearch])

    const handleAddProblem = async (problemId: string) => {
        if (!id) return
        const nextIndex = String.fromCharCode(65 + problems.length)
        
        try {
            await api.contests.addProblem(id, {
                problem_id: problemId,
                index: nextIndex,
                score: 100,
            })
            await loadData()
            setSearchQuery('')
            setSearchResults([])
        } catch (err: any) {
            alert('Failed to add problem: ' + err.message)
        }
    }

    const handleRemoveProblem = async (problemId: string) => {
        if (!id || !confirm('Remove this problem from contest?')) return
        try {
            await api.contests.removeProblem(id, problemId)
            await loadData()
        } catch (err: any) {
            alert('Failed to remove problem: ' + err.message)
        }
    }

    const handleSave = async () => {
        if (!id) return
        setSaving(true)
        try {
            await api.contests.update(id, {
                title: form.title,
                description: form.description || undefined,
                start_time: form.start_time ? new Date(form.start_time).toISOString() : undefined,
                end_time: form.end_time ? new Date(form.end_time).toISOString() : undefined,
                freeze_time: form.freeze_time ? new Date(form.freeze_time).toISOString() : undefined,
                password: form.password || undefined,
                pdf_enabled: form.pdf_enabled,
                statement_hidden: form.statement_hidden,
                upsolving_enabled: form.upsolving_enabled,
                virtual_contest_enabled: form.virtual_contest_enabled,
            })
            nav(`/contests/${id}`)
        } catch (err: any) {
            setError(err.message || 'Failed to save')
        } finally {
            setSaving(false)
        }
    }

    if (loading) return <div className="text-center py-20 text-gray-400">Loading...</div>
    if (!contest) return <div className="text-center py-20 text-gray-400">Contest not found.</div>

    return (
        <div className="max-w-4xl mx-auto">
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Edit Contest</h1>
                <Link to={`/contests/${id}`} className="text-sm text-blue-600 hover:underline">← Back to Contest</Link>
            </div>

            {error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{error}</div>}

            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Contest Settings */}
                <div className="space-y-4">
                    <div className="border border-gray-200 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Contest Settings</h2>
                        <div className="space-y-3">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
                                <input value={form.title} onChange={e => setForm(p => ({ ...p, title: e.target.value }))}
                                    className="w-full border rounded px-3 py-2 text-sm" />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                                <textarea value={form.description} onChange={e => setForm(p => ({ ...p, description: e.target.value }))}
                                    rows={3} className="w-full border rounded px-3 py-2 text-sm" />
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Start Time</label>
                                    <input type="datetime-local" value={form.start_time} onChange={e => setForm(p => ({ ...p, start_time: e.target.value }))}
                                        className="w-full border rounded px-3 py-2 text-sm" />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">End Time</label>
                                    <input type="datetime-local" value={form.end_time} onChange={e => setForm(p => ({ ...p, end_time: e.target.value }))}
                                        className="w-full border rounded px-3 py-2 text-sm" />
                                </div>
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Freeze Time</label>
                                    <input type="datetime-local" value={form.freeze_time} onChange={e => setForm(p => ({ ...p, freeze_time: e.target.value }))}
                                        className="w-full border rounded px-3 py-2 text-sm" />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
                                    <input value={form.password} onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                                        placeholder="Empty = public" className="w-full border rounded px-3 py-2 text-sm" />
                                </div>
                            </div>
                        </div>
                    </div>

                    <div className="border border-gray-200 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Options</h2>
                        <div className="space-y-2">
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input type="checkbox" checked={form.pdf_enabled}
                                    onChange={e => setForm(p => ({ ...p, pdf_enabled: e.target.checked }))}
                                    className="rounded" />
                                <span className="text-sm">Enable PDF Generation</span>
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input type="checkbox" checked={form.statement_hidden}
                                    onChange={e => setForm(p => ({ ...p, statement_hidden: e.target.checked }))}
                                    className="rounded" />
                                <span className="text-sm">Hide Problem Statements (Onsite)</span>
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input type="checkbox" checked={form.upsolving_enabled}
                                    onChange={e => setForm(p => ({ ...p, upsolving_enabled: e.target.checked }))}
                                    className="rounded" />
                                <span className="text-sm">Enable Upsolving</span>
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input type="checkbox" checked={form.virtual_contest_enabled}
                                    onChange={e => setForm(p => ({ ...p, virtual_contest_enabled: e.target.checked }))}
                                    className="rounded" />
                                <span className="text-sm">Enable Virtual Contests</span>
                            </label>
                        </div>
                    </div>

                    <button onClick={handleSave} disabled={saving}
                        className="w-full bg-blue-600 text-white py-2 rounded hover:bg-blue-700 disabled:opacity-50">
                        {saving ? 'Saving...' : 'Save Changes'}
                    </button>
                </div>

                {/* Problem Management */}
                <div className="space-y-4">
                    <div className="border border-gray-200 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Add Problems</h2>
                        <div className="relative">
                            <input
                                value={searchQuery}
                                onChange={e => setSearchQuery(e.target.value)}
                                placeholder="Search problems by title or slug..."
                                className="w-full border rounded px-3 py-2 text-sm"
                            />
                            {searching && <div className="absolute right-3 top-2.5 text-gray-400 text-sm">Searching...</div>}
                            
                            {searchResults.length > 0 && (
                                <div className="absolute z-10 w-full mt-1 bg-white border rounded-lg shadow-lg max-h-60 overflow-y-auto">
                                    {searchResults.map(p => (
                                        <div key={p.id} className="flex items-center justify-between px-3 py-2 hover:bg-gray-50 border-b last:border-0">
                                            <div>
                                                <div className="font-medium text-sm">{p.title}</div>
                                                <div className="text-xs text-gray-500">{p.slug} • {p.difficulty}</div>
                                            </div>
                                            <button
                                                onClick={() => handleAddProblem(p.id)}
                                                className="text-xs bg-blue-100 text-blue-700 px-2 py-1 rounded hover:bg-blue-200"
                                            >
                                                Add
                                            </button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                        <p className="text-xs text-gray-500 mt-1">Search public problems and your private problems</p>
                    </div>

                    <div className="border border-gray-200 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Current Problems ({problems.length})</h2>
                        {problems.length === 0 ? (
                            <p className="text-sm text-gray-500">No problems added yet.</p>
                        ) : (
                            <div className="space-y-2">
                                {problems.map((p: any) => (
                                    <div key={p.problem_id} className="flex items-center justify-between p-2 bg-gray-50 rounded">
                                        <div>
                                            <span className="font-bold text-blue-600 mr-2">{p.index}</span>
                                            <span className="text-sm">{p.problem_id.substring(0, 8)}...</span>
                                        </div>
                                        <button
                                            onClick={() => handleRemoveProblem(p.problem_id)}
                                            className="text-xs text-red-600 hover:text-red-800"
                                        >
                                            Remove
                                        </button>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </div>
    )
}
