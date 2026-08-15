import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

// ── Common topic tags surfaced as quick-pick chips ──────────────────────────
const TOPIC_SUGGESTIONS = [
    'dp', 'graphs', 'greedy', 'binary search', 'two pointers',
    'math', 'number theory', 'combinatorics', 'strings', 'trees',
    'segment trees', 'bit manipulation', 'sorting', 'bfs', 'dfs',
    'shortest paths', 'flows', 'geometry', 'hashing', 'constructive',
]

const PROBLEM_TYPES = ['standard', 'interactive', 'output_only'] as const

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

// ── Tag input component ─────────────────────────────────────────────────────
function TagInput({
    tags,
    onChange,
}: {
    tags: string[]
    onChange: (t: string[]) => void
}) {
    const [input, setInput] = useState('')

    const add = (tag: string) => {
        const t = tag.trim().toLowerCase()
        if (t && !tags.includes(t)) onChange([...tags, t])
        setInput('')
    }

    const remove = (tag: string) => onChange(tags.filter(t => t !== tag))

    return (
        <div className="space-y-2">
            {/* Chip suggestions */}
            <div className="flex flex-wrap gap-2">
                {TOPIC_SUGGESTIONS.map(s => (
                    <button
                        key={s}
                        type="button"
                        onClick={() => add(s)}
                        disabled={tags.includes(s)}
                        className={`px-2 py-0.5 rounded-full text-xs border transition-colors cursor-pointer
                            ${tags.includes(s)
                                ? 'bg-blue-100 dark:bg-blue-900 border-blue-400 text-blue-700 dark:text-blue-300 opacity-60 cursor-default'
                                : 'border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'}`}
                    >
                        {s}
                    </button>
                ))}
            </div>

            {/* Custom tag input */}
            <div className="flex gap-2">
                <input
                    type="text"
                    value={input}
                    onChange={e => setInput(e.target.value)}
                    onKeyDown={e => {
                        if (e.key === 'Enter' || e.key === ',') {
                            e.preventDefault()
                            add(input)
                        }
                    }}
                    placeholder="Type a custom tag and press Enter…"
                    className="flex-1 border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <button
                    type="button"
                    onClick={() => add(input)}
                    className="px-3 py-2 bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg text-sm hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors cursor-pointer"
                >
                    Add
                </button>
            </div>

            {/* Selected tags */}
            {tags.length > 0 && (
                <div className="flex flex-wrap gap-2 pt-1">
                    {tags.map(t => (
                        <span
                            key={t}
                            className="inline-flex items-center gap-1 px-2 py-0.5 bg-blue-600 text-white rounded-full text-xs"
                        >
                            {t}
                            <button
                                type="button"
                                onClick={() => remove(t)}
                                className="hover:text-blue-200 leading-none cursor-pointer"
                            >
                                ×
                            </button>
                        </span>
                    ))}
                </div>
            )}
        </div>
    )
}

// ── Rating slider ───────────────────────────────────────────────────────────
function RatingSlider({
    value,
    onChange,
}: {
    value: number
    onChange: (v: number) => void
}) {
    // Canonical CF rating bands
    const bands = [800, 900, 1000, 1100, 1200, 1300, 1400, 1500, 1600, 1700, 1800, 1900, 2000, 2100, 2200, 2300, 2400, 2500, 2600, 2700, 2800, 2900, 3000, 3200, 3500]

    function color(r: number) {
        if (r < 1200) return 'text-gray-500'
        if (r < 1400) return 'text-green-600 dark:text-green-400'
        if (r < 1600) return 'text-teal-600 dark:text-teal-400'
        if (r < 1900) return 'text-blue-600 dark:text-blue-400'
        if (r < 2100) return 'text-violet-600 dark:text-violet-400'
        if (r < 2300) return 'text-orange-500'
        if (r < 2400) return 'text-orange-600'
        return 'text-red-600 dark:text-red-400'
    }

    function label(r: number) {
        if (r < 1200) return 'Newbie'
        if (r < 1400) return 'Pupil'
        if (r < 1600) return 'Specialist'
        if (r < 1900) return 'Expert'
        if (r < 2100) return 'Candidate Master'
        if (r < 2300) return 'Master'
        if (r < 2400) return 'International Master'
        if (r < 2600) return 'Grandmaster'
        if (r < 3000) return 'International Grandmaster'
        return 'Legendary Grandmaster'
    }

    return (
        <div className="space-y-2">
            <input
                type="range"
                min={800}
                max={3500}
                step={100}
                value={value}
                onChange={e => onChange(Number(e.target.value))}
                className="w-full accent-blue-600"
            />
            <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500 dark:text-gray-400">800</span>
                <span className={`font-bold text-lg ${color(value)}`}>
                    {value} — {label(value)}
                </span>
                <span className="text-gray-500 dark:text-gray-400">3500</span>
            </div>
            {/* Quick-pick buttons */}
            <div className="flex flex-wrap gap-1 pt-1">
                {bands.map(b => (
                    <button
                        key={b}
                        type="button"
                        onClick={() => onChange(b)}
                        className={`px-2 py-0.5 rounded text-xs border cursor-pointer transition-colors
                            ${value === b
                                ? 'bg-blue-600 text-white border-blue-600'
                                : 'border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'}`}
                    >
                        {b}
                    </button>
                ))}
            </div>
        </div>
    )
}

