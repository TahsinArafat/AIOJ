import { useState } from 'react'
import type { AIBrief } from '../../types/api'
import { ratingColor, ratingLabel, withRange, withRating } from '../../lib/aiBrief'

/**
 * The authoring brief shared by every AI generation surface.
 *
 * These controls previously lived only on the legacy `/generate/problem` page,
 * which the harness replaced. Deleting that page would have silently dropped the
 * operator's ability to steer the model (topics, rating calibration, ICPC vs
 * Codeforces calibration, forbidden techniques, …), so the whole set lives here
 * and is used by both AI Studio and the standalone "new problem" page.
 *
 * The backend already renders every field into the model's brief
 * (internal/harness/problemgen/brief.go Brief.Render), so the form was the only
 * place these options were missing.
 */

const TOPIC_SUGGESTIONS = [
    'dp', 'graphs', 'greedy', 'binary search', 'two pointers',
    'math', 'number theory', 'combinatorics', 'strings', 'trees',
    'segment trees', 'bit manipulation', 'sorting', 'bfs', 'dfs',
    'shortest paths', 'flows', 'geometry', 'hashing', 'constructive',
]

/** Canonical Codeforces rating steps, used for the quick-pick row. */
const RATING_BANDS = [
    800, 900, 1000, 1100, 1200, 1300, 1400, 1500, 1600, 1700,
    1800, 1900, 2000, 2100, 2200, 2300, 2400, 2500, 2600, 2700,
    2800, 2900, 3000, 3200, 3500,
]

