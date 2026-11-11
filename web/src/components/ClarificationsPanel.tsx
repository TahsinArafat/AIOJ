import { useMemo, useState } from 'react'
import { Check, CheckCheck, ChevronDown, Loader2, Megaphone, MessageSquare } from 'lucide-react'
import type { Clarification } from '../types/api'

/**
 * A round's clarifications.
 *
 * Two jobs in one panel, because the two surfaces that needed this are the same
 * list at different levels of detail:
 *
 *   - a competitor reads questions and official answers, and can ask their own
 *     against a specific problem;
 *   - a jury works a queue: filter to what is unanswered, answer several at once
 *     with the same ruling, and choose whether an answer is public (it joins the
 *     list every competitor sees) or private (only the asker sees it).
 *
 * A question is bound to a problem when it is asked, which is what makes
 * "unanswered, about problem C" possible. An answer marked public is the
 * round's ruling; the list says so on the row rather than leaving a reader to
 * infer it from which names they can see.
 */

export interface ClarificationProblem {
    problem_id: string
    index: string
    title: string
}

interface Props {
    items: Clarification[]
    loading: boolean
    problems: ClarificationProblem[]
    /** Staff: may answer, and sees every question rather than their own plus public. */
    isJudge: boolean
    /** Ids that arrived since this surface was last opened; rendered as a badge. */
    newIds?: Set<string>
    /** Pre-bound problem, when opened from a problem page. */
    lockedProblemId?: string
    onAnswer?: (id: string, answer: string, isPublic: boolean) => Promise<void>
    onAnswerMany?: (ids: string[], answer: string, isPublic: boolean) => Promise<void>
}

// An answer the jury marks public joins the list every contestant reads; that is
// the default, because a clarification is usually a ruling the round needs.
const ANSWER_PUBLIC = true

