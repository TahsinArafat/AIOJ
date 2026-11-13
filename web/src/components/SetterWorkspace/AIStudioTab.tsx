import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { AIBrief, AIGenerationJob, AIModel } from '../../types/api'
import { AIBriefForm } from '../AI/AIBriefForm'
import { DEFAULT_BRIEF } from '../../lib/aiBrief'
import { isJobActive, statusLabel } from '../../lib/aiJobs'
import AIJobStatusBadge from '../AI/AIJobStatusBadge'
import AIValidationList from '../AI/AIValidationList'
import AIProposalReview from '../AI/AIProposalReview'
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

// Artifact types with labels and what sections they produce.
const ARTIFACTS = [
    { id: 'full_problem',  label: 'Full package',         desc: 'Statement, tests, generator & reference solution', sections: ['statement', 'solution', 'tests'] },
    { id: 'statement',     label: 'Statement & samples',  desc: 'Rewrites the problem statement only', sections: ['statement'] },
    { id: 'tests',         label: 'Tests & generator',    desc: 'Generates a new test suite and generator', sections: ['tests'] },
    { id: 'solution',      label: 'Reference solution',   desc: 'Writes or replaces the reference solution', sections: ['solution'] },
    { id: 'editorial',     label: 'Editorial',            desc: 'Authors an editorial for this problem', sections: ['editorial'] },
] as const