function TagInput({
    tags,
    onChange,
    disabled,
    label,
    placeholder,
}: {
    tags: string[]
    onChange: (t: string[]) => void
    disabled?: boolean
    label: string
    placeholder: string
}) {
    const [input, setInput] = useState('')

    const add = (tag: string) => {
        const t = tag.trim().toLowerCase()
        if (t && !tags.includes(t)) onChange([...tags, t])
        setInput('')
    }

    return (
        <div className="space-y-2">
            <div className="flex flex-wrap gap-1.5">
                {TOPIC_SUGGESTIONS.map(s => (
                    <button
                        key={s}
                        type="button"
                        disabled={disabled || tags.includes(s)}
                        onClick={() => add(s)}
                        className={`cursor-pointer rounded-full border px-2 py-0.5 text-xs transition-colors disabled:cursor-default
                            ${tags.includes(s)
                                ? 'border-brand/50 bg-brand-soft text-brand opacity-60'
                                : 'border-hairline-strong text-muted hover:bg-surface-2'}`}
                    >
                        {s}
                    </button>
                ))}
            </div>

            <div className="flex gap-2">
                <input
                    type="text"
                    aria-label={label}
                    value={input}
                    disabled={disabled}
                    onChange={e => setInput(e.target.value)}
                    onKeyDown={e => {
                        if (e.key === 'Enter' || e.key === ',') {
                            e.preventDefault()
                            add(input)
                        }
                    }}
                    placeholder={placeholder}
                    className="min-h-11 flex-1 rounded-lg border border-hairline-strong bg-surface px-3 py-2 text-sm text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
                <button
                    type="button"
                    disabled={disabled}
                    onClick={() => add(input)}
                    className="min-h-11 cursor-pointer rounded-lg border border-hairline-strong bg-surface-3 px-3 py-2 text-sm hover:bg-surface-2"
                >
                    Add
                </button>
            </div>

            {tags.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1">
                    {tags.map(t => (
                        <span key={t} className="inline-flex items-center gap-1 rounded-full bg-brand px-2 py-0.5 text-xs text-on-brand">
                            {t}
                            <button
                                type="button"
                                aria-label={`Remove ${t}`}
                                disabled={disabled}
                                onClick={() => onChange(tags.filter(x => x !== t))}
                                className="cursor-pointer leading-none hover:text-canvas"
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

function RatingSlider({ value, onChange, disabled }: { value: number; onChange: (v: number) => void; disabled?: boolean }) {
    return (
        <div className="space-y-2">
            <input
                type="range"
                aria-label={`Rating target: ${value}`}
                min={800}
                max={3500}
                step={100}
                value={value}
                disabled={disabled}
                onChange={e => onChange(Number(e.target.value))}
                className="w-full accent-[var(--primary)]"
            />
            <div className="flex items-center justify-between text-sm">
                <span className="text-muted">800</span>
                <span className={`text-lg font-bold ${ratingColor(value)}`}>{value} — {ratingLabel(value)}</span>
                <span className="text-muted">3500</span>
            </div>
            <div className="flex flex-wrap gap-1 pt-1">
                {RATING_BANDS.map(b => (
                    <button
                        key={b}
                        type="button"
                        disabled={disabled}
                        onClick={() => onChange(b)}
                        className={`cursor-pointer rounded border px-2 py-0.5 text-xs transition-colors disabled:cursor-default
                            ${value === b
                                ? 'border-brand bg-brand text-on-brand'
                                : 'border-hairline-strong text-muted hover:bg-surface-2'}`}
                    >
                        {b}
                    </button>
                ))}
            </div>
        </div>
    )
}

const field = 'mt-1 min-h-11 w-full rounded-lg border border-hairline-strong bg-surface px-3 py-2 text-sm text-ink'

export function AIBriefForm({
    brief,
    onChange,
    disabled,
}: {
    brief: AIBrief
    onChange: (patch: Partial<AIBrief>) => void
    disabled?: boolean
}) {
    const profile = brief.format_profile ?? 'general'
    // The backend rejects ICPC + subtasks ("ICPC profile does not support
    // subtasks") and forces team/onsite for ICPC and solo for CF/OI, so the
    // dependent controls reflect what will actually be sent.
    const icpc = profile === 'icpc'
    const soloForced = profile === 'codeforces' || profile === 'oi'

    return (
        <div className="space-y-5">
            <fieldset className="rounded-xl border border-hairline bg-surface p-4 shadow-sm">
                <legend className="px-2 text-sm font-semibold text-ink-soft">
                    Algorithmic topics <span className="text-bad">*</span>
                </legend>
                <p className="mb-3 text-xs text-muted">Required. Pick chips or type your own; the model builds the problem around them.</p>
                <TagInput
                    tags={brief.tags}
                    onChange={tags => onChange({ tags })}
                    disabled={disabled}
                    label="Add a topic tag"
                    placeholder="Type a topic and press Enter…"
                />
            </fieldset>

            <fieldset className="rounded-xl border border-hairline bg-surface p-4 shadow-sm">
                <legend className="px-2 text-sm font-semibold text-ink-soft">Difficulty rating</legend>
                <p className="mb-3 text-xs text-muted">Codeforces-style rating. Calibrates algorithmic depth and the expected solution complexity.</p>
                <RatingSlider value={brief.rating} onChange={v => onChange(withRating(brief, v))} disabled={disabled} />
                <div className="mt-4 grid grid-cols-2 gap-3">
                    <label className="text-xs text-muted">
                        Minimum acceptable
                        <input
                            type="number" min={800} max={3500} step={100}
                            value={brief.rating_min ?? brief.rating - 100}
                            disabled={disabled}
                            onChange={e => onChange(withRange(brief, 'min', Number(e.target.value)))}
                            className={field}
                        />
                    </label>
                    <label className="text-xs text-muted">
                        Maximum acceptable
                        <input
                            type="number" min={800} max={3500} step={100}
                            value={brief.rating_max ?? brief.rating + 100}
                            disabled={disabled}
                            onChange={e => onChange(withRange(brief, 'max', Number(e.target.value)))}
                            className={field}
                        />
                    </label>
                </div>
            </fieldset>

            <fieldset className="space-y-3 rounded-xl border border-hairline bg-surface p-4 shadow-sm">
                <legend className="px-2 text-sm font-semibold text-ink-soft">Authoring profile</legend>
                <div className="grid gap-3 sm:grid-cols-2">
                    <label className="text-xs text-muted">
                        Format profile
                        <select value={profile} disabled={disabled} onChange={e => onChange({ format_profile: e.target.value as AIBrief['format_profile'] })} className={field}>
                            <option value="general">General practice</option>
                            <option value="codeforces">Codeforces round</option>
                            <option value="icpc">ICPC onsite</option>
                            <option value="oi">OI / IOI style</option>
                        </select>
                    </label>
                    <label className="text-xs text-muted">
                        Event context
                        <select
                            value={icpc ? 'onsite' : (brief.event_context ?? 'practice')}
                            disabled={disabled || icpc}
                            onChange={e => onChange({ event_context: e.target.value as AIBrief['event_context'] })}
                            className={field}
                        >
                            <option value="practice">Practice</option>
                            <option value="online">Online contest</option>
                            <option value="onsite">Onsite contest</option>
                        </select>
                    </label>
                    <label className="text-xs text-muted">
                        Solver mode
                        <select
                            value={icpc ? 'team' : soloForced ? 'solo' : (brief.solver_mode ?? 'solo')}
                            disabled={disabled || icpc || soloForced}
                            onChange={e => onChange({ solver_mode: e.target.value as AIBrief['solver_mode'] })}
                            className={field}
                        >
                            <option value="solo">Solo</option>
                            <option value="team">Team</option>
                        </select>
                    </label>
                    <label className="text-xs text-muted">
                        Expected solve minutes
                        <input
                            type="number" min={1} max={360}
                            value={brief.expected_solve_minutes ?? 45}
                            disabled={disabled}
                            onChange={e => onChange({ expected_solve_minutes: Number(e.target.value) })}
                            className={field}
                        />
                    </label>
                    <label className="text-xs text-muted">
                        Audience level
                        <select value={brief.audience_level ?? 'intermediate'} disabled={disabled} onChange={e => onChange({ audience_level: e.target.value })} className={field}>
                            <option value="beginner">Beginner</option>
                            <option value="intermediate">Intermediate</option>
                            <option value="advanced">Advanced</option>
                        </select>
                    </label>
                    <label className="text-xs text-muted">
                        Insight / implementation emphasis
                        <select
                            value={brief.insight_emphasis ?? 'balanced'}
                            disabled={disabled}
                            onChange={e => onChange({ insight_emphasis: e.target.value as AIBrief['insight_emphasis'] })}
                            className={field}
                        >
                            <option value="balanced">Balanced</option>
                            <option value="insight">Insight-heavy</option>
                            <option value="implementation">Implementation-heavy</option>
                        </select>
                    </label>
                    <label className="text-xs text-muted">
                        Story tone
                        <input value={brief.story_tone ?? 'neutral'} disabled={disabled} onChange={e => onChange({ story_tone: e.target.value })} className={field} />
                    </label>
                    {(icpc || brief.solver_mode === 'team') && !soloForced && (
                        <label className="text-xs text-muted">
                            Team size
                            <input
                                type="number" min={1} max={8}
                                value={icpc ? 3 : (brief.team_size ?? 3)}
                                disabled={disabled || icpc}
                                onChange={e => onChange({ team_size: Number(e.target.value) })}
                                className={field}
                            />
                        </label>
                    )}
                </div>
                <p className="text-xs text-muted-soft">
                    Target {brief.rating} within {brief.rating_min ?? brief.rating - 100}–{brief.rating_max ?? brief.rating + 100};{' '}
                    {icpc ? 'team of 3 / onsite / icpc' : `${soloForced ? 'solo' : (brief.solver_mode ?? 'solo')} / ${brief.event_context ?? 'practice'} / ${profile}`}.
                </p>
            </fieldset>

            <fieldset className="space-y-3 rounded-xl border border-hairline bg-surface p-4 shadow-sm">
                <legend className="px-2 text-sm font-semibold text-ink-soft">Technique constraints</legend>
                <div className="grid gap-3 sm:grid-cols-3">
                    <label className="text-xs text-muted">
                        Secondary topics
                        <input
                            value={(brief.secondary_topics ?? []).join(', ')}
                            disabled={disabled}
                            onChange={e => onChange({ secondary_topics: e.target.value.split(',').map(v => v.trim()).filter(Boolean) })}
                            placeholder="arrays, math"
                            className={field}
                        />
                    </label>
                    <label className="text-xs text-muted">
                        Required techniques
                        <input
                            value={(brief.required_techniques ?? []).join(', ')}
                            disabled={disabled}
                            onChange={e => onChange({ required_techniques: e.target.value.split(',').map(v => v.trim()).filter(Boolean) })}
                            placeholder="binary search"
                            className={field}
                        />
                    </label>
                    <label className="text-xs text-muted">
                        Forbidden techniques
                        <input
                            value={(brief.forbidden_techniques ?? []).join(', ')}
                            disabled={disabled}
                            onChange={e => onChange({ forbidden_techniques: e.target.value.split(',').map(v => v.trim()).filter(Boolean) })}
                            placeholder="brute force"
                            className={field}
                        />
                    </label>
                </div>
                <label className="block text-xs text-muted">
                    Constraint guidance
                    {/* Prose that lands in the generator prompt: one bound or
                        caution per line has to survive as separate lines. */}
                    <textarea
                        value={brief.constraint_guidance ?? ''}
                        disabled={disabled}
                        onChange={e => onChange({ constraint_guidance: e.target.value })}
                        rows={2}
                        placeholder="e.g. N up to 2e5; avoid constraints that let brute force pass"
                        className={field}
                    />
                </label>
            </fieldset>

            <fieldset className="space-y-3 rounded-xl border border-hairline bg-surface p-4 shadow-sm">
                <legend className="px-2 text-sm font-semibold text-ink-soft">Test cases &amp; scoring</legend>
                <div className="grid gap-3 sm:grid-cols-2">
                    <label className="text-xs text-muted">
                        Test case count (1–100)
                        <input
                            type="number" min={1} max={100}
                            aria-label="Test case count"
                            value={brief.testcase_count}
                            disabled={disabled}
                            onChange={e => onChange({ testcase_count: Math.min(100, Math.max(1, Number(e.target.value))) })}
                            className={field}
                        />
                    </label>
                    <label className="flex items-start gap-2 self-end pb-2 text-xs text-ink-soft">
                        <input
                            type="checkbox"
                            checked={brief.has_subtasks}
                            disabled={disabled || icpc}
                            onChange={e => onChange({ has_subtasks: e.target.checked })}
                            className="mt-0.5 accent-[var(--primary)]"
                        />
                        <span>
                            Subtask scoring (partial credit)
                            {icpc && <span className="block text-muted-soft">Not available for the ICPC profile.</span>}
                        </span>
                    </label>
                </div>
                <p className="text-xs text-muted-soft">
                    The harness materialises the hidden cases from the generator it writes, so this number steers statement examples and curated edge cases.
                </p>
            </fieldset>

            <fieldset className="rounded-xl border border-hairline bg-surface p-4 shadow-sm">
                <legend className="px-2 text-sm font-semibold text-ink-soft">
                    Additional guidance <span className="font-normal text-muted-soft">(optional)</span>
                </legend>
                <p className="mb-2 text-xs text-muted">
                    Free-form steer, e.g. &ldquo;grid DP on an N×M array&rdquo;, &ldquo;the key insight is modular inverses&rdquo;, &ldquo;avoid trivial brute force&rdquo;.
                </p>
                <textarea
                    aria-label="Additional instructions for the model"
                    value={brief.additional_prompt ?? ''}
                    disabled={disabled}
                    rows={3}
                    onChange={e => onChange({ additional_prompt: e.target.value })}
                    placeholder="Describe any scenario, constraint, or insight you want the model to incorporate…"
                    className="w-full resize-none rounded-lg border border-hairline-strong bg-surface px-3 py-2 text-sm text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                />
            </fieldset>
        </div>
    )
}
