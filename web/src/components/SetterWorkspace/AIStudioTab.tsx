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

// Artifact types with labels and what sections they produce.
const ARTIFACTS = [
    { id: 'full_problem',  label: 'Full package',         desc: 'Statement, tests, generator & reference solution', sections: ['statement', 'solution', 'tests'] },
    { id: 'statement',     label: 'Statement & samples',  desc: 'Rewrites the problem statement only', sections: ['statement'] },
    { id: 'tests',         label: 'Tests & generator',    desc: 'Generates a new test suite and generator', sections: ['tests'] },
    { id: 'solution',      label: 'Reference solution',   desc: 'Writes or replaces the reference solution', sections: ['solution'] },
    { id: 'editorial',     label: 'Editorial',            desc: 'Authors an editorial for this problem', sections: ['editorial'] },
] as const

type ArtifactId = typeof ARTIFACTS[number]['id']

/** Map each artifact to the apply section keys it can produce. */
const ARTIFACT_SECTIONS: Record<ArtifactId, string[]> = {
    full_problem: ['statement', 'solution', 'tests'],
    statement:    ['statement'],
    tests:        ['tests'],
    solution:     ['solution'],
    editorial:    ['editorial'],
}

/** Human-readable labels for the apply checkboxes. */
const SECTION_LABELS: Record<string, string> = {
    statement: 'Statement & samples',
    solution:  'Reference solution',
    tests:     'Test data & generator',
    editorial: 'Editorial',
}

function previewValue(section: string, proposal: AIProposal) {
    switch (section) {
        case 'statement':
            return JSON.stringify({ title: proposal.title, description: proposal.description }, null, 2)
        case 'solution':
            return proposal.solution_code || '(no solution code)'
        case 'tests':
            return JSON.stringify({ generator: proposal.generator_code, seeds: proposal.seeds }, null, 2)
        case 'editorial':
            return JSON.stringify({ title: proposal.editorial_title, content: proposal.editorial_content }, null, 2)
        default:
            return ''
    }
}

export default function AIStudioTab({ problemId, problemUpdatedAt, onApplied, onCreated }: Props) {
    const isNew = !problemId
    const [connections, setConnections] = useState<AIModel[]>([])
    const [modelId, setModelId] = useState<string>('')
    // A new problem must be generated as a whole package.
    // For existing problems default to nothing selected so the user has to actively choose the scope.
    const [artifact, setArtifact] = useState<ArtifactId>(isNew ? 'full_problem' : 'statement')
    const [brief, setBrief] = useState<AIBrief>({ ...DEFAULT_BRIEF })
    const [job, setJob] = useState<AIGenerationJob | null>(null)
    const [selected, setSelected] = useState<string[]>([])
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
                        if (ARTIFACT_SECTIONS[art]) setArtifact(art)
                    }
                })
                .catch(() => {})
        })
    }, [problemId])

    // Poll while running.
    useEffect(() => {
        if (!job || !['queued', 'running'].includes(job.status)) return
        const id = window.setInterval(() => api.ai.getJob(job.id).then(setJob).catch(() => {}), 1500)
        return () => window.clearInterval(id)
    }, [job])

    // When a job becomes ready, pre-select all sections that have content.
    useEffect(() => {
        if (!job || job.status !== 'ready' || !job.proposal) return
        const art = (job.artifact_type as ArtifactId)
        const candidateSections = ARTIFACT_SECTIONS[art] ?? []
        const p = job.proposal
        const withContent = candidateSections.filter(s => {
            if (s === 'statement') return !!p.title
            if (s === 'solution') return !!p.solution_code
            if (s === 'tests') return !!p.generator_code
            if (s === 'editorial') return !!p.editorial_title || !!p.editorial_content
            return false
        })
        setSelected(withContent)
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
        } catch (e) {
            setError(e instanceof Error ? e.message : String(e))
        }
    }

    const apply = async () => {
        if (!job) return
        setError('')
        try {
            // For a new problem the statement section is mandatory.
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
    const artifactDef = ARTIFACTS.find(a => a.id === artifact)!
    const applySections = ARTIFACT_SECTIONS[job?.artifact_type as ArtifactId] ?? []

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
                            <span className="text-xs text-muted">
                                {brief.tags.length === 0 ? 'Pick at least one topic.' : 'Connect an AI model first.'}
                            </span>
                        )}
                    </div>
                </div>
            )}

            {job && (
                <div className="space-y-4">
                    {/* Progress card */}
                    <div className="rounded border border-hairline p-4">
                        <div className="flex items-start justify-between text-sm">
                            <div>
                                <span className="font-medium capitalize">{job.stage.replaceAll('_', ' ')}</span>
                                <span className="ml-2 text-xs text-muted-soft capitalize">
                                    ({job.artifact_type.replaceAll('_', ' ')})
                                </span>
                            </div>
                            <span>{job.progress}%</span>
                        </div>
                        <div className="mt-2 h-2 rounded bg-surface-3">
                            <div
                                className={`h-2 rounded transition-all ${
                                    job.status === 'failed' || job.status === 'validation_failed' ? 'bg-bad' : 'bg-brand'
                                }`}
                                style={{ width: `${job.progress}%` }}
                            />
                        </div>
                        <p className="mt-2 text-xs text-muted">
                            {job.status} · {job.prompt_tokens + job.completion_tokens} tokens · {job.latency_ms} ms
                        </p>
                        <a
                            href={`/setter/ai/jobs/${job.id}`}
                            className="mt-1.5 block text-xs text-brand hover:underline"
                        >
                            Open full status page →
                        </a>
                    </div>

                    {(job.status === 'running' || job.status === 'queued') ? (
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

                    {/* Apply section — scoped to what this artifact produces */}
                    {proposal && job.status === 'ready' && (
                        <div className="space-y-3">
                            <h3 className="font-medium">
                                {isNew ? 'Review the generated problem' : `Review proposed ${job.artifact_type.replaceAll('_', ' ')}`}
                            </h3>
                            {applySections.map(section => {
                                // Check if the proposal actually has content for this section
                                const hasContent =
                                    section === 'statement' ? !!proposal.title :
                                    section === 'solution' ? !!proposal.solution_code :
                                    section === 'tests' ? !!proposal.generator_code :
                                    section === 'editorial' ? (!!proposal.editorial_title || !!proposal.editorial_content) :
                                    false
                                if (!hasContent) return null
                                const isForced = isNew && section === 'statement'
                                return (
                                    <label key={section} className="block rounded border border-hairline p-3 cursor-pointer hover:bg-surface-2">
                                        <span className="flex items-center gap-2 font-medium">
                                            <input
                                                type="checkbox"
                                                checked={selected.includes(section) || isForced}
                                                disabled={isForced}
                                                onChange={e => setSelected(e.target.checked
                                                    ? [...selected, section]
                                                    : selected.filter(x => x !== section))}
                                            />
                                            {SECTION_LABELS[section] || section}
                                        </span>
                                        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap text-xs text-muted">
                                            {previewValue(section, proposal)}
                                        </pre>
                                    </label>
                                )
                            })}
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