// ── Main page ───────────────────────────────────────────────────────────────
export default function GenerateProblem() {
    const navigate = useNavigate()
    const role = decodeRole()
    const allowed = role === 'admin' || role === 'setter'

    // Form state
    const [tags, setTags] = useState<string[]>([])
    const [rating, setRating] = useState(1400)
    const [problemType, setProblemType] = useState<typeof PROBLEM_TYPES[number]>('standard')
    const [testCaseCount, setTestCaseCount] = useState(10)
    const [hasSubtasks, setHasSubtasks] = useState(false)
    const [additionalPrompt, setAdditionalPrompt] = useState('')

    // Generation state
    const [generating, setGenerating] = useState(false)
    const [result, setResult] = useState<{ problem_id: string; problem_slug: string; editorial_id: string } | null>(null)
    const [error, setError] = useState<string | null>(null)

    if (!allowed) {
        return (
            <div className="max-w-md mx-auto mt-20 text-center text-gray-500 dark:text-gray-400">
                <p className="text-lg font-semibold mb-2">Access Restricted</p>
                <p className="text-sm">Only admins and setters can generate problems.</p>
            </div>
        )
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (tags.length === 0) return setError('Please select at least one tag.')
        setError(null)
        setResult(null)
        setGenerating(true)

        try {
            const res = await api.generate.problem({
                tags,
                rating,
                problem_type: problemType,
                testcase_count: testCaseCount,
                has_subtasks: hasSubtasks,
                additional_prompt: additionalPrompt || undefined,
            })
            setResult(res)
        } catch (err: any) {
            setError(err.message || 'Generation failed.')
        } finally {
            setGenerating(false)
        }
    }

    return (
        <div className="max-w-3xl mx-auto py-8 px-4">
            <div className="mb-8">
                <h1 className="text-3xl font-bold text-gray-900 dark:text-gray-100 mb-1">
                    AI Problem Generator
                </h1>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                    Generates a complete Codeforces-style problem — statement, solution, editorial — and saves it as a <span className="font-medium text-yellow-600 dark:text-yellow-400">draft</span> for your review.
                </p>
            </div>

            {/* ── Result banner ─────────────────────────────────────────────── */}
            {result && (
                <div className="mb-6 bg-green-50 dark:bg-green-900/20 border border-green-300 dark:border-green-700 rounded-xl p-5 space-y-3">
                    <div className="flex items-center gap-2">
                        <span className="text-green-600 dark:text-green-400 text-xl">✓</span>
                        <p className="font-semibold text-green-700 dark:text-green-300">Problem generated successfully!</p>
                    </div>
                    <p className="text-sm text-gray-600 dark:text-gray-300">
                        Saved as a draft. Review it in the setter workspace and publish when ready.
                    </p>
                    <div className="flex flex-wrap gap-3">
                        <button
                            type="button"
                            onClick={() => navigate(`/setter/${result.problem_slug}`)}
                            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors cursor-pointer"
                        >
                            Open in Setter Workspace →
                        </button>
                        <button
                            type="button"
                            onClick={() => { setResult(null); setTags([]); setAdditionalPrompt('') }}
                            className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer"
                        >
                            Generate Another
                        </button>
                    </div>
                    <p className="text-xs text-gray-400 dark:text-gray-500 font-mono">
                        problem_id: {result.problem_id} · slug: {result.problem_slug}
                    </p>
                </div>
            )}

            {/* ── Error banner ───────────────────────────────────────────────── */}
            {error && (
                <div className="mb-6 bg-red-50 dark:bg-red-900/20 border border-red-300 dark:border-red-700 rounded-xl p-4 text-sm text-red-700 dark:text-red-300">
                    <span className="font-semibold">Error: </span>{error}
                </div>
            )}

            {/* ── Form ──────────────────────────────────────────────────────── */}
            <form onSubmit={handleSubmit} className="space-y-7">

                {/* Tags */}
                <fieldset className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl p-5 shadow-sm">
                    <legend className="px-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
                        Algorithmic Topics <span className="text-red-500">*</span>
                    </legend>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                        Click chips or type custom tags. The model will weave these topics into the problem.
                    </p>
                    <TagInput tags={tags} onChange={setTags} />
                </fieldset>

                {/* Rating */}
                <fieldset className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl p-5 shadow-sm">
                    <legend className="px-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
                        Difficulty Rating <span className="text-red-500">*</span>
                    </legend>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                        Codeforces-style rating. This calibrates the algorithmic depth and expected solution complexity.
                    </p>
                    <RatingSlider value={rating} onChange={setRating} />
                </fieldset>

                {/* Problem type + test case count */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
                    <fieldset className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl p-5 shadow-sm">
                        <legend className="px-2 text-sm font-semibold text-gray-700 dark:text-gray-300">Problem Type</legend>
                        <div className="mt-2 space-y-2">
                            {PROBLEM_TYPES.map(pt => (
                                <label key={pt} className="flex items-center gap-3 cursor-pointer">
                                    <input
                                        type="radio"
                                        name="problem_type"
                                        value={pt}
                                        checked={problemType === pt}
                                        onChange={() => setProblemType(pt)}
                                        className="accent-blue-600"
                                    />
                                    <span className="text-sm capitalize text-gray-700 dark:text-gray-300">
                                        {pt.replace('_', ' ')}
                                    </span>
                                </label>
                            ))}
                        </div>
                    </fieldset>

                    <fieldset className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl p-5 shadow-sm">
                        <legend className="px-2 text-sm font-semibold text-gray-700 dark:text-gray-300">Test Case Count</legend>
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 mb-3">
                            Number of example/hidden test cases to describe in the statement (1–100).
                        </p>
                        <input
                            type="number"
                            min={1}
                            max={100}
                            value={testCaseCount}
                            onChange={e => setTestCaseCount(Math.min(100, Math.max(1, Number(e.target.value))))}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                        <label className="flex items-center gap-3 mt-4 cursor-pointer">
                            <input
                                type="checkbox"
                                checked={hasSubtasks}
                                onChange={e => setHasSubtasks(e.target.checked)}
                                className="accent-blue-600 w-4 h-4"
                            />
                            <span className="text-sm text-gray-700 dark:text-gray-300">Use subtask scoring (2–4 subtasks)</span>
                        </label>
                    </fieldset>
                </div>

                {/* Additional prompt */}
                <fieldset className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl p-5 shadow-sm">
                    <legend className="px-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
                        Additional Guidance <span className="text-gray-400 font-normal">(optional)</span>
                    </legend>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                        Free-form hint to steer the model. e.g. "grid DP on N×M array", "the key insight involves modular inverses", "avoid trivial brute-force solutions".
                    </p>
                    <textarea
                        value={additionalPrompt}
                        onChange={e => setAdditionalPrompt(e.target.value)}
                        rows={3}
                        placeholder="Describe any specific scenario, constraint, or insight you want the model to incorporate…"
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                    />
                </fieldset>

                {/* Submit */}
                <div className="flex items-center gap-4">
                    <button
                        type="submit"
                        disabled={generating || tags.length === 0}
                        className="px-6 py-3 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl font-semibold text-sm transition-colors cursor-pointer flex items-center gap-2"
                    >
                        {generating ? (
                            <>
                                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4l3-3-3-3v4a8 8 0 00-8 8h4z" />
                                </svg>
                                Generating…
                            </>
                        ) : (
                            '✦ Generate Problem'
                        )}
                    </button>
                    {generating && (
                        <p className="text-sm text-gray-500 dark:text-gray-400 italic">
                            The AI is writing the problem, solution, and editorial. This takes ~15–60 s.
                        </p>
                    )}
                </div>

                {/* Info panel */}
                <div className="text-xs text-gray-400 dark:text-gray-500 border-t border-gray-200 dark:border-gray-700 pt-4 space-y-1">
                    <p>• Generated problems are saved as <strong>drafts</strong> (not visible to contestants) until you publish them.</p>
                    <p>• A reference solution and editorial are generated alongside the problem statement.</p>
                    <p>• You can edit every field in the Setter Workspace after generation.</p>
                    <p>• Only <strong>admin</strong> and <strong>setter</strong> roles can access this page.</p>
                </div>
            </form>
        </div>
    )
}