type ArtifactId = typeof ARTIFACTS[number]['id']

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
    // A new problem must be generated as a whole package.
    // For existing problems default to nothing selected so the user has to actively choose the scope.
    const [artifact, setArtifact] = useState<ArtifactId>(isNew ? 'full_problem' : 'statement')
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

    // Resume the newest unfinished/reviewable job for this problem.
    useEffect(() => {
        if (!problemId) return
        queueMicrotask(() => {
            api.ai.problemJobs(problemId)
                .then(r => {
                    const latest = (r.data || []).find(j => !['discarded', 'cancelled'].includes(j.status))
                    if (latest) {
                        setJob(latest)
                        // Restore artifact picker to match the resumed job
                        const art = latest.artifact_type as ArtifactId
                        if (ARTIFACTS.some(a => a.id === art)) setArtifact(art)
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

    // The poll that used to live here is gone: AIJobDetail owns the timer for the
    // job in view, and it is the only component that needs it. Two loops over one
    // job was the reason a cancel could appear to do nothing for a second.
    useEffect(() => {
        if (!job || !isJobActive(job)) return
        const id = window.setInterval(() => {
            api.ai.getJob(job.id).then(setJob).catch(() => {})
        }, 1500)
        return () => window.clearInterval(id)
    }, [job?.status, job?.id]) // eslint-disable-line react-hooks/exhaustive-deps

    const updateBrief = (patch: Partial<AIBrief>) => setBrief(b => ({ ...b, ...patch }))

    const start = async () => {
        setError('')
        setAppliedSlug('')
        try {
            const newJob = await api.ai.createJob({
                problem_id: problemId,
                ai_model_id: modelId || undefined,
                artifact_type: isNew ? 'full_problem' : artifact,
                brief,
            })
            setJob(newJob)
            refreshActive()
        } catch (e) {
            setError(e instanceof Error ? e.message : String(e))
        }
    }

    // Cancelling frees the setter's slot on the server immediately, so reflect
    // it here too. Refetching would show 'running' again until the worker
    // notices (up to a second), and the Generate button would stay blocked with
    // no visible reason.
    const cancelCurrent = async () => {
        if (!job) return
        setError('')
        try {
            await api.ai.cancelJob(job.id)
            setJob(j => (j ? { ...j, status: 'cancelled' } : j))
            setActiveJobs(list => list.filter(j => j.id !== job.id))
        } catch (e) {
            setError(e instanceof Error ? e.message : String(e))
        }
    }

    const apply = async (sections: string[]) => {
        if (!job) return
        setError('')
        try {
            const result = await api.ai.applyJob(job.id, sections, problemUpdatedAt)
            setJob(await api.ai.getJob(job.id))
            if (isNew) {
                setAppliedSlug(result.problem_slug)
                onCreated?.(result.problem_slug)
            }
            onApplied?.()
        } catch (e) {
            setError(e instanceof Error ? e.message : String(e))
        }
    }

    // Gate the button on the same rule the server enforces, so the click cannot
    // walk into a limit error. The form stays editable; only starting a run is
    // blocked.
    const blocking = activeJobs[0] ?? null
    const runningCount = activeJobs.filter(j => j.status === 'running').length
    const queuedCount = activeJobs.length - runningCount
    const canStart = connections.length > 0 && brief.tags.length > 0 && !blocking
    const blockedReason = blocking
        ? `A generation is already ${blocking.status} (${statusLabel(blocking.stage)}). Cancel it from the job list below to start another.`
        : brief.tags.length === 0
            ? 'Pick at least one topic.'
            : 'Connect an AI model first.'
    const artifactDef = ARTIFACTS.find(a => a.id === artifact)!
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
                    {/* Artifact scope picker — only shown for existing problems */}
                    {!isNew && (
                        <fieldset className="rounded-xl border border-hairline bg-surface p-4">
                            <legend className="px-2 text-sm font-semibold text-ink-soft">What do you want to generate?</legend>
                            <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
                                {ARTIFACTS.filter(a => a.id !== 'full_problem').map(a => (
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
                                        <span className="mt-0.5 text-xs text-muted">{a.desc}</span>
                                    </label>
                                ))}
                            </div>
                        </fieldset>
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

                    <AIBriefForm brief={brief} onChange={updateBrief} />

                    <div className="flex flex-wrap items-center gap-3">
                        <button
                            type="button"
                            disabled={!canStart}
                            onClick={start}
                            className="min-h-11 cursor-pointer rounded bg-brand px-5 text-on-brand disabled:opacity-50"
                        >
                            {isNew ? 'Generate problem' : `Generate ${artifactDef.label}`}
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
                    <div className="flex flex-wrap items-center justify-between gap-2">
                        <AIJobStatusBadge status={job.status} />
                    </div>

                    {/* Progress card */}
                    <div className="rounded border border-hairline p-4">
                        <div className="flex items-start justify-between text-sm">
                            <div>
                                <span className="font-medium capitalize">{statusLabel(job.stage)}</span>
                                <span className="ml-2 text-xs text-muted-soft capitalize">
                                    ({statusLabel(job.artifact_type)})
                                </span>
                            </div>
                            <span>{job.progress}%</span>
                        </div>
                        <div className="mt-2 h-2 rounded bg-surface-3">
                            <div
                                className={`h-2 rounded transition-all ${jobActive ? 'bg-brand' : 'bg-bad'}`}
                                style={{ width: `${job.progress}%` }}
                            />
                        </div>
                        <p className="mt-2 text-xs text-muted">
                            {statusLabel(job.status)} · {job.prompt_tokens + job.completion_tokens} tokens · {job.latency_ms} ms
                        </p>
                        <button
                            type="button"
                            onClick={() => { setHistoryJobId(job.id); setHistoryOpen(true) }}
                            className="mt-1.5 cursor-pointer text-xs text-brand hover:underline"
                        >
                            Open full status →
                        </button>
                    </div>

                    {jobActive && (
                        <button
                            type="button"
                            onClick={cancelCurrent}
                            className="min-h-11 cursor-pointer rounded border border-hairline-strong px-4"
                        >
                            Cancel
                        </button>
                    )}

                    {job.validation_report && (
                        <div className="rounded border border-hairline p-4">
                            <h3 className="mb-2 font-medium">Validation</h3>
                            <AIValidationList report={job.validation_report} />
                        </div>
                    )}

                    {/* Apply section — scoped to what this artifact produces, by the
                        same component the full status view uses, so the two cannot
                        offer different sections for one job. */}
                    {job.status === 'ready' && job.proposal && (
                        <div className="space-y-3">
                            <h3 className="font-medium">
                                {isNew ? 'Review the generated problem' : `Review proposed ${statusLabel(job.artifact_type)}`}
                            </h3>
                            <AIProposalReview
                                proposal={job.proposal}
                                artifactType={job.artifact_type}
                                requireStatement={isNew}
                                applyLabel={isNew ? 'Create problem' : 'Apply selected'}
                                onApply={apply}
                            />
                            <button
                                type="button"
                                onClick={() => api.ai.discardJob(job.id).then(() => { setJob(null); refreshActive() })}
                                className="min-h-11 cursor-pointer rounded border border-hairline-strong px-4"
                            >
                                Discard
                            </button>
                        </div>
                    )}

                    {(job.status === 'failed' || job.status === 'validation_failed' || job.status === 'cancelled') && (
                        <div>
                            <p className="text-sm text-bad">{job.error_message || `Job ${statusLabel(job.status)}`}</p>
                            <div className="mt-2 flex gap-2">
                                <button
                                    type="button"
                                    onClick={() => api.ai.retryJob(job.id).then(setJob).catch(e => setError(String(e)))}
                                    className="min-h-11 cursor-pointer rounded bg-brand px-4 text-on-brand"
                                >
                                    Retry
                                </button>
                                <button
                                    type="button"
                                    onClick={() => setJob(null)}
                                    className="min-h-11 cursor-pointer rounded border border-hairline-strong px-4"
                                >
                                    Start again
                                </button>
                            </div>
                        </div>
                    )}
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
