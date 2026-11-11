import { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Minus, Plus, Scale, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { useConfirm } from '../../components/ConfirmDialog'
import { useToast } from '../../components/Toast'
import { EmptyState } from '../../components/EmptyState'
import type { ContestAdjustment } from '../../types/api'

/**
 * Manual score corrections.
 *
 * A round sometimes needs a fix no verdict can express: a test case was wrong,
 * the judge crashed on a valid solution, a problem was ambiguous and the setter
 * ruled the points stand. Editing the submission by hand would lie about what
 * the code did, so instead this writes a signed delta over the computed
 * standings — the submissions keep what they earned, and the correction is its
 * own row with a required reason.
 *
 * Three scopes, because judges think in three shapes:
 *   contest    the whole round for one person (e.g. give them back time lost to
 *              a judge outage)
 *   problem    one problem's column for one person (a broken test case)
 *   submission one submission (credit the solution that was wrongly rejected)
 *
 * The panel is deliberately read-mostly: a correction is a notable act, so the
 * history is always visible next to the form rather than hidden in a modal.
 */

interface ParticipantOption {
    user_id: string
    username: string
    team_name?: string
}

interface ProblemOption {
    problem_id: string
    index: string
    title: string
}

export function ContestAdjustmentsPanel({ contestId }: { contestId: string }) {
    const confirm = useConfirm()
    const toast = useToast()

    const [items, setItems] = useState<ContestAdjustment[]>([])
    const [participants, setParticipants] = useState<ParticipantOption[]>([])
    const [problems, setProblems] = useState<ProblemOption[]>([])
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)

    const [userId, setUserId] = useState('')
    const [scope, setScope] = useState<'contest' | 'problem' | 'submission'>('contest')
    const [problemId, setProblemId] = useState('')
    const [submissionId, setSubmissionId] = useState('')
    const [pointDelta, setPointDelta] = useState(0)
    const [solvedDelta, setSolvedDelta] = useState(0)
    const [penaltyDelta, setPenaltyDelta] = useState(0)
    const [reason, setReason] = useState('')

    const load = useCallback(() => {
        Promise.all([
            api.adjustments.list(contestId),
            api.contests.get(contestId),
            // The people on the board are exactly the people a correction can
            // apply to, so the picker is sourced from the standings rather than
            // a user search.
            api.contests.standings(contestId).catch(() => ({ entries: [] })),
        ]).then(([adj, contest, board]) => {
            setItems(adj.data || [])
            setProblems((contest.problems || [])
                .map(p => ({ problem_id: p.problem_id, index: p.index, title: p.title || '' }))
                .sort((a, b) => a.index.localeCompare(b.index)))
            const entries = (board.entries || []) as {
                user_id: string; username: string; team_name?: string
            }[]
            setParticipants(entries.map(e => ({
                user_id: e.user_id,
                username: e.username,
                team_name: e.team_name,
            })))
        }).catch(e => toast.error(errorMessage(e) || 'Failed to load adjustments'))
            .finally(() => setLoading(false))
    }, [contestId, toast])

    useEffect(() => { load() }, [load])

    const selected = useMemo(
        () => participants.find(p => p.user_id === userId),
        [participants, userId],
    )

    const canSubmit = Boolean(
        userId && reason.trim().length >= 3 &&
        (pointDelta !== 0 || solvedDelta !== 0 || penaltyDelta !== 0) &&
        (scope !== 'problem' || problemId) &&
        (scope !== 'submission' || submissionId),
    )

    const submit = async () => {
        if (!canSubmit) return
        setSaving(true)
        try {
            await api.adjustments.create(contestId, {
                user_id: userId,
                scope,
                problem_id: scope === 'problem' ? problemId : undefined,
                submission_id: scope === 'submission' ? submissionId : undefined,
                point_delta: pointDelta,
                solved_delta: solvedDelta,
                penalty_delta: penaltyDelta,
                reason: reason.trim(),
            })
            toast.success('Correction applied to the standings')
            setPointDelta(0); setSolvedDelta(0); setPenaltyDelta(0); setReason('')
            setSubmissionId('')
            load()
        } catch (e) {
            toast.error(errorMessage(e) || 'Failed to apply the correction')
        } finally {
            setSaving(false)
        }
    }

    const remove = async (a: ContestAdjustment) => {
        if (!(await confirm({
            message: `Remove this correction (${describeDelta(a)})? The standings will go back to the computed result.`,
            variant: 'danger',
        }))) return
        try {
            await api.adjustments.remove(contestId, a.id)
            setItems(prev => prev.filter(x => x.id !== a.id))
        } catch (e) {
            toast.error(errorMessage(e) || 'Failed to remove the correction')
        }
    }

    if (loading) {
        return <div className="py-10 text-center text-sm text-muted">Loading corrections…</div>
    }

    return (
        <div className="space-y-6">
            <div className="rounded-lg border border-warn/40 bg-warn/[0.06] px-4 py-3 text-xs text-warn">
                <span className="flex items-start gap-2">
                    <AlertTriangle className="mt-0.5 h-3.5 w-3.5 flex-shrink-0" aria-hidden="true" />
                    <span>
                        A correction is layered over the computed result — the submissions keep the
                        verdicts they earned. It changes the standings immediately and is visible to
                        everyone who can see the scoreboard.
                    </span>
                </span>
            </div>

            {/* ── New correction ─────────────────────────────────────────── */}
            <section className="rounded-xl border border-hairline bg-surface p-4">
                <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-ink">
                    <Scale className="h-4 w-4" aria-hidden="true" /> Add a correction
                </h3>

                {participants.length === 0 ? (
                    <p className="text-sm text-muted">
                        Nobody has submitted to this contest yet, so there is nothing to correct.
                    </p>
                ) : (
                    <div className="space-y-3">
                        <div className="grid gap-3 sm:grid-cols-2">
                            <label className="block">
                                <span className="mb-1 block text-xs font-medium text-muted">Participant</span>
                                <select
                                    aria-label="Participant"
                                    value={userId}
                                    onChange={e => setUserId(e.target.value)}
                                    className="w-full rounded-lg border border-hairline bg-surface px-3 py-2 text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
                                >
                                    <option value="">Choose a participant…</option>
                                    {participants.map(p => (
                                        <option key={p.user_id} value={p.user_id}>
                                            {p.team_name ? `${p.team_name} (${p.username})` : p.username}
                                        </option>
                                    ))}
                                </select>
                            </label>

                            <label className="block">
                                <span className="mb-1 block text-xs font-medium text-muted">Applies to</span>
                                <select
                                    aria-label="Applies to"
                                    value={scope}
                                    onChange={e => setScope(e.target.value as typeof scope)}
                                    className="w-full rounded-lg border border-hairline bg-surface px-3 py-2 text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
                                >
                                    <option value="contest">The whole round</option>
                                    <option value="problem">One problem</option>
                                    <option value="submission">One submission</option>
                                </select>
                            </label>
                        </div>

                        {scope === 'problem' && (
                            <label className="block">
                                <span className="mb-1 block text-xs font-medium text-muted">Problem</span>
                                <select
                                    aria-label="Problem"
                                    value={problemId}
                                    onChange={e => setProblemId(e.target.value)}
                                    className="w-full rounded-lg border border-hairline bg-surface px-3 py-2 text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
                                >
                                    <option value="">Choose a problem…</option>
                                    {problems.map(p => (
                                        <option key={p.problem_id} value={p.problem_id}>
                                            {p.index} — {p.title}
                                        </option>
                                    ))}
                                </select>
                            </label>
                        )}

                        {scope === 'submission' && (
                            <label className="block">
                                <span className="mb-1 block text-xs font-medium text-muted">
                                    Submission ID
                                </span>
                                <input
                                    aria-label="Submission ID"
                                    value={submissionId}
                                    onChange={e => setSubmissionId(e.target.value.trim())}
                                    placeholder="Paste the submission id the result belongs to"
                                    className="w-full rounded-lg border border-hairline bg-surface px-3 py-2 font-mono text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
                                />
                                <span className="mt-1 block text-[11px] text-muted">
                                    Must belong to this contest and to the participant you chose.
                                </span>
                            </label>
                        )}

                        <div className="grid gap-3 sm:grid-cols-3">
                            <DeltaField
                                label="Points"
                                value={pointDelta}
                                onChange={setPointDelta}
                                hint="Added to the total score"
                            />
                            <DeltaField
                                label="Problems solved"
                                value={solvedDelta}
                                onChange={setSolvedDelta}
                                hint="Changes the solved count"
                            />
                            <DeltaField
                                label="Penalty"
                                value={penaltyDelta}
                                onChange={setPenaltyDelta}
                                min={0}
                                hint="Added to the penalty"
                            />
                        </div>

                        <label className="block">
                            <span className="mb-1 block text-xs font-medium text-muted">
                                Reason <span className="text-bad">*</span>
                            </span>
                            <textarea
                                aria-label="Reason"
                                value={reason}
                                onChange={e => setReason(e.target.value)}
                                rows={2}
                                placeholder="e.g. test case 4 was wrong, problem statement was ambiguous…"
                                className="w-full rounded-lg border border-hairline p-2 text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
                            />
                            <span className="mt-1 block text-[11px] text-muted">
                                Required. This is what someone reads months from now to understand
                                why the standings differ from the judge's output.
                            </span>
                        </label>

                        <div className="flex items-center justify-between gap-3">
                            <p className="text-[11px] text-muted">
                                {selected
                                    ? `Applies to ${selected.team_name || selected.username}`
                                    : 'Choose a participant to apply the correction.'}
                            </p>
                            <button
                                type="button"
                                onClick={submit}
                                disabled={!canSubmit || saving}
                                className="rounded-lg bg-brand px-4 py-2 text-sm font-semibold text-on-brand transition-colors hover:bg-brand/90 disabled:cursor-not-allowed disabled:opacity-50"
                            >
                                {saving ? 'Applying…' : 'Apply correction'}
                            </button>
                        </div>
                    </div>
                )}
            </section>

            {/* ── History ────────────────────────────────────────────────── */}
            <section>
                <h3 className="mb-2 text-sm font-semibold text-ink">
                    Correction history
                    {items.length > 0 && <span className="ml-2 text-xs font-normal text-muted">{items.length}</span>}
                </h3>

                {items.length === 0 ? (
                    <EmptyState
                        icon={<Scale className="h-5 w-5 text-muted-soft" />}
                        text="No corrections yet"
                        description="Manual adjustments to the standings will be recorded here with their reason."
                    />
                ) : (
                    <ul className="space-y-2">
                        {items.map(a => (
                            <li key={a.id} className="rounded-lg border border-hairline bg-surface p-3">
                                <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                                    <span className="font-medium text-ink-soft">
                                        {a.team_name || a.username}
                                    </span>
                                    <span className="rounded bg-surface-3 px-1.5 py-0.5 text-[11px] font-medium">
                                        {scopeLabel(a)}
                                    </span>
                                    <span className="tnum font-semibold text-ink-soft">{describeDelta(a)}</span>
                                    <span className="ml-auto flex items-center gap-2">
                                        <span>{new Date(a.created_at).toLocaleString()}</span>
                                        <button
                                            type="button"
                                            onClick={() => remove(a)}
                                            aria-label={`Remove correction for ${a.team_name || a.username}`}
                                            className="rounded p-1 text-muted transition-colors hover:bg-bad/10 hover:text-bad"
                                        >
                                            <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                                        </button>
                                    </span>
                                </div>
                                <p className="mt-1.5 whitespace-pre-wrap break-words text-sm text-ink-soft">
                                    {a.reason}
                                </p>
                            </li>
                        ))}
                    </ul>
                )}
            </section>
        </div>
    )
}