export function ClarificationsPanel({
    items, loading, problems, isJudge, newIds, lockedProblemId,
    onAnswer, onAnswerMany,
}: Props) {
    const [problemFilter, setProblemFilter] = useState<string>(lockedProblemId ?? '')
    const [answeredFilter, setAnsweredFilter] = useState<'all' | 'open' | 'answered'>('all')
    // Which rows the jury has ticked for a bulk answer.
    const [selected, setSelected] = useState<Set<string>>(new Set())

    const labelFor = (c: Clarification) => {
        if (!c.problem_id) return null
        const p = problems.find(x => x.problem_id === c.problem_id)
        return p ? `${p.index} — ${p.title}` : c.problem_idx ?? null
    }

    const visible = useMemo(() => {
        // The problem select and the status buttons are independent, so a row
        // has to pass both.
        let rows = items
        if (problemFilter) {
            rows = rows.filter(c => c.problem_id === problemFilter)
        }
        // Filtering is done here as well as on the server so the counts on the
        // filter buttons describe what is actually on screen, including the rows
        // the server already narrowed.
        if (answeredFilter === 'open') return rows.filter(c => !c.answer)
        if (answeredFilter === 'answered') return rows.filter(c => Boolean(c.answer))
        return rows
    }, [items, problemFilter, answeredFilter])

    const openCount = items.filter(c => !c.answer).length
    const allSelected = visible.length > 0 && visible.every(c => selected.has(c.id))

    const toggle = (id: string) => setSelected(prev => {
        const next = new Set(prev)
        if (next.has(id)) next.delete(id); else next.add(id)
        return next
    })

    const [bulkAnswer, setBulkAnswer] = useState('')
    const [bulkBusy, setBulkBusy] = useState(false)

    const submitBulk = async () => {
        if (selected.size === 0 || !bulkAnswer.trim() || !onAnswerMany) return
        setBulkBusy(true)
        try {
            await onAnswerMany([...selected], bulkAnswer.trim(), ANSWER_PUBLIC)
            setSelected(new Set())
            setBulkAnswer('')
        } finally {
            setBulkBusy(false)
        }
    }

    return (
        <div className="space-y-4">
            {/* Filters. The jury's default need is "what is still unanswered";
                the competitor's is "what has been answered about this round". */}
            <div className="flex flex-wrap items-center gap-2">
                <div className="inline-flex rounded-lg border border-hairline overflow-hidden">
                    {([
                        ['all', `All (${items.length})`],
                        ['open', `Unanswered (${openCount})`],
                        ['answered', 'Answered'],
                    ] as const).map(([key, label]) => (
                        <button
                            key={key}
                            type="button"
                            onClick={() => setAnsweredFilter(key)}
                            aria-pressed={answeredFilter === key}
                            className={`px-3 py-1.5 text-xs font-semibold transition-colors ${answeredFilter === key
                                ? 'bg-brand text-on-brand'
                                : 'bg-surface text-muted hover:bg-surface-2'}`}
                        >
                            {label}
                        </button>
                    ))}
                </div>

                <div className="relative">
                    <select
                        aria-label="Filter by problem"
                        value={problemFilter}
                        onChange={e => setProblemFilter(e.target.value)}
                        disabled={Boolean(lockedProblemId)}
                        className="appearance-none rounded-lg border border-hairline bg-surface px-3 py-1.5 pr-8 text-xs font-medium text-ink focus:outline-none focus-visible:outline-[var(--ring)] disabled:opacity-60"
                    >
                        <option value="">All problems</option>
                        {problems.map(p => (
                            <option key={p.problem_id} value={p.problem_id}>{p.index} — {p.title}</option>
                        ))}
                    </select>
                    <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted" aria-hidden="true" />
                </div>
            </div>

            {loading && items.length === 0 ? (
                <div className="flex items-center gap-2 py-8 text-sm text-muted">
                    <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> Loading clarifications…
                </div>
            ) : visible.length === 0 ? (
                <div className="rounded-xl border border-hairline bg-surface px-4 py-10 text-center">
                    <MessageSquare className="mx-auto mb-2 h-6 w-6 text-muted-soft" aria-hidden="true" />
                    <p className="text-sm text-muted">
                        {items.length === 0
                            ? 'No clarifications yet.'
                            : 'No clarifications match this filter.'}
                    </p>
                </div>
            ) : (
                <ul className="space-y-3">
                    {isJudge && (
                        <li className="flex items-center gap-2 px-1 text-xs text-muted">
                            <label className="flex cursor-pointer items-center gap-1.5">
                                <input
                                    type="checkbox"
                                    aria-label="Select all visible"
                                    checked={allSelected}
                                    onChange={() => setSelected(allSelected
                                        ? new Set()
                                        : new Set(visible.map(c => c.id)))}
                                    className="h-3.5 w-3.5 rounded border-hairline text-brand focus:ring-brand"
                                />
                                Select all
                            </label>
                            <span>{selected.size} selected</span>
                        </li>
                    )}
                    {visible.map(c => {
                        const label = labelFor(c)
                        const mine = newIds?.has(c.id)
                        return (
                            <li
                                key={c.id}
                                className={`rounded-xl border p-4 ${c.answer ? 'border-hairline bg-surface' : 'border-warn/40 bg-warn/[0.04]'}`}
                            >
                                <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                                    {isJudge && (
                                        <input
                                            type="checkbox"
                                            aria-label={`Select clarification about: ${c.question}`}
                                            checked={selected.has(c.id)}
                                            onChange={() => toggle(c.id)}
                                            className="h-3.5 w-3.5 rounded border-hairline text-brand focus:ring-brand"
                                        />
                                    )}
                                    <span className="font-medium text-ink-soft">{c.username}</span>
                                    {label && (
                                        <span className="rounded bg-surface-3 px-1.5 py-0.5 font-mono text-[11px] text-muted">
                                            {label}
                                        </span>
                                    )}
                                    <span>{new Date(c.created_at).toLocaleString()}</span>
                                    {mine && (
                                        <span className="rounded-full bg-brand/15 px-1.5 py-0.5 text-[10px] font-semibold text-brand">new</span>
                                    )}
                                    {!c.answer && (
                                        <span className="rounded-full bg-warn/20 px-1.5 py-0.5 text-[10px] font-semibold text-warn">unanswered</span>
                                    )}
                                </div>

                                <p className="mt-2 whitespace-pre-wrap break-words text-sm text-ink">{c.question}</p>

                                {c.answer ? (
                                    <div className={`mt-3 flex gap-2 rounded-lg border-l-2 p-3 text-sm ${c.is_public ? 'border-emerald-400 bg-emerald-50/60 dark:bg-emerald-500/5' : 'border-brand/40 bg-brand/[0.04]'}`}>
                                        {c.is_public
                                            ? <Megaphone className="mt-0.5 h-4 w-4 flex-shrink-0 text-emerald-600 dark:text-emerald-400" aria-hidden="true" />
                                            : <MessageSquare className="mt-0.5 h-4 w-4 flex-shrink-0 text-brand" aria-hidden="true" />}
                                        <div className="min-w-0">
                                            <p className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-muted">
                                                {c.is_public ? 'Answer — shared with all contestants' : 'Answer — private'}
                                            </p>
                                            <p className="whitespace-pre-wrap break-words text-ink-soft">{c.answer}</p>
                                        </div>
                                    </div>
                                ) : onAnswer ? (
                                    <AnswerRow busy={false} onSubmit={(text, isPublic) => onAnswer(c.id, text, isPublic)} />
                                ) : null}
                            </li>
                        )
                    })}
                </ul>
            )}

            {isJudge && selected.size > 0 && onAnswerMany && (
                <div className="sticky bottom-0 rounded-xl border border-brand/40 bg-surface p-3 shadow-lg">
                    <p className="mb-2 text-xs font-semibold text-ink">
                        Answer {selected.size} question{selected.size === 1 ? '' : 's'} the same way
                    </p>
                    <textarea
                        value={bulkAnswer}
                        onChange={e => setBulkAnswer(e.target.value)}
                        rows={2}
                        aria-label="Answer for the selected clarifications"
                        placeholder="Type the answer that applies to all of them…"
                        className="w-full rounded-lg border border-hairline p-2 text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
                    />
                    <div className="mt-2 flex justify-end gap-2">
                        <button
                            type="button"
                            onClick={submitBulk}
                            disabled={bulkBusy || !bulkAnswer.trim()}
                            className="inline-flex items-center gap-1.5 rounded-lg bg-brand px-3 py-1.5 text-xs font-semibold text-on-brand transition-colors hover:bg-brand/90 disabled:opacity-50"
                        >
                            <CheckCheck className="h-3.5 w-3.5" aria-hidden="true" />
                            {bulkBusy ? 'Answering…' : 'Answer selected'}
                        </button>
                        <button
                            type="button"
                            onClick={() => { setSelected(new Set()); setBulkAnswer('') }}
                            className="rounded-lg bg-surface-3 px-3 py-1.5 text-xs font-semibold text-muted transition-colors hover:bg-surface-3/70"
                        >
                            Clear
                        </button>
                    </div>
                </div>
            )}
        </div>
    )
}

