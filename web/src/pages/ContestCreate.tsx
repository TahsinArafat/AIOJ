import { useState, useEffect, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, getAccessToken, contestSlug } from '../lib/api'
import { DIVISIONS } from '../lib/divisions'

const FORMAT_OPTIONS = [
    { value: 'acm', label: 'ACM/ICPC', desc: 'Standard penalty-based scoring with time penalty.' },
    { value: 'oi', label: 'OI', desc: 'Max score per problem. No time penalty.' },
    { value: 'ioi', label: 'IOI', desc: 'Subtask-based scoring with partial credit.' },
    { value: 'atcoder', label: 'AtCoder', desc: 'First AC time as penalty. No wrong submission penalty.' },
    { value: 'codeforces', label: 'Codeforces', desc: 'Dynamic score that decays over time.' },
]

const TYPE_OPTIONS = [
    { value: 'acm', label: 'ACM' },
    { value: 'oi', label: 'OI' },
    { value: 'ioi', label: 'IOI' },
    { value: 'practice', label: 'Practice' },
    { value: 'educational', label: 'Educational' },
]

const PROBLEM_LETTERS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'

interface SelectedProblem {
    id: string
    title: string
    slug: string
    difficulty: string
}

function slugify(text: string): string {
    return text
        .toLowerCase()
        .replace(/[^a-z0-9\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '')
        .slice(0, 80)
}

function formatDuration(minutes: number): string {
    const h = Math.floor(minutes / 60)
    const m = minutes % 60
    if (h === 0) return `${m}m`
    if (m === 0) return `${h}h`
    return `${h}h ${m}m`
}

function generateCFScores(count: number): number[] {
    const scores: number[] = []
    for (let i = 0; i < count; i++) {
        scores.push(500 * (i + 1))
    }
    return scores
}

export default function ContestCreate() {
    const nav = useNavigate()

    // Basic info
    const [title, setTitle] = useState('')
    const [slug, setSlug] = useState('')
    const [slugEdited, setSlugEdited] = useState(false)
    const [description, setDescription] = useState('')

    // Schedule
    const [startTime, setStartTime] = useState('')
    const [endTime, setEndTime] = useState('')
    const [freezeTime, setFreezeTime] = useState('')

    // Format & division
    const [format, setFormat] = useState('acm')
    const [division, setDivision] = useState('0')
    const [type, setType] = useState('acm')

    // Format-specific
    const [penaltyPerWrong, setPenaltyPerWrong] = useState('20')
    const [maxScorePerProblem, setMaxScorePerProblem] = useState('100')
    const [decayFactor, setDecayFactor] = useState('250')
    const [minRatio, setMinRatio] = useState('0.3')
    const [cfPenalty, setCfPenalty] = useState('50')

    // Problems
    const [selectedProblems, setSelectedProblems] = useState<SelectedProblem[]>([])
    const [searchQuery, setSearchQuery] = useState('')
    const [searchResults, setSearchResults] = useState<SelectedProblem[]>([])
    const [searchLoading, setSearchLoading] = useState(false)
    const [showDropdown, setShowDropdown] = useState(false)
    const searchRef = useRef<HTMLDivElement>(null)
    const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined)

    // Options
    const [pdfEnabled, setPdfEnabled] = useState(true)
    const [statementHidden, setStatementHidden] = useState(false)
    const [password, setPassword] = useState('')

    // UI state
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)

    // Auto-generate slug from title
    useEffect(() => {
        if (!slugEdited) {
            setSlug(slugify(title))
        }
    }, [title, slugEdited])

    // Auth guard
    useEffect(() => {
        if (!getAccessToken()) nav('/login')
    }, [nav])

    // Click outside to close search dropdown
    useEffect(() => {
        const handler = (e: MouseEvent) => {
            if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
                setShowDropdown(false)
            }
        }
        document.addEventListener('mousedown', handler)
        return () => document.removeEventListener('mousedown', handler)
    }, [])

    // Duration calculation
    const durationMinutes = (() => {
        if (!startTime || !endTime) return null
        const diff = new Date(endTime).getTime() - new Date(startTime).getTime()
        return diff > 0 ? Math.round(diff / 60000) : null
    })()

    // Debounced problem search
    const handleSearchChange = useCallback((value: string) => {
        setSearchQuery(value)
        if (debounceRef.current) clearTimeout(debounceRef.current)

        if (!value.trim()) {
            setSearchResults([])
            setShowDropdown(false)
            return
        }

        debounceRef.current = setTimeout(async () => {
            setSearchLoading(true)
            try {
                const res = await api.problems.list(0, 10, { search: value.trim() })
                const items = (res.data || []).map((p: any) => ({
                    id: p.id,
                    title: p.title,
                    slug: p.slug,
                    difficulty: p.difficulty || '',
                }))
                setSearchResults(items)
                setShowDropdown(true)
            } catch {
                setSearchResults([])
            } finally {
                setSearchLoading(false)
            }
        }, 300)
    }, [])

    const addProblem = (problem: SelectedProblem) => {
        if (selectedProblems.some(p => p.id === problem.id)) return
        setSelectedProblems(prev => [...prev, problem])
        setSearchQuery('')
        setSearchResults([])
        setShowDropdown(false)
    }

    const removeProblem = (index: number) => {
        setSelectedProblems(prev => prev.filter((_, i) => i !== index))
    }

    const moveProblem = (index: number, direction: -1 | 1) => {
        const newIndex = index + direction
        if (newIndex < 0 || newIndex >= selectedProblems.length) return
        setSelectedProblems(prev => {
            const copy = [...prev]
            ;[copy[index], copy[newIndex]] = [copy[newIndex], copy[index]]
            return copy
        })
    }

    // Validation
    const validate = (): string | null => {
        if (!title.trim()) return 'Contest title is required.'
        if (!startTime) return 'Start time is required.'
        if (!endTime) return 'End time is required.'
        if (new Date(endTime) <= new Date(startTime)) return 'End time must be after start time.'
        if (freezeTime) {
            if (new Date(freezeTime) <= new Date(startTime)) return 'Freeze time must be after start time.'
            if (new Date(freezeTime) >= new Date(endTime)) return 'Freeze time must be before end time.'
        }
        return null
    }

    // Submit
    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        const err = validate()
        if (err) { setError(err); return }
        setError('')
        setSubmitting(true)

        try {
            let formatConfig: any = {}
            if (format === 'acm') {
                formatConfig = { penalty_per_wrong: Number(penaltyPerWrong), time_penalty: true }
            } else if (format === 'oi') {
                formatConfig = { max_score_per_problem: Number(maxScorePerProblem) }
            } else if (format === 'ioi') {
                formatConfig = { partial_credit: true, subtask_scoring: true }
            } else if (format === 'atcoder') {
                formatConfig = { penalty_is_time_of_ac: true, no_wrong_attempt_penalty: true }
            } else if (format === 'codeforces') {
                formatConfig = {
                    initial_scores: generateCFScores(selectedProblems.length),
                    decay_factor: Number(decayFactor),
                    min_score_ratio: Number(minRatio),
                    wrong_submission_penalty: Number(cfPenalty),
                }
            }

            const contest = await api.contests.create({
                title: title.trim(),
                slug: slug || undefined,
                type,
                format,
                format_config: formatConfig,
                start_time: new Date(startTime).toISOString(),
                end_time: new Date(endTime).toISOString(),
                freeze_time: freezeTime ? new Date(freezeTime).toISOString() : undefined,
                password: password || undefined,
                description: description || undefined,
                problem_ids: selectedProblems.length > 0 ? selectedProblems.map(p => p.id) : undefined,
                division: Number(division),
                pdf_enabled: pdfEnabled,
                statement_hidden: statementHidden,
            })
            nav(`/contests/${contestSlug(contest)}`)
        } catch (err: any) {
            setError(err.message || 'Failed to create contest.')
        } finally {
            setSubmitting(false)
        }
    }

    const difficultyColor = (d: string) => {
        const map: Record<string, string> = {
            easy: 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20',
            medium: 'text-yellow-600 dark:text-yellow-400 bg-yellow-50 dark:bg-yellow-900/20',
            hard: 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20',
            'very easy': 'text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-900/20',
            'very hard': 'text-red-700 dark:text-red-300 bg-red-100 dark:bg-red-900/30',
        }
        return map[(d || '').toLowerCase()] || 'text-gray-500 bg-gray-50 dark:bg-gray-900/20'
    }

    const sectionLabel = (text: string) => (
        <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 uppercase tracking-wide mb-3">{text}</h3>
    )

    const inputCls = "w-full border border-gray-300 dark:border-gray-700 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition"
    const labelCls = "block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"

    return (
        <div className="max-w-3xl mx-auto pb-12">
            <div className="mb-8">
                <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">Create Contest</h1>
                <p className="text-sm text-gray-500 mt-1">Set up a new competitive programming contest.</p>
            </div>

            {error && (
                <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 text-red-700 dark:text-red-300 px-4 py-3 rounded-lg mb-6 text-sm flex items-start gap-2">
                    <svg className="w-4 h-4 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                    </svg>
                    <span>{error}</span>
                </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-8">

                {/* ── Basic Info ── */}
                <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                    {sectionLabel('Basic Information')}
                    <div className="space-y-4">
                        <div>
                            <label className={labelCls}>
                                Title <span className="text-red-400">*</span>
                            </label>
                            <input
                                required
                                value={title}
                                onChange={e => setTitle(e.target.value)}
                                placeholder="e.g. AIOJ Round 1 - Division 2"
                                className={inputCls}
                            />
                        </div>
                        <div>
                            <label className={labelCls}>URL Slug</label>
                            <div className="flex items-center gap-0">
                                <span className="inline-flex items-center px-3 py-2 rounded-l-md border border-r-0 border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/20 text-gray-500 text-sm">
                                    /contests/
                                </span>
                                <input
                                    value={slug}
                                    onChange={e => { setSlug(e.target.value); setSlugEdited(true) }}
                                    placeholder="auto-generated-from-title"
                                    pattern="[a-z0-9\-]*"
                                    className="flex-1 border border-gray-300 dark:border-gray-700 rounded-r-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition"
                                />
                            </div>
                        </div>
                        <div>
                            <label className={labelCls}>Description</label>
                            <textarea
                                rows={3}
                                value={description}
                                onChange={e => setDescription(e.target.value)}
                                placeholder="Optional description shown on the contest page..."
                                className={inputCls + " resize-none"}
                            />
                        </div>
                    </div>
                </section>

                {/* ── Schedule ── */}
                <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                    {sectionLabel('Schedule')}
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                        <div>
                            <label className={labelCls}>
                                Start Time <span className="text-red-400">*</span>
                            </label>
                            <input
                                type="datetime-local"
                                required
                                value={startTime}
                                onChange={e => setStartTime(e.target.value)}
                                className={inputCls}
                            />
                        </div>
                        <div>
                            <label className={labelCls}>
                                End Time <span className="text-red-400">*</span>
                            </label>
                            <input
                                type="datetime-local"
                                required
                                value={endTime}
                                onChange={e => setEndTime(e.target.value)}
                                className={inputCls}
                            />
                        </div>
                        <div>
                            <label className={labelCls}>Freeze Time</label>
                            <input
                                type="datetime-local"
                                value={freezeTime}
                                onChange={e => setFreezeTime(e.target.value)}
                                className={inputCls}
                            />
                        </div>
                    </div>
                    {durationMinutes !== null && (
                        <p className="mt-2 text-xs text-gray-500">
                            Duration: <span className="font-medium text-gray-700 dark:text-gray-300">{formatDuration(durationMinutes)}</span>
                        </p>
                    )}
                </section>

                {/* ── Format ── */}
                <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                    {sectionLabel('Format & Division')}
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                        <div>
                            <label className={labelCls}>Scoring Format</label>
                            <select value={format} onChange={e => setFormat(e.target.value)} className={inputCls}>
                                {FORMAT_OPTIONS.map(o => (
                                    <option key={o.value} value={o.value}>{o.label}</option>
                                ))}
                            </select>
                            <p className="mt-1 text-xs text-gray-400">
                                {FORMAT_OPTIONS.find(o => o.value === format)?.desc}
                            </p>
                        </div>
                        <div>
                            <label className={labelCls}>Division</label>
                            <select value={division} onChange={e => setDivision(e.target.value)} className={inputCls}>
                                {(Object.entries(DIVISIONS) as [string, { name: string }][]).map(([k, v]) => (
                                    <option key={k} value={k}>{v.name}</option>
                                ))}
                            </select>
                        </div>
                        <div>
                            <label className={labelCls}>Contest Type</label>
                            <select value={type} onChange={e => setType(e.target.value)} className={inputCls}>
                                {TYPE_OPTIONS.map(o => (
                                    <option key={o.value} value={o.value}>{o.label}</option>
                                ))}
                            </select>
                        </div>
                    </div>

                    {/* Format-specific settings */}
                    {format === 'acm' && (
                        <div className="mt-4 max-w-xs">
                            <label className={labelCls}>Penalty Per Wrong Answer (minutes)</label>
                            <input
                                type="number"
                                min="0"
                                value={penaltyPerWrong}
                                onChange={e => setPenaltyPerWrong(e.target.value)}
                                className={inputCls}
                            />
                        </div>
                    )}

                    {format === 'oi' && (
                        <div className="mt-4 max-w-xs">
                            <label className={labelCls}>Max Score Per Problem</label>
                            <input
                                type="number"
                                min="1"
                                value={maxScorePerProblem}
                                onChange={e => setMaxScorePerProblem(e.target.value)}
                                className={inputCls}
                            />
                        </div>
                    )}

                    {format === 'codeforces' && (
                        <div className="mt-4 space-y-3">
                            <div className="grid grid-cols-3 gap-3 max-w-lg">
                                <div>
                                    <label className={labelCls}>Decay Factor</label>
                                    <input
                                        type="number"
                                        min="1"
                                        value={decayFactor}
                                        onChange={e => setDecayFactor(e.target.value)}
                                        className={inputCls}
                                    />
                                </div>
                                <div>
                                    <label className={labelCls}>Min Score Ratio</label>
                                    <input
                                        type="number"
                                        min="0"
                                        max="1"
                                        step="0.05"
                                        value={minRatio}
                                        onChange={e => setMinRatio(e.target.value)}
                                        className={inputCls}
                                    />
                                </div>
                                <div>
                                    <label className={labelCls}>Wrong Sub. Penalty</label>
                                    <input
                                        type="number"
                                        min="0"
                                        value={cfPenalty}
                                        onChange={e => setCfPenalty(e.target.value)}
                                        className={inputCls}
                                    />
                                </div>
                            </div>
                            {selectedProblems.length > 0 && (
                                <p className="text-xs text-gray-500">
                                    Initial scores (auto):{' '}
                                    <span className="font-mono text-gray-700 dark:text-gray-300">
                                        [{generateCFScores(selectedProblems.length).join(', ')}]
                                    </span>
                                </p>
                            )}
                        </div>
                    )}
                </section>

                {/* ── Problem Selection ── */}
                <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                    {sectionLabel('Problems')}
                    <div className="relative" ref={searchRef}>
                        <label className={labelCls}>Search &amp; Add Problems</label>
                        <div className="relative">
                            <input
                                type="text"
                                value={searchQuery}
                                onChange={e => handleSearchChange(e.target.value)}
                                onFocus={() => { if (searchResults.length > 0) setShowDropdown(true) }}
                                placeholder="Type a problem title or slug..."
                                className={inputCls + " pr-8"}
                            />
                            {searchLoading && (
                                <div className="absolute right-3 top-1/2 -translate-y-1/2">
                                    <div className="w-4 h-4 border-2 border-blue-300 dark:border-blue-700 border-t-blue-600 rounded-full animate-spin" />
                                </div>
                            )}
                        </div>

                        {showDropdown && searchResults.length > 0 && (
                            <div className="absolute z-20 w-full mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg max-h-60 overflow-y-auto">
                                {searchResults.map(p => (
                                    <button
                                        key={p.id}
                                        type="button"
                                        onClick={() => addProblem(p)}
                                        className="w-full text-left px-4 py-2.5 hover:bg-blue-50 flex items-center justify-between gap-2 border-b border-gray-100 dark:border-gray-800 last:border-0 transition"
                                    >
                                        <div className="min-w-0">
                                            <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate block">{p.title}</span>
                                            <span className="text-xs text-gray-400">{p.slug}</span>
                                        </div>
                                        {p.difficulty && (
                                            <span className={`text-xs px-2 py-0.5 rounded-full font-medium flex-shrink-0 ${difficultyColor(p.difficulty)}`}>
                                                {p.difficulty}
                                            </span>
                                        )}
                                    </button>
                                ))}
                            </div>
                        )}
                        {showDropdown && searchQuery.trim() && searchResults.length === 0 && !searchLoading && (
                            <div className="absolute z-20 w-full mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg px-4 py-3 text-sm text-gray-400">
                                No problems found.
                            </div>
                        )}
                    </div>

                    {/* Selected problems list */}
                    {selectedProblems.length > 0 && (
                        <div className="mt-4 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                            <table className="w-full text-sm">
                                <thead>
                                    <tr className="bg-gray-50 dark:bg-gray-900/20 text-left">
                                        <th className="px-4 py-2 font-medium text-gray-500 w-12">#</th>
                                        <th className="px-4 py-2 font-medium text-gray-500">Problem</th>
                                        <th className="px-4 py-2 font-medium text-gray-500 w-24">Difficulty</th>
                                        <th className="px-4 py-2 font-medium text-gray-500 w-24 text-right">Actions</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {selectedProblems.map((p, i) => (
                                        <tr key={p.id} className="border-t border-gray-100 dark:border-gray-800 hover:bg-gray-50">
                                            <td className="px-4 py-2.5">
                                                <span className="inline-flex items-center justify-center w-6 h-6 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-bold text-xs">
                                                    {PROBLEM_LETTERS[i] || (i + 1)}
                                                </span>
                                            </td>
                                            <td className="px-4 py-2.5">
                                                <span className="font-medium text-gray-900 dark:text-gray-100">{p.title}</span>
                                                <span className="text-gray-400 ml-2 text-xs">{p.slug}</span>
                                            </td>
                                            <td className="px-4 py-2.5">
                                                {p.difficulty && (
                                                    <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${difficultyColor(p.difficulty)}`}>
                                                        {p.difficulty}
                                                    </span>
                                                )}
                                            </td>
                                            <td className="px-4 py-2.5">
                                                <div className="flex items-center justify-end gap-1">
                                                    <button
                                                        type="button"
                                                        onClick={() => moveProblem(i, -1)}
                                                        disabled={i === 0}
                                                        className="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30 disabled:cursor-not-allowed"
                                                        title="Move up"
                                                    >
                                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" /></svg>
                                                    </button>
                                                    <button
                                                        type="button"
                                                        onClick={() => moveProblem(i, 1)}
                                                        disabled={i === selectedProblems.length - 1}
                                                        className="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30 disabled:cursor-not-allowed"
                                                        title="Move down"
                                                    >
                                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" /></svg>
                                                    </button>
                                                    <button
                                                        type="button"
                                                        onClick={() => removeProblem(i)}
                                                        className="p-1 text-gray-400 hover:text-red-500"
                                                        title="Remove"
                                                    >
                                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
                                                    </button>
                                                </div>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}
                    {selectedProblems.length === 0 && (
                        <p className="mt-3 text-sm text-gray-400 text-center py-4">
                            No problems added yet. Search above to add problems.
                        </p>
                    )}
                </section>

                {/* ── Options ── */}
                <section className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6">
                    {sectionLabel('Options')}
                    <div className="space-y-4">
                        <div className="flex items-center gap-6">
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={pdfEnabled}
                                    onChange={e => setPdfEnabled(e.target.checked)}
                                    className="rounded border-gray-300 dark:border-gray-700 text-blue-600 dark:text-blue-400 focus:ring-blue-500"
                                />
                                <span className="text-sm text-gray-700 dark:text-gray-300">Enable PDF statements</span>
                            </label>
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={statementHidden}
                                    onChange={e => setStatementHidden(e.target.checked)}
                                    className="rounded border-gray-300 dark:border-gray-700 text-blue-600 dark:text-blue-400 focus:ring-blue-500"
                                />
                                <span className="text-sm text-gray-700 dark:text-gray-300">Hide statements (onsite mode)</span>
                            </label>
                        </div>
                        <div className="max-w-sm">
                            <label className={labelCls}>Password</label>
                            <input
                                type="text"
                                value={password}
                                onChange={e => setPassword(e.target.value)}
                                placeholder="Leave empty for public contest"
                                className={inputCls}
                            />
                        </div>
                    </div>
                </section>

                {/* ── Submit ── */}
                <div className="flex items-center justify-end gap-3">
                    <button
                        type="button"
                        onClick={() => nav(-1)}
                        className="px-5 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-lg hover:bg-gray-50 transition"
                    >
                        Cancel
                    </button>
                    <button
                        type="submit"
                        disabled={submitting}
                        className="px-6 py-2.5 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition flex items-center gap-2"
                    >
                        {submitting && <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />}
                        {submitting ? 'Creating...' : 'Create Contest'}
                    </button>
                </div>
            </form>
        </div>
    )
}
