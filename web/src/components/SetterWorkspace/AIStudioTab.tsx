import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { AIBrief, AIGenerationJob, AIModel } from '../../types/api'
import { AIBriefForm } from '../AI/AIBriefForm'
import { DEFAULT_BRIEF } from '../../lib/aiBrief'
import { offeredForExisting, pruneBrief } from '../../lib/aiArtifacts'
import { useArtifactScope, useArtifacts } from '../../hooks/useArtifacts'
import { isJobActive, statusLabel } from '../../lib/aiJobs'
import AIJobList from '../AI/AIJobList'
import AIJobDetail from '../AI/AIJobDetail'

type Props = {
    /**
     * Existing problem to improve. Omit to author a brand-new problem: the job is
     * queued without a problem_id and ApplyService creates the problem from the
     * applied statement, exactly as it does for an existing one.
     */
    problemId?: string
    problemUpdatedAt?: string
    onApplied?: () => void
    /** Called with the new problem's slug after a new problem is created. */
    onCreated?: (slug: string) => void
    /**
     * Whether the job history is on screen. Collapsed by default: the Studio's
     * question is "what am I generating", and a table of the last fifty jobs is an
     * answer to a different one. This is what replaced the separate /setter/ai/jobs
     * page.
     */
    showHistory?: boolean
    /** Admin mode passes through to the history table: every setter's jobs. */
    isAdmin?: boolean
}

