import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { RefreshCw } from 'lucide-react'
import { api } from '../lib/api'
import type { AIGenerationJob, AIProposal } from '../types/api'
import { Skeleton } from '../components/ui/PageHeader'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card, CardBody, CardHeader } from '../components/ui/Card'
import { errorMessage } from '../lib/errors'

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

function decodeUserID(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.uid ?? null
    } catch {
        return null
    }
}

const STATUS_TONES: Record<string, BadgeTone> = {
    queued: 'neutral',
    running: 'brand',
    ready: 'ok',
    validation_failed: 'warn',
    failed: 'bad',
    cancelled: 'neutral',
    applied: 'ok',
    discarded: 'neutral',
}

function StatusBadge({ status }: { status: string }) {
    return (
        <Badge tone={STATUS_TONES[status] ?? 'neutral'} dot={status === 'running' || status === 'queued'}>
            {status.replaceAll('_', ' ')}
        </Badge>
    )
}

function elapsed(job: AIGenerationJob): string {
    if (job.finished_at && job.started_at) {
        const ms = new Date(job.finished_at).getTime() - new Date(job.started_at).getTime()
        return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`
    }
    if (job.latency_ms > 0) return `${(job.latency_ms / 1000).toFixed(1)} s`
    return '—'
}

function Stat({ label, value }: { label: string; value: string | number }) {
    return (
        <div className="rounded-xl border border-hairline bg-surface px-4 py-3">
            <p className="text-[11px] font-medium uppercase tracking-wide text-muted">{label}</p>
            <p className="tnum mt-1 text-base font-semibold text-ink">{value}</p>
        </div>
    )
}

function previewValue(v: unknown) {
    return typeof v === 'string' ? v : JSON.stringify(v, null, 2)
}

export default function AIJobDetailPage() {
    const { id } = useParams<{ id: string }>()
    const role = decodeRole()
    const userID = decodeUserID()
    const isAdmin = role === 'admin'
    const navigate = useNavigate()

    const [job, setJob] = useState<AIGenerationJob | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [actionError, setActionError] = useState('')
    const [busy, setBusy] = useState('')

    // Apply selection
    const [selected, setSelected] = useState<string[]>(['statement', 'solution', 'tests'])

    const loadJob = () => {
        if (!id) return
        api.ai.getJob(id)
            .then(setJob)
            .catch(e => setError(errorMessage(e)))
            .finally(() => setLoading(false))
    }

    useEffect(() => { loadJob() }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

    // Poll while active
    useEffect(() => {
        if (!job || !['queued', 'running'].includes(job.status)) return
        const interval = window.setInterval(() => api.ai.getJob(id!).then(setJob).catch(() => {}), 1500)
        return () => window.clearInterval(interval)
    }, [job, id])

    const doCancel = async () => {
        if (!job) return
        setBusy('cancel')
        setActionError('')
        try {
            if (isAdmin && job.actor_id !== userID) {
                await api.ai.adminCancelJob(job.id)
            } else {
                await api.ai.cancelJob(job.id)
            }
            const updated = await api.ai.getJob(job.id)
            setJob(updated)
        } catch (e) {
            setActionError(errorMessage(e))
        } finally {
            setBusy('')
        }
    }

    const doDiscard = async () => {
        if (!job) return
        setBusy('discard')
        setActionError('')
        try {
            await api.ai.discardJob(job.id)
            const updated = await api.ai.getJob(job.id)
            setJob(updated)
        } catch (e) {
            setActionError(errorMessage(e))
        } finally {
            setBusy('')
        }
    }

    const doRetry = async () => {
        if (!job) return
        setBusy('retry')
        setActionError('')
        try {
            const newJob = await api.ai.retryJob(job.id)
            navigate(`/setter/ai/jobs/${newJob.id}`)
        } catch (e) {
            setActionError(errorMessage(e))
        } finally {
            setBusy('')
        }
    }

    const doApply = async () => {
        if (!job) return
        setBusy('apply')
        setActionError('')
        try {
            const result = await api.ai.applyJob(job.id, selected)
            const updated = await api.ai.getJob(job.id)
            setJob(updated)
            navigate(`/setter/${result.problem_slug}`)
        } catch (e) {
            setActionError(errorMessage(e))
        } finally {
            setBusy('')
        }
    }

    if (!role || (role !== 'admin' && role !== 'setter')) {
        return (
            <div className="mx-auto mt-20 max-w-md text-center text-muted">
                <p className="text-lg font-semibold">Access restricted</p>
                <p className="text-sm mt-1">Only admins and setters can view AI generation jobs.</p>
            </div>
        )
    }

    if (loading) {
        return (
            <div className="space-y-4">
                <Skeleton className="h-8 w-56" />
                <Skeleton className="h-4 w-80" />
                <Skeleton className="h-48 w-full" />
            </div>
        )
    }

    if (error || !job) {
        return (
            <div className="py-20 text-center">
                <p className="text-sm text-bad">{error || 'Job not found.'}</p>
                <Link to="/setter/ai/jobs" className="mt-2 inline-block text-sm text-brand hover:underline">
                    Back to AI Jobs
                </Link>
            </div>
        )
    }

    const isOwner = job.actor_id === userID
    const canAct = isAdmin || isOwner
    const proposal: AIProposal | undefined = job.proposal
    const isActive = job.status === 'queued' || job.status === 'running'

    return (
        <div className="max-w-4xl space-y-6">
            {/* Breadcrumb + Header */}
            <div>
                <nav className="mb-2 flex items-center gap-1.5 text-xs text-muted">
                    <Link to="/setter" className="hover:text-ink">Setter workspace</Link>
                    <span aria-hidden="true" className="text-hairline-strong">/</span>
                    <Link to="/setter/ai/jobs" className="hover:text-ink">AI Jobs</Link>
                    <span aria-hidden="true" className="text-hairline-strong">/</span>
                    <span className="font-mono text-muted-soft">{job.id.slice(0, 8)}</span>
                </nav>
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                        <h1 className="flex flex-wrap items-center gap-2.5 text-2xl font-semibold tracking-tight text-ink">
                            AI Job
                            <span className="font-mono text-lg text-muted">{job.id.slice(0, 8)}</span>
                        </h1>
                        <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted">
                            <span className="capitalize">{job.artifact_type.replaceAll('_', ' ')}</span>
                            {job.problem_id && (
                                <>
                                    <span aria-hidden="true">·</span>
                                    <span className="font-mono text-xs">{job.problem_id.slice(0, 8)}</span>
                                </>
                            )}
                            <span aria-hidden="true">·</span>
                            <time dateTime={job.created_at}>{new Date(job.created_at).toLocaleString()}</time>
                            {isAdmin && job.actor_id !== userID && (
                                <>
                                    <span aria-hidden="true">·</span>
                                    <span className="rounded bg-surface-3 px-1.5 py-0.5 font-mono text-xs">
                                        actor: {job.actor_id.slice(0, 8)}
                                    </span>
                                </>
                            )}
                        </p>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                        <StatusBadge status={job.status} />
                        <Button variant="secondary" size="sm" onClick={loadJob}>
                            <RefreshCw className="h-3.5 w-3.5" />
                        </Button>
                    </div>
                </div>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <Stat label="Progress" value={`${job.progress}%`} />
                <Stat label="Stage" value={job.stage.replaceAll('_', ' ')} />
                <Stat label="Tokens" value={(job.prompt_tokens + job.completion_tokens).toLocaleString()} />
                <Stat label="Elapsed" value={elapsed(job)} />
            </div>

            {/* Progress bar */}
            {isActive && (
                <div className="rounded-xl border border-hairline bg-surface p-4">
                    <div className="flex justify-between text-sm">
                        <span className="font-medium capitalize">{job.stage.replaceAll('_', ' ')}</span>
                        <span>{job.progress}%</span>
                    </div>
                    <div className="mt-2 h-2 rounded bg-surface-3">
                        <div
                            className="h-2 rounded bg-brand transition-all"
                            style={{ width: `${job.progress}%` }}
                        />
                    </div>
                    {job.heartbeat_at && (
                        <p className="mt-1.5 text-xs text-muted">
                            Last heartbeat: {new Date(job.heartbeat_at).toLocaleString()}
                        </p>
                    )}
                </div>
            )}

            {/* Error */}
            {(job.status === 'failed' || job.status === 'validation_failed') && job.error_message && (
                <div className="rounded border border-bad/40 bg-bad/10 p-4">
                    <p className="mb-1 text-sm font-medium text-bad">
                        {job.error_code || 'Error'}: {job.error_message}
                    </p>
                </div>
            )}

            {/* Action error */}
            {actionError && (
                <div className="rounded border border-bad/40 bg-bad/10 p-3 text-sm text-bad">
                    {actionError}
                </div>
            )}

            {/* Action buttons */}
            {canAct && (
                <div className="flex flex-wrap gap-2">
                    {isActive && (
                        <Button
                            variant="danger"
                            size="sm"
                            onClick={doCancel}
                            disabled={busy === 'cancel'}
                        >
                            {busy === 'cancel' ? 'Cancelling…' : 'Cancel job'}
                        </Button>
                    )}
                    {(job.status === 'failed' || job.status === 'validation_failed' || job.status === 'cancelled') && (
                        <>
                            <Button size="sm" onClick={doRetry} disabled={busy === 'retry'}>
                                {busy === 'retry' ? 'Starting…' : 'Retry'}
                            </Button>
                            <Button variant="secondary" size="sm" onClick={doDiscard} disabled={busy === 'discard'}>
                                {busy === 'discard' ? 'Discarding…' : 'Discard'}
                            </Button>
                        </>
                    )}
                    {job.status === 'ready' && proposal && (
                        <Button variant="secondary" size="sm" onClick={doDiscard} disabled={busy === 'discard'}>
                            {busy === 'discard' ? 'Discarding…' : 'Discard'}
                        </Button>
                    )}
                </div>
            )}

            {/* Validation Report */}
            {job.validation_report && (
                <Card>
                    <CardHeader
                        title="Validation report"
                        description={job.validation_report.valid ? 'All gates passed.' : 'Some gates failed.'}
                    />
                    <CardBody>
                        <ul className="space-y-1.5">
                            {job.validation_report.checks.map(c => (
                                <li key={c.name} className={`flex items-start gap-2 text-sm ${c.passed ? 'text-ok' : 'text-bad'}`}>
                                    <span className="mt-0.5 shrink-0">{c.passed ? '✓' : '✗'}</span>
                                    <span>
                                        <span className="font-medium">{c.name}</span>
                                        {c.message && <span className="ml-1 text-xs text-muted">— {c.message}</span>}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    </CardBody>
                </Card>
            )}

            {/* Proposal + Apply */}
            {proposal && job.status === 'ready' && canAct && (
                <Card>
                    <CardHeader
                        title="Proposal — select sections to apply"
                        description="Only the checked sections will be written to the problem."
                    />
                    <CardBody className="space-y-4">
                        {(['statement', 'solution', 'tests'] as const).map(section => {
                            const hasContent =
                                section === 'statement'
                                    ? !!proposal.title
                                    : section === 'solution'
                                    ? !!proposal.solution_code
                                    : !!proposal.generator_code

                            if (!hasContent) return null
                            return (
                                <label key={section} className="block cursor-pointer rounded border border-hairline p-3 hover:bg-surface-2">
                                    <span className="flex items-center gap-2 font-medium text-sm capitalize">
                                        <input
                                            type="checkbox"
                                            checked={selected.includes(section)}
                                            onChange={e => setSelected(e.target.checked
                                                ? [...selected, section]
                                                : selected.filter(x => x !== section))}
                                            className="h-4 w-4"
                                        />
                                        {section}
                                    </span>
                                    <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-code px-3 py-2 font-mono text-xs text-code-ink">
                                        {previewValue(
                                            section === 'statement'
                                                ? { title: proposal.title, description: proposal.description }
                                                : section === 'solution'
                                                ? proposal.solution_code
                                                : { generator: proposal.generator_code, seeds: proposal.seeds },
                                        )}
                                    </pre>
                                </label>
                            )
                        })}

                        {proposal.title && (
                            <div className="space-y-1">
                                <p className="text-sm font-medium">Tags &amp; difficulty</p>
                                <p className="text-sm text-muted-soft">
                                    {proposal.difficulty} · {proposal.tags?.join(', ')}
                                </p>
                                {proposal.time_limit_ms && (
                                    <p className="text-xs text-muted">
                                        {proposal.time_limit_ms} ms · {((proposal.memory_limit_kb ?? 262144) / 1024).toFixed(0)} MB
                                    </p>
                                )}
                            </div>
                        )}

                        <Button
                            size="sm"
                            onClick={doApply}
                            disabled={busy === 'apply' || selected.length === 0}
                        >
                            {busy === 'apply' ? 'Applying…' : 'Apply selected sections'}
                        </Button>
                    </CardBody>
                </Card>
            )}

            {/* Brief */}
            <Card>
                <CardHeader title="Brief" description="The generation request that created this job." />
                <CardBody>
                    <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-code px-3 py-2 font-mono text-xs text-code-ink">
                        {JSON.stringify(job.normalized_brief, null, 2)}
                    </pre>
                </CardBody>
            </Card>

            {/* Raw job info */}
            <details className="rounded-xl border border-hairline bg-surface">
                <summary className="cursor-pointer px-5 py-3 text-sm font-medium text-ink-soft hover:text-ink">
                    Raw job metadata
                </summary>
                <pre className="max-h-96 overflow-auto px-5 pb-4 font-mono text-xs text-muted">
                    {JSON.stringify(
                        {
                            id: job.id,
                            actor_id: job.actor_id,
                            problem_id: job.problem_id,
                            ai_model_id: job.ai_model_id,
                            artifact_type: job.artifact_type,
                            status: job.status,
                            stage: job.stage,
                            progress: job.progress,
                            prompt_tokens: job.prompt_tokens,
                            completion_tokens: job.completion_tokens,
                            latency_ms: job.latency_ms,
                            error_code: job.error_code,
                            error_message: job.error_message,
                            created_at: job.created_at,
                            started_at: job.started_at,
                            finished_at: job.finished_at,
                            applied_at: job.applied_at,
                            cancel_requested_at: job.cancel_requested_at,
                        },
                        null, 2
                    )}
                </pre>
            </details>
        </div>
    )
}
