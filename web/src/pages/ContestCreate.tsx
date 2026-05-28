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
    })
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [showPreview, setShowPreview] = useState(false)

    useEffect(() => {
        if (!getAccessToken()) nav('/login')
    }, [nav])

    const handleChange = (field: string, value: string) => {
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

            // Construct format config
            let formatConfig: any = {}
            if (form.format === 'acm') {
                formatConfig = {
                    penalty_per_wrong: Number(form.penalty_per_wrong),
                    time_penalty: true,
                }
            } else if (form.format === 'oi') {
                formatConfig = {
                    max_score_per_problem: Number(form.max_score),
                }
            } else if (form.format === 'ioi') {
                formatConfig = {
                    partial_credit: true,
                    subtask_scoring: true,
                }
            } else if (form.format === 'atcoder') {
                formatConfig = {
                    penalty_is_time_of_ac: true,
                    no_wrong_attempt_penalty: true,
                }
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
            })
            nav(`/contests/${contest.id}`)
        } catch (err: any) {
            setError(err.message || 'Failed to create contest')
        } finally {
            setSubmitting(false)
        }
    }

    const previewProblems = form.problem_ids
        .split(',')
        .map(s => s.trim())
        .filter(Boolean)

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Create Contest</h1>
            {error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{error}</div>}
            <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
                    <input required value={form.title} onChange={e => handleChange('title', e.target.value)}
                        placeholder="e.g. AIOJ Round 1"
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>
                <div className="grid grid-cols-3 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Type</label>
                        <select value={form.type} onChange={e => handleChange('type', e.target.value)}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
                            <option value="acm">ACM</option>
                            <option value="oi">OI</option>
                            <option value="ioi">IOI</option>
                            <option value="practice">Practice</option>
                            <option value="educational">Educational</option>
                        </select>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Scoring Format</label>
                        <select value={form.format} onChange={e => handleChange('format', e.target.value)}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
                            {FORMAT_OPTIONS.map(opt => (
                                <option key={opt.value} value={opt.value}>{opt.label}</option>
                            ))}
                        </select>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Division</label>
                        <select value={form.division} onChange={e => handleChange('division', e.target.value)}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
                            {(Object.entries(DIVISIONS) as [string, { name: string }][]).map(([key, info]) => (
                                <option key={key} value={key}>{info.name}</option>
                            ))}
                        </select>
                    </div>
                </div>

                {/* Dynamic scoring settings */}
                <div className="border border-gray-200 rounded-lg p-4 bg-gray-50 space-y-3">
                    <h3 className="font-semibold text-sm text-gray-700">Format-Specific Settings</h3>
                    {form.format === 'acm' && (
                        <div>
                            <label className="block text-xs font-medium text-gray-500 mb-1">Penalty Per Wrong Attempt (minutes)</label>
                            <input type="number" value={form.penalty_per_wrong} onChange={e => handleChange('penalty_per_wrong', e.target.value)}
                                className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:outline-none" />
                        </div>
                    )}
                    {form.format === 'oi' && (
                        <div>
                            <label className="block text-xs font-medium text-gray-500 mb-1">Max Score Per Problem</label>
                            <input type="number" value={form.max_score} onChange={e => handleChange('max_score', e.target.value)}
                                className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:outline-none" />
                        </div>
                    )}
                    {form.format === 'codeforces' && (
                        <div className="grid grid-cols-3 gap-3">
                            <div>
                                <label className="block text-xs font-medium text-gray-500 mb-1">Decay Factor</label>
                                <input type="number" value={form.decay_factor} onChange={e => handleChange('decay_factor', e.target.value)}
                                    className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:outline-none" />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-500 mb-1">Min Score Ratio</label>
                                <input type="number" step="0.05" value={form.min_ratio} onChange={e => handleChange('min_ratio', e.target.value)}
                                    className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:outline-none" />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-500 mb-1">Penalty (pts)</label>
                                <input type="number" value={form.cf_penalty} onChange={e => handleChange('cf_penalty', e.target.value)}
                                    className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:outline-none" />
                            </div>
                        </div>
                    )}
                    {(form.format === 'ioi' || form.format === 'atcoder') && (
                        <div className="text-xs text-gray-500">No additional parameters needed for {form.format.toUpperCase()} scoring.</div>
                    )}
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
                <div className="grid grid-cols-2 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Freeze Time <span className="text-gray-400 font-normal">(optional)</span></label>
                        <input type="datetime-local" value={form.freeze_time} onChange={e => handleChange('freeze_time', e.target.value)}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Password <span className="text-gray-400 font-normal">(optional)</span></label>
                        <input type="text" value={form.password} onChange={e => handleChange('password', e.target.value)}
                            placeholder="Leave empty for public"
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Problem IDs <span className="text-gray-400 font-normal">(comma-separated)</span></label>
                    <input value={form.problem_ids} onChange={e => handleChange('problem_ids', e.target.value)}
                        placeholder="e.g. p1, p2, p3"
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Description <span className="text-gray-400 font-normal">(optional)</span></label>
                    <textarea rows={4} value={form.description} onChange={e => handleChange('description', e.target.value)}
                        placeholder="Contest description or rules..."
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>

                <div className="flex gap-3">
                    <button type="submit" disabled={submitting}
                        className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700 disabled:opacity-50 transition-colors">
                        {submitting ? 'Creating...' : 'Create Contest'}
                    </button>
                    <button type="button" onClick={() => setShowPreview(!showPreview)}
                        className="border border-gray-300 px-4 py-2 rounded text-sm hover:bg-gray-50 transition-colors">
                        {showPreview ? 'Hide Preview' : 'Preview'}
                    </button>
                </div>
            </form>

            {showPreview && (
                <div className="mt-6 border border-gray-200 rounded-lg p-5 bg-gray-50">
                    <h2 className="text-lg font-semibold mb-3">Contest Preview</h2>
                    <div className="space-y-2 text-sm">
                        <div><span className="text-gray-500">Title:</span> <span className="font-medium">{form.title || '—'}</span></div>
                        <div><span className="text-gray-500">Type:</span> <span className="uppercase font-medium">{form.type}</span></div>
                        <div><span className="text-gray-500">Format:</span> <span className="uppercase font-medium">{form.format}</span></div>
                        <div><span className="text-gray-500">Division:</span> <span className="font-medium">{DIVISIONS[Number(form.division) as keyof typeof DIVISIONS]?.name ?? 'Open'}</span></div>
                        <div><span className="text-gray-500">Start:</span> <span className="font-medium">{form.start_time ? new Date(form.start_time).toLocaleString() : '—'}</span></div>
                        <div><span className="text-gray-500">End:</span> <span className="font-medium">{form.end_time ? new Date(form.end_time).toLocaleString() : '—'}</span></div>
                        {form.freeze_time && <div><span className="text-gray-500">Freeze:</span> <span className="font-medium">{new Date(form.freeze_time).toLocaleString()}</span></div>}
                        {form.password && <div><span className="text-gray-500">Password:</span> <span className="font-medium text-orange-600">Set</span></div>}
                        {form.description && <div><span className="text-gray-500">Description:</span> <span>{form.description}</span></div>}
                        <div>
                            <span className="text-gray-500">Problems:</span>{' '}
                            {previewProblems.length > 0
                                ? previewProblems.map((p, i) => <span key={i} className="inline-block bg-blue-100 text-blue-700 text-xs px-2 py-0.5 rounded mr-1 font-medium">{p}</span>)
                                : <span className="text-gray-400">None</span>}
                        </div>
                        {form.start_time && form.end_time && (
                            <div>
                                <span className="text-gray-500">Duration:</span>{' '}
                                <span className="font-medium">
                                    {Math.round((new Date(form.end_time).getTime() - new Date(form.start_time).getTime()) / 60000)} minutes
                                </span>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    )
}