/** An inline answer editor: the reply is typed on the row it belongs to. */
function AnswerRow({ busy, onSubmit }: {
    busy: boolean
    onSubmit: (answer: string, isPublic: boolean) => void
}) {
    const [text, setText] = useState('')
    const [isPublic, setIsPublic] = useState(ANSWER_PUBLIC)

    return (
        <form
            className="mt-3 space-y-2"
            onSubmit={e => { e.preventDefault(); if (text.trim()) onSubmit(text.trim(), isPublic) }}
        >
            <textarea
                value={text}
                onChange={e => setText(e.target.value)}
                rows={2}
                aria-label="Your answer"
                placeholder="Type the answer…"
                className="w-full rounded-lg border border-hairline p-2 text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
            />
            <div className="flex items-center justify-between gap-2">
                <label className="flex cursor-pointer items-center gap-1.5 text-xs text-muted">
                    <input
                        type="checkbox"
                        checked={isPublic}
                        onChange={e => setIsPublic(e.target.checked)}
                        className="h-3.5 w-3.5 rounded border-hairline text-brand focus:ring-brand"
                    />
                    Visible to everyone
                </label>
                <button
                    type="submit"
                    disabled={busy || !text.trim()}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-brand px-3 py-1.5 text-xs font-semibold text-on-brand transition-colors hover:bg-brand/90 disabled:opacity-50"
                >
                    <Check className="h-3.5 w-3.5" aria-hidden="true" /> Answer
                </button>
            </div>
        </form>
    )
}

/**
 * The "ask a question" form.
 *
 * A competitor asks about a specific problem whenever the question is about one,
 * which is nearly always: that is what makes the answer attributable and the
 * queue filterable. A general question (no problem) stays available for rulings
 * that are about the round.
 */
export function AskClarificationForm({
    problems, lockedProblemId, onSubmit,
}: {
    problems: ClarificationProblem[]
    lockedProblemId?: string
    onSubmit: (problemId: string | undefined, question: string) => Promise<void>
}) {
    const [problemId, setProblemId] = useState(lockedProblemId ?? '')
    const [question, setQuestion] = useState('')
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const submit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!question.trim()) return
        setBusy(true)
        setError(null)
        try {
            await onSubmit(problemId || undefined, question.trim())
            setQuestion('')
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Could not send the question')
        } finally {
            setBusy(false)
        }
    }

    return (
        <form onSubmit={submit} className="space-y-2 rounded-xl border border-brand/30 bg-surface p-4">
            <div className="relative">
                <select
                    aria-label="Which problem is this about"
                    value={problemId}
                    onChange={e => setProblemId(e.target.value)}
                    disabled={Boolean(lockedProblemId)}
                    className="w-full appearance-none rounded-lg border border-hairline bg-surface px-3 py-2 pr-8 text-sm text-ink focus:outline-none focus-visible:outline-[var(--ring)] disabled:opacity-60"
                >
                    <option value="">General question (not about one problem)</option>
                    {problems.map(p => (
                        <option key={p.problem_id} value={p.problem_id}>{p.index} — {p.title}</option>
                    ))}
                </select>
                <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden="true" />
            </div>
            <textarea
                value={question}
                onChange={e => setQuestion(e.target.value)}
                rows={3}
                aria-label="Your question"
                placeholder="Ask the organizers a question…"
                className="w-full rounded-lg border border-hairline p-2 text-sm focus:outline-none focus-visible:outline-[var(--ring)]"
            />
            {error && <p className="text-xs text-bad">{error}</p>}
            <div className="flex items-center justify-between gap-2">
                <p className="text-[11px] text-muted">
                    Only public questions and answers are shown to other contestants.
                </p>
                <button
                    type="submit"
                    disabled={busy || !question.trim()}
                    className="flex-shrink-0 rounded-lg bg-brand px-3 py-1.5 text-xs font-semibold text-on-brand transition-colors hover:bg-brand/90 disabled:opacity-50"
                >
                    {busy ? 'Sending…' : 'Ask'}
                </button>
            </div>
        </form>
    )
}