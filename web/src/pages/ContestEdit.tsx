import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api } from '../lib/api'
import { resolveProblemTitle } from '../lib/problemSlugResolver'

function indexLabel(i: number): string {
    let s = ''
    let n = i
    do {
        s = String.fromCharCode(65 + (n % 26)) + s
        n = Math.floor(n / 26) - 1
    } while (n >= 0)
    return s
}

export default function ContestEdit() {
    const { id } = useParams<{ id: string }>()
    const nav = useNavigate()
    const [contest, setContest] = useState<any>(null)
    const [problems, setProblems] = useState<any[]>([])
    const [problemTitles, setProblemTitles] = useState<Record<string, string>>({})
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
            const probs = data.problems || []
            setProblems(probs)
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
            // Resolve titles for all problems
            const titles: Record<string, string> = {}
            await Promise.all(
                probs.map(async (p: any) => {
                    const title = await resolveProblemTitle(p.problem_id)
                    titles[p.problem_id] = title || p.problem_id.substring(0, 8) + '...'
                })
            )
            setProblemTitles(titles)
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
        const nextIndex = indexLabel(problems.length)
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
        if (!id || !confirm('Remove this problem from the contest?')) return
        try {
            await api.contests.removeProblem(id, problemId)
            await loadData()
        } catch (err: any) {
            alert('Failed to remove problem: ' + err.message)
        }
    }

    const handleMoveProblem = (idx: number, dir: -1 | 1) => {
        const target = idx + dir
        if (target < 0 || target >= problems.length) return
        const next = [...problems]
        ;[next[idx], next[target]] = [next[target], next[idx]]
        // Reassign indices
        next.forEach((p, i) => { p.index = indexLabel(i) })
        setProblems(next)
    }

    const validate = (): string | null => {
        if (!form.title.trim()) return 'Title is required'
        if (!form.start_time) return 'Start time is required'
        if (!form.end_time) return 'End time is required'
        if (new Date(form.end_time) <= new Date(form.start_time)) return 'End time must be after start time'
        if (form.freeze_time && new Date(form.freeze_time) <= new Date(form.start_time)) return 'Freeze time must be after start time'
        if (form.freeze_time && new Date(form.freeze_time) >= new Date(form.end_time)) return 'Freeze time must be before end time'
        return null
    }

    const handleSave = async () => {
        if (!id) return
        const err = validate()
        if (err) { setError(err); return }
        setError('')
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
        <div className="max-w-6xl mx-auto">
            {/* Header */}
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Edit Contest</h1>
                <Link to={`/contests/${id}`} className="text-sm text-blue-600 hover:underline">
                    ← Back to Contest
                </Link>
            </div>

            {error && (
                <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">
                    {error}
                </div>
            )}

            {/* Two-column layout */}
            <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
                {/* Left: Settings (60%) */}
                <div className="lg:col-span-3 space-y-4">
                    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Contest Settings</h2>
                        <div className="space-y-3">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
                                <input
                                    value={form.title}
                                    onChange={e => setForm(p => ({ ...p, title: e.target.value }))}
                                    className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                                <textarea
                                    value={form.description}
                                    onChange={e => setForm(p => ({ ...p, description: e.target.value }))}
                                    rows={3}
                                    className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                />
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Start Time</label>
                                    <input
                                        type="datetime-local"
                                        value={form.start_time}
                                        onChange={e => setForm(p => ({ ...p, start_time: e.target.value }))}
                                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">End Time</label>
                                    <input
                                        type="datetime-local"
                                        value={form.end_time}
                                        onChange={e => setForm(p => ({ ...p, end_time: e.target.value }))}
                                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    />
                                </div>
                            </div>
                            <div className="grid grid-cols-2 gap-3">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Freeze Time</label>
                                    <input
                                        type="datetime-local"
                                        value={form.freeze_time}
                                        onChange={e => setForm(p => ({ ...p, freeze_time: e.target.value }))}
                                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
                                    <input
                                        value={form.password}
                                        onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                                        placeholder="Empty = public"
                                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Options */}
                    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Options</h2>
                        <div className="grid grid-cols-2 gap-2">
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={form.pdf_enabled}
                                    onChange={e => setForm(p => ({ ...p, pdf_enabled: e.target.checked }))}
                                    className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                                />
                                <span className="text-sm">PDF Generation</span>
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={form.statement_hidden}
                                    onChange={e => setForm(p => ({ ...p, statement_hidden: e.target.checked }))}
                                    className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                                />
                                <span className="text-sm">Hide Statements (Onsite)</span>
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={form.upsolving_enabled}
                                    onChange={e => setForm(p => ({ ...p, upsolving_enabled: e.target.checked }))}
                                    className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                                />
                                <span className="text-sm">Enable Upsolving</span>
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={form.virtual_contest_enabled}
                                    onChange={e => setForm(p => ({ ...p, virtual_contest_enabled: e.target.checked }))}
                                    className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                                />
                                <span className="text-sm">Virtual Contests</span>
                            </label>
                        </div>
                    </div>

                    {/* Save button */}
                    <button
                        onClick={handleSave}
                        disabled={saving}
                        className="w-full bg-blue-600 text-white py-2.5 rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors font-medium"
                    >
                        {saving ? 'Saving...' : 'Save Changes'}
                    </button>
                </div>

                {/* Right: Problem Management (40%) */}
                <div className="lg:col-span-2 space-y-4">
                    {/* Add Problems */}
                    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Add Problems</h2>
                        <div className="relative">
                            <input
                                value={searchQuery}
                                onChange={e => setSearchQuery(e.target.value)}
                                placeholder="Search by title or slug..."
                                className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                            />
                            {searching && (
                                <div className="absolute right-3 top-2.5 text-gray-400 text-xs">Searching...</div>
                            )}
                            {searchResults.length > 0 && (
                                <div className="absolute z-10 w-full mt-1 bg-white dark:bg-gray-800 border rounded-lg shadow-lg max-h-60 overflow-y-auto">
                                    {searchResults.map(p => (
                                        <div
                                            key={p.id}
                                            className="flex items-center justify-between px-3 py-2 hover:bg-gray-50 border-b last:border-0"
                                        >
                                            <div className="min-w-0">
                                                <div className="font-medium text-sm truncate">{p.title}</div>
                                                <div className="text-xs text-gray-500">
                                                    {p.slug} · {p.difficulty}
                                                </div>
                                            </div>
                                            <button
                                                onClick={() => handleAddProblem(p.id)}
                                                className="ml-2 text-xs bg-blue-100 text-blue-700 px-2 py-1 rounded hover:bg-blue-200 shrink-0"
                                            >
                                                Add
                                            </button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                        <p className="text-xs text-gray-500 mt-1">Search public and your private problems</p>
                    </div>

                    {/* Current Problems */}
                    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                        <h2 className="font-semibold mb-3">Current Problems ({problems.length})</h2>
                        {problems.length === 0 ? (
                            <p className="text-sm text-gray-500 py-4 text-center">No problems added yet.</p>
                        ) : (
                            <div className="space-y-2">
                                {problems.map((p: any, idx: number) => (
                                    <div
                                        key={p.problem_id}
                                        className="flex items-center gap-2 p-2 bg-gray-50 rounded-md group"
                                    >
                                        <span className="font-bold text-blue-600 text-sm w-7 text-center shrink-0">
                                            {indexLabel(idx)}
                                        </span>
                                        <div className="flex-1 min-w-0">
                                            <div className="text-sm font-medium truncate">
                                                {problemTitles[p.problem_id] || 'Loading...'}
                                            </div>
                                        </div>
                                        <div className="flex items-center gap-1 shrink-0">
                                            <button
                                                onClick={() => handleMoveProblem(idx, -1)}
                                                disabled={idx === 0}
                                                className="p-1 text-gray-400 hover:text-gray-700 disabled:opacity-30 disabled:cursor-not-allowed"
                                                title="Move up"
                                            >
                                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                                                </svg>
                                            </button>
                                            <button
                                                onClick={() => handleMoveProblem(idx, 1)}
                                                disabled={idx === problems.length - 1}
                                                className="p-1 text-gray-400 hover:text-gray-700 disabled:opacity-30 disabled:cursor-not-allowed"
                                                title="Move down"
                                            >
                                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                                </svg>
                                            </button>
                                            <button
                                                onClick={() => handleRemoveProblem(p.problem_id)}
                                                className="p-1 text-gray-400 hover:text-red-600"
                                                title="Remove"
                                            >
                                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                                </svg>
                                            </button>
                                        </div>
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
