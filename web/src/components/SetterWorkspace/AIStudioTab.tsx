import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { AIBrief, AIGenerationJob, AIModel, AIProposal } from '../../types/api'
import { AIBriefForm } from '../AI/AIBriefForm'
import { DEFAULT_BRIEF } from '../../lib/aiBrief'

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
}

const artifacts = [
    ['full_problem', 'Full package'],
    ['statement', 'Statement & samples'],
    ['tests', 'Tests & generator'],
    ['solution', 'Reference solution'],
    ['editorial', 'Editorial'],
] as const

function previewValue(v: unknown) {
    return typeof v === 'string' ? v : JSON.stringify(v, null, 2)
}

export default function AIStudioTab({ problemId, problemUpdatedAt, onApplied, onCreated }: Props) {
    const isNew = !problemId
    const [connections, setConnections] = useState<AIModel[]>([])
    const [modelId, setModelId] = useState<string>('')
    // A new problem must be generated as a whole package: the other artifact
    // scopes only rewrite parts of an existing one.
    const [artifact, setArtifact] = useState('full_problem')
    const [brief, setBrief] = useState<AIBrief>({ ...DEFAULT_BRIEF })
    const [job, setJob] = useState<AIGenerationJob | null>(null)
    // A brand-new problem has no existing sections to merge with, so the
    // statement must always be applied or the created problem has no content.
    const [selected, setSelected] = useState<string[]>(['statement', 'solution', 'tests'])
    const [error, setError] = useState('')
    const [appliedSlug, setAppliedSlug] = useState<string>('')

    useEffect(() => {
        api.ai.connections()
            .then(r => {
                const list = r.data || []
                setConnections(list)
                if (list.length > 0) setModelId(id => id || list[0].id)
            })
            .catch(e => setError(String(e)))
    }, [])

    // Resume the newest unfinished/reviewable job for this problem. Without this
    // the tab always opened on a blank form, so a finished proposal was
    // unreachable after a reload and could never be reviewed or applied.
    useEffect(() => {
        if (!problemId) return
        queueMicrotask(() => {
            api.ai.problemJobs(problemId)
                .then(r => {
                    const latest = (r.data || []).find(j => !['discarded', 'cancelled'].includes(j.status))
                    if (latest) setJob(latest)
                })
                .catch(() => {})
        })
    }, [problemId])

    useEffect(() => {
        if (!job || !['queued', 'running'].includes(job.status)) return
        const id = window.setInterval(() => api.ai.getJob(job.id).then(setJob).catch(() => {}), 1500)
        return () => window.clearInterval(id)
    }, [job])

    const updateBrief = (patch: Partial<AIBrief>) => setBrief(b => ({ ...b, ...patch }))

    const start = async () => {
        setError('')
        setAppliedSlug('')
        try {
            setJob(await api.ai.createJob({
                problem_id: problemId,
                ai_model_id: modelId || undefined,
                artifact_type: isNew ? 'full_problem' : artifact,
                brief,
            }))
        } catch (e) {
            setError(e instanceof Error ? e.message : String(e))
        }
    }

    const apply = async () => {
        if (!job) return
        setError('')
        try {
            const sections = isNew && !selected.includes('statement') ? ['statement', ...selected] : selected
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

    const proposal: AIProposal | undefined = job?.proposal
    const canStart = connections.length > 0 && brief.tags.length > 0

    return (
        <div className="space-y-5">
            <div>
                <h2 className="text-lg font-semibold">
                    {isNew ? 'Create a problem with AI' : 'AI Studio'}
                </h2>
                <p className="text-sm text-muted">
                    {isNew
                        ? 'Describe the problem you want. The harness drafts it, generates test data, and validates the reference solution before you publish anything.'
                        : 'Generate a reviewable proposal. Nothing is overwritten until you apply selected sections.'}
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
                    {!isNew && (
                        <div className="flex flex-wrap gap-2">
                            {artifacts.map(([id, label]) => (
                                <button
                                    key={id}
                                    type="button"
                                    onClick={() => setArtifact(id)}
                                    className={`min-h-11 cursor-pointer rounded border px-3 text-sm ${artifact === id ? 'border-brand bg-brand text-on-brand' : 'border-hairline-strong'}`}
                                >
                                    {label}
                                </button>
                            ))}
                        </div>
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
                            {isNew ? 'Generate problem' : 'Generate proposal'}
                        </button>
                        {!canStart && (
                            <span className="text-xs text-muted">
                                {brief.tags.length === 0 ? 'Pick at least one topic.' : 'Connect an AI model first.'}
                            </span>
                        )}
                    </div>
                </div>
            )}

            {job && (
                <div className="space-y-4">
                    <div className="rounded border border-hairline p-4">
                        <div className="flex justify-between text-sm">
                            <span className="font-medium capitalize">{job.stage.replaceAll('_', ' ')}</span>
                            <span>{job.progress}%</span>
                        </div>
                        <div className="mt-2 h-2 rounded bg-surface-3">
                            <div className="h-2 rounded bg-brand" style={{ width: `${job.progress}%` }} />
                        </div>
                        <p className="mt-2 text-xs text-muted">
                            {job.status} · {job.prompt_tokens + job.completion_tokens} tokens · {job.latency_ms} ms
                        </p>
                    </div>

                    {job.status === 'running' || job.status === 'queued' ? (
                        <button
                            type="button"
                            onClick={() => api.ai.cancelJob(job.id).then(() => api.ai.getJob(job.id).then(setJob))}
                            className="min-h-11 cursor-pointer rounded border border-hairline-strong px-4"
                        >
                            Cancel
                        </button>
                    ) : null}

                    {job.validation_report && (
                        <div className="rounded border border-hairline p-4">
                            <h3 className="mb-2 font-medium">Validation</h3>
                            {job.validation_report.checks.map(c => (
                                <p key={c.name} className={`text-sm ${c.passed ? 'text-ok' : 'text-bad'}`}>
                                    {c.passed ? '✓' : '✗'} {c.name}: {c.message}
                                </p>
                            ))}
                        </div>
                    )}

                    {proposal && job.status === 'ready' && (
                        <div className="space-y-3">
                            <h3 className="font-medium">
                                {isNew ? 'Review the generated problem' : 'Review proposed artifacts'}
                            </h3>
                            {(['statement', 'solution', 'tests'] as const).map(section => (
                                <label key={section} className="block rounded border border-hairline p-3">
                                    <span className="flex items-center gap-2 font-medium">
                                        <input
                                            type="checkbox"
                                            checked={selected.includes(section) || (isNew && section === 'statement')}
                                            disabled={isNew && section === 'statement'}
                                            onChange={e => setSelected(e.target.checked ? [...selected, section] : selected.filter(x => x !== section))}
                                        />
                                        {section}
                                    </span>
                                    <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap text-xs text-muted">
                                        {previewValue(
                                            section === 'statement'
                                                ? { title: proposal.title, description: proposal.description }
                                                : section === 'solution'
                                                    ? proposal.solution_code
                                                    : { generator: proposal.generator_code, seeds: proposal.seeds },
                                        )}
                                    </pre>
                                </label>
                            ))}
                            <div className="flex flex-wrap gap-2">
                                <button type="button" onClick={apply} className="min-h-11 cursor-pointer rounded bg-brand px-4 text-on-brand">
                                    {isNew ? 'Create problem' : 'Apply selected'}
                                </button>
                                <button
                                    type="button"
                                    onClick={() => api.ai.discardJob(job.id).then(() => setJob(null))}
                                    className="min-h-11 cursor-pointer rounded border border-hairline-strong px-4"
                                >
                                    Discard
                                </button>
                            </div>
                        </div>
                    )}

                    {['failed', 'validation_failed', 'cancelled'].includes(job.status) && (
                        <div>
                            <p className="text-sm text-bad">{job.error_message || `Job ${job.status}`}</p>
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
        </div>
    )
}
