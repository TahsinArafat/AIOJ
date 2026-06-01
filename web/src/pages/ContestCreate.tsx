import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import { DIVISIONS } from '../lib/divisions'

const FORMAT_OPTIONS = [
    { value: 'acm', label: 'ACM/ICPC', desc: 'Standard penalty-based scoring.' },
    { value: 'oi', label: 'OI', desc: 'Max score per problem. No penalty.' },
    { value: 'ioi', label: 'IOI', desc: 'Subtask-based scoring with partial credit.' },
    { value: 'atcoder', label: 'AtCoder', desc: 'Time of first AC as penalty.' },
    { value: 'codeforces', label: 'Codeforces', desc: 'Dynamic score decaying over time.' },
]

export default function ContestCreate() {
    const nav = useNavigate()
    const [form, setForm] = useState({
        title: '',
        slug: '',
        type: 'acm',
        format: 'acm',
        penalty_per_wrong: '20',
        max_score: '100',
        decay_factor: '250',
        min_ratio: '0.3',
        cf_penalty: '50',
        start_time: '',
        end_time: '',
        freeze_time: '',
        password: '',
        description: '',
        problem_ids: '',
        division: '0',
        pdf_enabled: true,
        statement_hidden: false,
    })
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [showAdvanced, setShowAdvanced] = useState(false)
    const [showScoring, setShowScoring] = useState(false)

    useEffect(() => {
        if (!getAccessToken()) nav('/login')
    }, [nav])

    const handleChange = (field: string, value: string | boolean) => {
        setForm(p => ({ ...p, [field]: value }))
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

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        const err = validate()
        if (err) { setError(err); return }
        setError('')
        setSubmitting(true)
        try {
            const problemIds = form.problem_ids
                .split(',')
                .map(s => s.trim())
                .filter(Boolean)

            let formatConfig: any = {}
            if (form.format === 'acm') {
                formatConfig = { penalty_per_wrong: Number(form.penalty_per_wrong), time_penalty: true }
            } else if (form.format === 'oi') {
                formatConfig = { max_score_per_problem: Number(form.max_score) }
            } else if (form.format === 'ioi') {
                formatConfig = { partial_credit: true, subtask_scoring: true }
            } else if (form.format === 'atcoder') {
                formatConfig = { penalty_is_time_of_ac: true, no_wrong_attempt_penalty: true }
            } else if (form.format === 'codeforces') {
                formatConfig = {
                    initial_scores: [500, 1000, 1500, 2000, 2500],
                    decay_factor: Number(form.decay_factor),
                    min_score_ratio: Number(form.min_ratio),
                    wrong_submission_penalty: Number(form.cf_penalty),
                }
            }

            const contest = await api.contests.create({
                title: form.title,
                slug: form.slug || undefined,
                type: form.type,
                format: form.format,
                format_config: formatConfig,
                start_time: new Date(form.start_time).toISOString(),
                end_time: new Date(form.end_time).toISOString(),
                freeze_time: form.freeze_time ? new Date(form.freeze_time).toISOString() : undefined,
                password: form.password || undefined,
                description: form.description || undefined,
                problem_ids: problemIds.length > 0 ? problemIds : undefined,
                division: Number(form.division),
                pdf_enabled: form.pdf_enabled,
                statement_hidden: form.statement_hidden,
            })
            nav(`/contests/${contest.id}`)
        } catch (err: any) {
            setError(err.message || 'Failed to create contest')
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Create Contest</h1>
            {error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{error}</div>}
            
            <form onSubmit={handleSubmit} className="space-y-5">
                {/* Essential Fields */}
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Contest Title</label>
                        <input required value={form.title} onChange={e => handleChange('title', e.target.value)}
                            placeholder="e.g. AIOJ Round 1"
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">Start Time</label>
                            <input type="datetime-local" required value={form.start_time} onChange={e => handleChange('start_time', e.target.value)}
                                className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">End Time</label>
                            <input type="datetime-local" required value={form.end_time} onChange={e => handleChange('end_time', e.target.value)}
                                className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                        </div>
                    </div>

                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Problem IDs <span className="text-gray-400 font-normal">(comma-separated)</span></label>
                        <input value={form.problem_ids} onChange={e => handleChange('problem_ids', e.target.value)}
                            placeholder="e.g. p1, p2, p3"
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                </div>

                {/* Scoring & Format - Collapsible */}
                <div className="border border-gray-200 rounded-lg">
                    <button
                        type="button"
                        onClick={() => setShowScoring(!showScoring)}
                        className="w-full flex items-center justify-between px-4 py-3 text-left hover:bg-gray-50"
                    >
                        <span className="text-sm font-medium text-gray-700">Scoring & Format</span>
                        <svg className={`w-4 h-4 text-gray-500 transition-transform ${showScoring ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                    </button>
                    {showScoring && (
                        <div className="px-4 pb-4 space-y-3 border-t border-gray-200 pt-3">
                            <div className="grid grid-cols-3 gap-3">
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Type</label>
                                    <select value={form.type} onChange={e => handleChange('type', e.target.value)}
                                        className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none">
                                        <option value="acm">ACM</option>
                                        <option value="oi">OI</option>
                                        <option value="ioi">IOI</option>
                                        <option value="practice">Practice</option>
                                        <option value="educational">Educational</option>
                                    </select>
                                </div>
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Scoring</label>
                                    <select value={form.format} onChange={e => handleChange('format', e.target.value)}
                                        className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none">
                                        {FORMAT_OPTIONS.map(opt => (
                                            <option key={opt.value} value={opt.value}>{opt.label}</option>
                                        ))}
                                    </select>
                                </div>
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Division</label>
                                    <select value={form.division} onChange={e => handleChange('division', e.target.value)}
                                        className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none">
                                        {(Object.entries(DIVISIONS) as [string, { name: string }][]).map(([key, info]) => (
                                            <option key={key} value={key}>{info.name}</option>
                                        ))}
                                    </select>
                                </div>
                            </div>

                            {form.format === 'acm' && (
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Penalty Per Wrong (min)</label>
                                    <input type="number" value={form.penalty_per_wrong} onChange={e => handleChange('penalty_per_wrong', e.target.value)}
                                        className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                </div>
                            )}
                            {form.format === 'oi' && (
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Max Score Per Problem</label>
                                    <input type="number" value={form.max_score} onChange={e => handleChange('max_score', e.target.value)}
                                        className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                </div>
                            )}
                            {form.format === 'codeforces' && (
                                <div className="grid grid-cols-3 gap-2">
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">Decay</label>
                                        <input type="number" value={form.decay_factor} onChange={e => handleChange('decay_factor', e.target.value)}
                                            className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">Min Ratio</label>
                                        <input type="number" step="0.05" value={form.min_ratio} onChange={e => handleChange('min_ratio', e.target.value)}
                                            className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-500 mb-1">Penalty</label>
                                        <input type="number" value={form.cf_penalty} onChange={e => handleChange('cf_penalty', e.target.value)}
                                            className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                    </div>
                                </div>
                            )}
                        </div>
                    )}
                </div>

                {/* Advanced Settings - Collapsible */}
                <div className="border border-gray-200 rounded-lg">
                    <button
                        type="button"
                        onClick={() => setShowAdvanced(!showAdvanced)}
                        className="w-full flex items-center justify-between px-4 py-3 text-left hover:bg-gray-50"
                    >
                        <span className="text-sm font-medium text-gray-700">Advanced Settings</span>
                        <svg className={`w-4 h-4 text-gray-500 transition-transform ${showAdvanced ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                    </button>
                    {showAdvanced && (
                        <div className="px-4 pb-4 space-y-3 border-t border-gray-200 pt-3">
                            <div>
                                <label className="block text-xs font-medium text-gray-500 mb-1">Custom URL Slug</label>
                                <div className="flex items-center gap-1">
                                    <span className="text-xs text-gray-400">/contests/</span>
                                    <input value={form.slug} onChange={e => handleChange('slug', e.target.value)}
                                        placeholder="icpc-dhaka-2024"
                                        pattern="[a-z0-9-]+"
                                        className="flex-1 border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-3">
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Freeze Time</label>
                                    <input type="datetime-local" value={form.freeze_time} onChange={e => handleChange('freeze_time', e.target.value)}
                                        className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                </div>
                                <div>
                                    <label className="block text-xs font-medium text-gray-500 mb-1">Password</label>
                                    <input type="text" value={form.password} onChange={e => handleChange('password', e.target.value)}
                                        placeholder="Empty = public"
                                        className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                                </div>
                            </div>

                            <div>
                                <label className="block text-xs font-medium text-gray-500 mb-1">Description</label>
                                <textarea rows={2} value={form.description} onChange={e => handleChange('description', e.target.value)}
                                    placeholder="Optional contest description..."
                                    className="w-full border border-gray-300 rounded-md px-2 py-1.5 text-sm focus:outline-none" />
                            </div>

                            <div className="flex items-center gap-4 pt-1">
                                <label className="flex items-center gap-1.5 cursor-pointer">
                                    <input type="checkbox" checked={form.pdf_enabled}
                                        onChange={e => setForm(p => ({ ...p, pdf_enabled: e.target.checked }))}
                                        className="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
                                    <span className="text-xs text-gray-600">PDF enabled</span>
                                </label>
                                <label className="flex items-center gap-1.5 cursor-pointer">
                                    <input type="checkbox" checked={form.statement_hidden}
                                        onChange={e => setForm(p => ({ ...p, statement_hidden: e.target.checked }))}
                                        className="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
                                    <span className="text-xs text-gray-600">Hide statements (onsite)</span>
                                </label>
                            </div>
                        </div>
                    )}
                </div>

                <button type="submit" disabled={submitting}
                    className="w-full bg-blue-600 text-white px-6 py-2.5 rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors font-medium">
                    {submitting ? 'Creating...' : 'Create Contest'}
                </button>
            </form>
        </div>
    )
}
