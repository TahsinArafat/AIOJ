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
                                ? 'bg-blue-100 dark:bg-blue-900 border-blue-400 text-brand opacity-60 cursor-default'
                                : 'border-hairline-strong text-muted hover:bg-surface-2'}`}
                    >
                        {s}
                    </button>
                ))}
            </div>

            {/* Custom tag input */}
            <div className="flex gap-2">
                <input
                    type="text"
                    aria-label="Add a topic tag"
                    value={input}
                    onChange={e => setInput(e.target.value)}
                    onKeyDown={e => {
                        if (e.key === 'Enter' || e.key === ',') {
                            e.preventDefault()
                            add(input)
                        }
                    }}
                    placeholder="Type a custom tag and press Enter…"
                    className="flex-1 border border-hairline-strong rounded-lg px-3 py-2 text-sm bg-surface text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
                <button
                    type="button"
                    onClick={() => add(input)}
                    className="px-3 py-2 bg-surface-3 border border-hairline-strong rounded-lg text-sm hover:bg-surface-3 transition-colors cursor-pointer"
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
                            className="inline-flex items-center gap-1 px-2 py-0.5 bg-brand text-on-brand rounded-full text-xs"
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
        if (r < 1200) return 'text-muted'
        if (r < 1400) return 'text-green-600 dark:text-green-400'
        if (r < 1600) return 'text-teal-600 dark:text-teal-400'
        if (r < 1900) return 'text-brand'
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
                aria-label={`Rating target: ${value}`}
                min={800}
                max={3500}
                step={100}
                value={value}
                onChange={e => onChange(Number(e.target.value))}
                className="w-full accent-[var(--primary)]"
            />
            <div className="flex justify-between items-center text-sm">
                <span className="text-muted">800</span>
                <span className={`font-bold text-lg ${color(value)}`}>
                    {value} — {label(value)}
                </span>
                <span className="text-muted">3500</span>
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
                                ? 'bg-brand text-on-brand border-brand'
                                : 'border-hairline-strong text-muted hover:bg-surface-2'}`}
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
    const [secondaryTopics, setSecondaryTopics] = useState<string[]>([])
    const [requiredTechniques, setRequiredTechniques] = useState<string[]>([])
    const [forbiddenTechniques, setForbiddenTechniques] = useState<string[]>([])
    const [ratingMin, setRatingMin] = useState(1300)
    const [ratingMax, setRatingMax] = useState(1500)
    const [solverMode, setSolverMode] = useState<'solo' | 'team'>('solo')
    const [teamSize, setTeamSize] = useState(3)
    const [eventContext, setEventContext] = useState<'practice' | 'online' | 'onsite'>('practice')
    const [formatProfile, setFormatProfile] = useState<'general' | 'codeforces' | 'icpc' | 'oi'>('general')
    const [expectedSolveMinutes, setExpectedSolveMinutes] = useState(60)
    const [audienceLevel, setAudienceLevel] = useState('intermediate')
    const [insightEmphasis, setInsightEmphasis] = useState<'insight' | 'implementation' | 'balanced'>('balanced')
    const [storyTone, setStoryTone] = useState('neutral')
    const [constraintGuidance, setConstraintGuidance] = useState('')

    // Generation state
    const [generating, setGenerating] = useState(false)
    const [result, setResult] = useState<{ problem_id: string; problem_slug: string; editorial_id: string } | null>(null)
    const [error, setError] = useState<string | null>(null)

    if (!allowed) {
        return (
            <div className="max-w-md mx-auto mt-20 text-center text-muted">
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
                primary_topics: tags,
                secondary_topics: secondaryTopics,
                required_techniques: requiredTechniques,
                forbidden_techniques: forbiddenTechniques,
                rating_min: ratingMin,
                rating_max: ratingMax,
                solver_mode: solverMode,
                team_size: solverMode === 'team' ? teamSize : undefined,
                event_context: eventContext,
                format_profile: formatProfile,
                expected_solve_minutes: expectedSolveMinutes,
                audience_level: audienceLevel,
                insight_emphasis: insightEmphasis,
                story_tone: storyTone,
                constraint_guidance: constraintGuidance || undefined,
            })
            setResult(res)
        } catch (err) {
            setError((err instanceof Error ? err.message : '') || 'Generation failed.')
        } finally {
            setGenerating(false)
        }
    }

    return (
        <div className="max-w-3xl mx-auto py-8 px-4">
            <div className="mb-8">
                <h1 className="text-3xl font-bold text-ink mb-1">
                    AI Problem Generator
                </h1>
                <p className="text-sm text-muted">
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
                    <p className="text-sm text-muted">
                        Saved as a draft. Review it in the setter workspace and publish when ready.
                    </p>
                    <div className="flex flex-wrap gap-3">
                        <button
                            type="button"
                            onClick={() => navigate(`/setter/${result.problem_slug}`)}
                            className="px-4 py-2 bg-brand hover:bg-brand-hover text-on-brand rounded-lg text-sm font-medium transition-colors cursor-pointer"
                        >
                            Open in Setter Workspace →
                        </button>
                        <button
                            type="button"
                            onClick={() => { setResult(null); setTags([]); setAdditionalPrompt('') }}
                            className="px-4 py-2 border border-hairline-strong rounded-lg text-sm hover:bg-surface-2 transition-colors cursor-pointer"
                        >
                            Generate Another
                        </button>
                    </div>
                    <p className="text-xs text-muted-soft font-mono">
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
                <fieldset className="bg-surface border border-hairline rounded-xl p-5 shadow-sm">
                    <legend className="px-2 text-sm font-semibold text-ink-soft">
                        Algorithmic Topics <span className="text-red-500">*</span>
                    </legend>
                    <p className="text-xs text-muted mb-3">
                        Click chips or type custom tags. The model will weave these topics into the problem.
                    </p>
                    <TagInput tags={tags} onChange={setTags} />
                </fieldset>

                {/* Rating */}
                <fieldset className="bg-surface border border-hairline rounded-xl p-5 shadow-sm">
                    <legend className="px-2 text-sm font-semibold text-ink-soft">
                        Difficulty Rating <span className="text-red-500">*</span>
                    </legend>
                    <p className="text-xs text-muted mb-3">
                        Codeforces-style rating. This calibrates the algorithmic depth and expected solution complexity.
                    </p>
                    <RatingSlider value={rating} onChange={setRating} />
                </fieldset>

                <fieldset className="bg-surface border border-hairline rounded-xl p-5 shadow-sm space-y-4">
                    <legend className="px-2 text-sm font-semibold text-ink-soft">Authoring Profile</legend>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <label className="text-sm text-muted">Format profile
                            <select value={formatProfile} onChange={e => setFormatProfile(e.target.value as typeof formatProfile)} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface">
                                <option value="general">General practice</option><option value="codeforces">Codeforces round</option><option value="icpc">ICPC onsite</option><option value="oi">OI / IOI style</option>
                            </select>
                        </label>
                        <label className="text-sm text-muted">Event context
                            <select value={eventContext} onChange={e => setEventContext(e.target.value as typeof eventContext)} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface">
                                <option value="practice">Practice</option><option value="online">Online contest</option><option value="onsite">Onsite contest</option>
                            </select>
                        </label>
                        <label className="text-sm text-muted">Solver mode
                            <select value={solverMode} onChange={e => setSolverMode(e.target.value as typeof solverMode)} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface">
                                <option value="solo">Solo</option><option value="team">Team</option>
                            </select>
                        </label>
                        {solverMode === 'team' && <label className="text-sm text-muted">Team size<input type="number" min={1} max={8} value={teamSize} onChange={e => setTeamSize(Number(e.target.value))} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>}
                        <label className="text-sm text-muted">Expected solve minutes<input type="number" min={1} max={360} value={expectedSolveMinutes} onChange={e => setExpectedSolveMinutes(Number(e.target.value))} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                        <label className="text-sm text-muted">Audience level<select value={audienceLevel} onChange={e => setAudienceLevel(e.target.value)} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface"><option>beginner</option><option>intermediate</option><option>advanced</option></select></label>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <label className="text-sm text-muted">Minimum acceptable rating<input type="number" min={800} max={3500} step={100} value={ratingMin} onChange={e => setRatingMin(Number(e.target.value))} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                        <label className="text-sm text-muted">Maximum acceptable rating<input type="number" min={800} max={3500} step={100} value={ratingMax} onChange={e => setRatingMax(Number(e.target.value))} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <label className="text-sm text-muted">Insight / implementation emphasis<select value={insightEmphasis} onChange={e => setInsightEmphasis(e.target.value as typeof insightEmphasis)} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface"><option value="balanced">Balanced</option><option value="insight">Insight-heavy</option><option value="implementation">Implementation-heavy</option></select></label>
                        <label className="text-sm text-muted">Story tone<input value={storyTone} onChange={e => setStoryTone(e.target.value)} className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                    </div>
                    <label className="block text-sm text-muted">Constraint guidance<textarea value={constraintGuidance} onChange={e => setConstraintGuidance(e.target.value)} rows={2} placeholder="e.g. N up to 2e5; avoid constraints that make brute force pass" className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                        <label className="text-sm text-muted">Secondary topics<input value={secondaryTopics.join(', ')} onChange={e => setSecondaryTopics(e.target.value.split(',').map(v => v.trim()).filter(Boolean))} placeholder="e.g. arrays, math" className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                        <label className="text-sm text-muted">Required techniques<input value={requiredTechniques.join(', ')} onChange={e => setRequiredTechniques(e.target.value.split(',').map(v => v.trim()).filter(Boolean))} placeholder="e.g. binary search" className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                        <label className="text-sm text-muted">Forbidden techniques<input value={forbiddenTechniques.join(', ')} onChange={e => setForbiddenTechniques(e.target.value.split(',').map(v => v.trim()).filter(Boolean))} placeholder="e.g. brute force" className="mt-1 w-full border rounded-lg px-3 py-2 bg-surface" /></label>
                    </div>
                    <p className="text-xs text-muted">Target {rating} with acceptable range {ratingMin}–{ratingMax}; {solverMode === 'team' ? `team of ${teamSize}` : 'solo'} / {eventContext} / {formatProfile}.</p>
                </fieldset>

                {/* Problem type + test case count */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
                    <fieldset className="bg-surface border border-hairline rounded-xl p-5 shadow-sm">
                        <legend className="px-2 text-sm font-semibold text-ink-soft">Problem Type</legend>
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
                                    <span className="text-sm capitalize text-ink-soft">
                                        {pt.replace('_', ' ')}
                                    </span>
                                </label>
                            ))}
                        </div>
                    </fieldset>

                    <fieldset className="bg-surface border border-hairline rounded-xl p-5 shadow-sm">
                        <legend className="px-2 text-sm font-semibold text-ink-soft">Test Case Count</legend>
                        <p className="text-xs text-muted mt-1 mb-3">
                            Number of example/hidden test cases to describe in the statement (1–100).
                        </p>
                        <input
                            type="number"
                            aria-label="Test case count"
                            min={1}
                            max={100}
                            value={testCaseCount}
                            onChange={e => setTestCaseCount(Math.min(100, Math.max(1, Number(e.target.value))))}
                            className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm bg-surface text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                        />
                        <label className="flex items-center gap-3 mt-4 cursor-pointer">
                            <input
                                type="checkbox"
                                checked={hasSubtasks}
                                onChange={e => setHasSubtasks(e.target.checked)}
                                className="accent-blue-600 w-4 h-4"
                            />
                            <span className="text-sm text-ink-soft">Use subtask scoring (2–4 subtasks)</span>
                        </label>
                    </fieldset>
                </div>

                {/* Additional prompt */}
                <fieldset className="bg-surface border border-hairline rounded-xl p-5 shadow-sm">
                    <legend className="px-2 text-sm font-semibold text-ink-soft">
                        Additional Guidance <span className="text-muted-soft font-normal">(optional)</span>
                    </legend>
                    <p className="text-xs text-muted mb-3">
                        Free-form hint to steer the model. e.g. "grid DP on N×M array", "the key insight involves modular inverses", "avoid trivial brute-force solutions".
                    </p>
                    <textarea
                        aria-label="Additional instructions for the model"
                        value={additionalPrompt}
                        onChange={e => setAdditionalPrompt(e.target.value)}
                        rows={3}
                        placeholder="Describe any specific scenario, constraint, or insight you want the model to incorporate…"
                        className="w-full border border-hairline-strong rounded-lg px-3 py-2 text-sm bg-surface text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] resize-none"
                    />
                </fieldset>

                {/* Submit */}
                <div className="flex items-center gap-4">
                    <button
                        type="submit"
                        disabled={generating || tags.length === 0}
                        className="px-6 py-3 bg-brand hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed text-on-brand rounded-xl font-semibold text-sm transition-colors cursor-pointer flex items-center gap-2"
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
                        <p className="text-sm text-muted italic">
                            The AI is writing the problem, solution, and editorial. This takes ~15–60 s.
                        </p>
                    )}
                </div>

                {/* Info panel */}
                <div className="text-xs text-muted-soft border-t border-hairline pt-4 space-y-1">
                    <p>• Generated problems are saved as <strong>drafts</strong> (not visible to contestants) until you publish them.</p>
                    <p>• A reference solution and editorial are generated alongside the problem statement.</p>
                    <p>• You can edit every field in the Setter Workspace after generation.</p>
                    <p>• Only <strong>admin</strong> and <strong>setter</strong> roles can access this page.</p>
                </div>
            </form>
        </div>
    )
}