/** A signed numeric delta with +/- affordances, because typing a minus is fiddly. */
function DeltaField({ label, value, onChange, hint, min }: {
    label: string
    value: number
    onChange: (v: number) => void
    hint: string
    min?: number
}) {
    return (
        <label className="block">
            <span className="mb-1 block text-xs font-medium text-muted">{label}</span>
            <div className="flex items-stretch gap-1">
                <button
                    type="button"
                    aria-label={`Decrease ${label}`}
                    onClick={() => onChange(value - 1)}
                    disabled={min !== undefined && value <= min}
                    className="rounded-l-lg border border-hairline bg-surface-2 px-2 text-muted transition-colors hover:bg-surface-3 disabled:opacity-40"
                >
                    <Minus className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
                <input
                    aria-label={label}
                    type="number"
                    value={value}
                    onChange={e => onChange(Number(e.target.value) || 0)}
                    className="tnum w-full min-w-0 border-y border-hairline bg-surface px-2 py-2 text-center text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
                />
                <button
                    type="button"
                    aria-label={`Increase ${label}`}
                    onClick={() => onChange(value + 1)}
                    className="rounded-r-lg border border-hairline bg-surface-2 px-2 text-muted transition-colors hover:bg-surface-3"
                >
                    <Plus className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
            </div>
            <span className="mt-1 block text-[11px] text-muted">{hint}</span>
        </label>
    )
}

function scopeLabel(a: ContestAdjustment): string {
    if (a.scope === 'contest') return 'Whole round'
    if (a.scope === 'problem') return a.problem_index ? `Problem ${a.problem_index}` : 'Problem'
    return a.submission_id ? `Submission ${a.submission_id.slice(0, 8)}` : 'Submission'
}

/** "+100 pts, +1 solved, −20 penalty", naming only the deltas that were applied. */
function describeDelta(a: ContestAdjustment): string {
    const parts: string[] = []
    if (a.point_delta) parts.push(`${a.point_delta > 0 ? '+' : ''}${a.point_delta} pts`)
    if (a.solved_delta) parts.push(`${a.solved_delta > 0 ? '+' : ''}${a.solved_delta} solved`)
    if (a.penalty_delta) parts.push(`+${a.penalty_delta} penalty`)
    return parts.join(', ') || 'no change'
}