export default function AIStudioTab({
    problemId,
    problemUpdatedAt,
    onApplied,
    onCreated,
    showHistory = false,
    isAdmin = false,
}: Props) {
    const isNew = !problemId
    const [connections, setConnections] = useState<AIModel[]>([])
    const [modelId, setModelId] = useState<string>('')
    // The artifact scopes come from the server (GET /api/ai/artifacts), which is
    // the same table that validates a brief and builds the prompt. The picker,
    // the brief form's controls and the summary below all read from it, so the
    // client cannot offer something the server would ignore or reject.
    const artifacts = useArtifacts()
    const specs = artifacts.data?.data ?? []
    const choosable = offeredForExisting(specs)
    // A new problem must be generated as a whole package. For an existing problem
    // the default is the statement, the narrowest useful scope.
    const [artifact, setArtifact] = useState<string>(isNew ? 'full_problem' : 'statement')
    // The scope questions the form asks. Undefined until the catalog lands, which
    // the scope answers as "accepts everything" — so a slow fetch shows the full
    // form briefly rather than an empty one, and the server is still the judge.
    const scope = useArtifactScope(specs.find(s => s.id === artifact))
    const [brief, setBrief] = useState<AIBrief>({ ...DEFAULT_BRIEF })
    const [job, setJob] = useState<AIGenerationJob | null>(null)
    const [error, setError] = useState('')
    const [appliedSlug, setAppliedSlug] = useState<string>('')
    const [historyOpen, setHistoryOpen] = useState(false)
    const [historyJobId, setHistoryJobId] = useState<string | null>(null)

    useEffect(() => {
        api.ai.connections()
            .then(r => {
                const list = r.data || []
                setConnections(list)
                if (list.length > 0) setModelId(id => id || list[0].id)
            })
            .catch(e => setError(String(e)))
    }, [])

    // Resume a job that is genuinely still in flight, or a finished proposal
    // nobody has looked at yet. Deliberately NOT every job that is not
    // discarded or cancelled: that test matched `applied` too, so once a job had
    // been applied the form below could never render again and the tab offered no
    // way to start another run — the "only usable once" report. A finished job is
    // now reached through Job history, and `New run` is always available.
    useEffect(() => {
        if (!problemId) return
        queueMicrotask(() => {
            api.ai.problemJobs(problemId)
                .then(r => {
                    const latest = (r.data || []).find(j => isJobActive(j) || j.status === 'ready')
                    if (latest) {
                        setJob(latest)
                        // Restore artifact picker to match the resumed job
                        setArtifact(latest.artifact_type)
                    }
                })
                .catch(() => {})
        })
    }, [problemId])

    // The server runs one generation per setter. A job started from another
    // problem never shows up in this tab — the per-problem lookup above cannot
    // see it — so it is fetched explicitly. Without this the click looked safe
    // and failed with a limit error that named nothing the setter could find.
    const [activeJobs, setActiveJobs] = useState<AIGenerationJob[]>([])

    const refreshActive = () => {
        api.ai.listJobs(0, 20)
            .then(r => setActiveJobs((r.data || []).filter(isJobActive)))
            .catch(() => {})
    }

    useEffect(() => { refreshActive() }, [])

    const updateBrief = (patch: Partial<AIBrief>) => setBrief(b => ({ ...b, ...patch }))

    const start = async () => {
        setError('')
        setAppliedSlug('')
        try {
            const newJob = await api.ai.createJob({
                problem_id: problemId,
                ai_model_id: modelId || undefined,
                artifact_type: isNew ? 'full_problem' : artifact,
                // Only the fields this scope uses. The form stops editing the
                // others, but the brief carries whatever an earlier scope
                // collected — and the job record renders the brief it was given,
                // so sending a stale rating with an editorial would record an
                // instruction the setter never made.
                brief: pruneBrief(brief, scope),
            })
            setJob(newJob)
            refreshActive()
        } catch (e) {
            setError(e instanceof Error ? e.message : String(e))
        }
    }

    // Cancelling, applying, retrying and discarding the job in view are all
    // AIJobDetail's job now — the Studio used to keep a second copy of each,
    // which is how a cancel could be fired twice and an apply could skip the
    // re-apply confirmation. The Studio only starts runs and tracks the active
    // set for the Generate gate below.

    // Gate the button on the same rules the server enforces, so the click cannot
    // walk into a rejection. The form stays editable; only starting a run is
    // blocked. "Topics required" is the scope's own answer, so an editorial —
    // which takes no topics — is not blocked for lacking them.
    const blocking = activeJobs[0] ?? null
    const runningCount = activeJobs.filter(j => j.status === 'running').length
    const queuedCount = activeJobs.length - runningCount
    const needsTopics = scope.requires('topics') && brief.tags.length === 0
    const canStart = connections.length > 0 && !needsTopics && !blocking
    const blockedReason = blocking
        ? `A generation is already ${blocking.status} (${statusLabel(blocking.stage)}). Cancel it from the job list below to start another.`
        : needsTopics
            ? 'Pick at least one topic.'
            : 'Connect an AI model first.'
    const artifactDef = specs.find(a => a.id === artifact)
    const artifactLabel = artifactDef?.label ?? statusLabel(artifact)
    // A resumed or just-created job. job.status==='ready' renders the review panel;
    // every other state is the progress card.
    const jobActive = job ? isJobActive(job) : false

    return (
        <div className="space-y-5">
            <div>
                <h2 className="text-lg font-semibold">
                    {isNew ? 'Create a problem with AI' : 'AI Studio'}
                </h2>
                <p className="text-sm text-muted">
                    {isNew
                        ? 'Describe the problem you want. The harness drafts it, generates test data, and validates the reference solution before you publish anything.'
                        : 'Generate a reviewable proposal for the selected scope. Nothing is overwritten until you apply selected sections.'}
                </p>
            </div>

            {error && <div className="rounded border border-bad/40 bg-bad/10 p-3 text-sm text-bad">{error}</div>}

            {appliedSlug && (
                <div className="rounded border border-ok/40 bg-ok/10 p-3 text-sm text-ok">
                    Problem created as a draft. Open it at <span className="font-mono">/setter/{appliedSlug}</span> to review and publish.
                </div>
            )}

            {!job && (
                <div className="space-y-4">
                    {/* Artifact scope picker — only shown for existing problems.
                        The options are what the server offers for a problem that
                        already exists, so the whole-package scope (which authors
                        a new one) is absent without a second filter here. */}
                    {!isNew && (
                        <fieldset className="rounded-xl border border-hairline bg-surface p-4">
                            <legend className="px-2 text-sm font-semibold text-ink-soft">What do you want to generate?</legend>
                            {artifacts.error ? (
                                <p className="mt-2 text-sm text-bad">
                                    Could not load the generation scopes: {artifacts.error}
                                </p>
                            ) : artifacts.loading ? (
                                <p className="mt-2 text-sm text-muted">Loading scopes…</p>
                            ) : (
                                <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
                                    {choosable.map(a => (
                                        <label
                                            key={a.id}
                                            className={`flex cursor-pointer flex-col rounded-lg border p-3 transition-colors ${
                                                artifact === a.id
                                                    ? 'border-brand bg-brand-soft'
                                                    : 'border-hairline-strong hover:border-muted-soft'
                                            }`}
                                        >
                                            <input
                                                type="radio"
                                                name="artifact"
                                                value={a.id}
                                                checked={artifact === a.id}
                                                onChange={() => setArtifact(a.id)}
                                                className="sr-only"
                                            />
                                            <span className={`text-sm font-medium ${artifact === a.id ? 'text-brand' : 'text-ink'}`}>
                                                {a.label}
                                            </span>
                                            <span className="mt-0.5 text-xs text-muted">{a.description}</span>
                                        </label>
                                    ))}
                                </div>
                            )}
                        </fieldset>
                    )}

                    {/* What the run builds on. Every scope except the full package
                        reads the existing problem — the harness passes it to the
                        model as authoritative context — so the setter should be
                        able to see that, and see that it is the version on screen
                        rather than a stale one. The apply step refuses a write if
                        the problem changed in between, which is why the timestamp
                        being shown is not just a reassurance. */}
                    {!isNew && (
                        <p className="text-xs text-muted">
                            <span className="font-medium text-ink-soft">Based on</span>{' '}
                            {problemUpdatedAt ? (
                                <>this problem as of {new Date(problemUpdatedAt).toLocaleString()}</>
                            ) : (
                                <>the problem as it is saved now</>
                            )}
                            {'. '}
                            {scope.accepts('topics')
                                ? 'Its statement, limits and samples are passed to the model, and your brief below builds a new one around them.'
                                : 'Its statement, constraints, limits and samples are passed to the model as authoritative — this run rewrites only the part you selected, reusing them verbatim.'}
                        </p>
                    )}

                    <label className="block text-sm">
                        Connection
                        <select
                            value={modelId}
                            onChange={e => setModelId(e.target.value)}
                            className="mt-1 min-h-11 w-full rounded border border-hairline-strong bg-surface px-3 py-2"
                        >
                            {connections.map(c => (
                                <option key={c.id} value={c.id}>{c.name} · {c.model_name}</option>
                            ))}
                        </select>
                        {connections.length === 0 && (
                            <span className="mt-1 block text-xs text-warn">
                                No enabled AI model. Add one in Admin › AI Models first.
                            </span>
                        )}
                    </label>

                    {/* The form asks only what this scope can use: an editorial
                        needs no topic, rating or test count, and the server would
                        ignore them if it sent them. */}
                    <AIBriefForm brief={brief} onChange={updateBrief} scope={scope} />

                    <div className="flex flex-wrap items-center gap-3">
                        <button
                            type="button"
                            disabled={!canStart}
                            onClick={start}
                            className="min-h-11 cursor-pointer rounded bg-brand px-5 text-on-brand disabled:opacity-50"
                        >
                            {isNew ? 'Generate problem' : `Generate ${artifactLabel}`}
                        </button>
                        {!canStart && (
                            <span className="text-xs text-muted">{blockedReason}</span>
                        )}
                    </div>
                    {activeJobs.length > 0 && (
                        <p className="text-xs text-muted">
                            {runningCount} running, {queuedCount} queued · limit 1 running, 3 queued
                        </p>
                    )}
                </div>
            )}

            {job && (
                <div className="space-y-4">
                    {/* Shown whenever this job is not still working. The form
                        above is hidden while a job is in view, so without this
                        an `applied` job — which has no Discard and no Start
                        again — left the setter with no way to run anything
                        else on this problem. Leaving the view never discards
                        the job; it stays in Job history. */}
                    {!jobActive && (
                        <button
                            type="button"
                            onClick={() => setJob(null)}
                            className="cursor-pointer text-sm font-medium text-brand hover:underline"
                        >
                            Start a new run
                        </button>
                    )}
                    {/* The live job view — progress, cancel, validation, apply,
                        retry, discard — is AIJobDetail, the same component Job
                        history uses. The Studio previously kept a second copy of
                        all of it (its own poll loop, its own cancel/apply/retry
                        handlers, a raw progress card); that was the double-cancel
                        bug and the "1000ms vs 1000 ms" drift. One component, one
                        timer, one set of actions. */}
                    <AIJobDetail
                        key={job.id}
                        jobId={job.id}
                        isNew={isNew}
                        expectedUpdatedAt={problemUpdatedAt}
                        onCreated={(slug) => { setAppliedSlug(slug); onCreated?.(slug) }}
                        onApplied={() => { refreshActive(); onApplied?.() }}
                        onDiscarded={() => { setJob(null); refreshActive() }}
                        onCancelled={() => { if (job) api.ai.getJob(job.id).then(setJob).catch(() => {}); refreshActive() }}
                        onRetried={(newId) => { setHistoryJobId(newId); refreshActive() }}
                    />
                </div>
            )}

            {/* Job history. Collapsed behind a disclosure, and rendered by the same
                component the admin tab uses — this is what replaced the standalone
                /setter/ai/jobs page, whose table, filters, polling and cancel/retry
                logic were a second copy of exactly this. */}
            {showHistory && (
                <div className="border-t border-hairline pt-4">
                    <button
                        type="button"
                        onClick={() => setHistoryOpen(v => !v)}
                        className="cursor-pointer text-sm font-medium text-ink-soft hover:text-ink"
                    >
                        {historyOpen ? '▾' : '▸'} Job history
                    </button>
                    {historyOpen && (
                        <div className="mt-3">
                            <AIJobList
                                isAdmin={isAdmin}
                                selectedId={historyJobId}
                                onSelect={setHistoryJobId}
                                renderDetail={(id) => (
                                    <div className="rounded-xl border border-hairline bg-surface p-4">
                                        <AIJobDetail
                                            jobId={id}
                                            isNew={isNew}
                                            onCreated={onCreated}
                                            onApplied={() => { refreshActive(); onApplied?.() }}
                                            onDiscarded={() => { setHistoryJobId(null); refreshActive(); setJob(null) }}
                                            onRetried={(newId) => { setHistoryJobId(newId); refreshActive() }}
                                        />
                                    </div>
                                )}
                            />
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
