import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { AIGenerationJob } from '../../types/api'
import { canRetryJob, elapsed, isJobActive, totalTokens } from '../../lib/aiJobs'
import { errorMessage } from '../../lib/errors'
import { Button } from '../ui/Button'
import { Card, CardBody, CardHeader } from '../ui/Card'
import AIJobStatusBadge from './AIJobStatusBadge'
import AIValidationList from './AIValidationList'
import AIProposalReview from './AIProposalReview'

type Props = {
    jobId: string
    /**
     * Brand-new-problem mode. Passed through to the review panel, which forces and
     * locks the statement section, and used for the apply button's label.
     */
    isNew?: boolean
    /** Called with the created problem's slug when a new problem is applied. */
    onCreated?: (slug: string) => void
    /** Called after any successful apply, so a parent can refetch. */
    onApplied?: () => void
    /** Called when a ready-or-failed job is discarded, to clear the parent's selection. */
    onDiscarded?: () => void
    /** Called with the retried job's id — a retry creates a NEW job. */
    onRetried?: (newJobId: string) => void
}

function Stat({ label, value }: { label: string; value: string | number }) {
    return (
        <div className="rounded-xl border border-hairline bg-surface px-4 py-3">
            <p className="text-[11px] font-medium uppercase tracking-wide text-muted">{label}</p>
            <p className="tnum mt-1 text-base font-semibold text-ink">{value}</p>
        </div>
    )
}

/**
 * One job, live, with every action that applies to its state.
 *
 * This is the piece all three AI surfaces needed: AI Studio showed a running job
 * with cancel and retry, the detail page showed the same job with the same
 * actions plus stats and raw metadata, and both polled it every 1.5s with their
 * own copy of the loop. The divergence was already real — the detail page read
 * `busy` state per button and the Studio did not, so a double-click could fire two
 * cancels.
 *
 * Polling lives here and nowhere else: one job in view, one timer. The previous
 * arrangement had the list polling every 3s and the detail view every 1.5s over
 * the same endpoint, so opening a detail view while the list was mounted meant two
 * independent loops on one resource.
 */
export default function AIJobDetail({
    jobId,
    isNew = false,
    onCreated,
    onApplied,
    onDiscarded,
    onRetried,
}: Props) {
    const [job, setJob] = useState<AIGenerationJob | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState('')
    const [showRaw, setShowRaw] = useState(false)

    useEffect(() => {
        let cancelled = false
        // deferred a microtask: the spinner lands pre-paint (react-hooks/set-state-in-effect)
        queueMicrotask(() => {
            if (cancelled) return
            setLoading(true)
            api.ai.getJob(jobId)
                .then((j) => { if (!cancelled) setJob(j) })
                .catch((e) => { if (!cancelled) setError(errorMessage(e)) })
                .finally(() => { if (!cancelled) setLoading(false) })
        })
        return () => { cancelled = true }
    }, [jobId])

    // Poll while the job is active. Keyed on the id and the status so the timer is
    // torn down the moment the job settles, rather than running forever behind a
    // terminal status.
    const active = isJobActive(job)
    useEffect(() => {
        if (!active) return
        const id = window.setInterval(() => {
            api.ai.getJob(jobId).then(setJob).catch(() => {})
        }, 1500)
        return () => window.clearInterval(id)
    }, [active, jobId])

    const act = async (name: string, fn: () => Promise<unknown>) => {
        setBusy(name)
        setError('')
        try {
            await fn()
        } catch (e) {
            setError(errorMessage(e))
        } finally {
            setBusy('')
        }
    }

    const doCancel = () => act('cancel', async () => {
        await api.ai.cancelJob(jobId)
        setJob(await api.ai.getJob(jobId))
    })

    const doDiscard = () => act('discard', async () => {
        await api.ai.discardJob(jobId)
        onDiscarded?.()
    })

    const doRetry = () => act('retry', async () => {
        // A retry is a NEW job with the same brief. Handing the new id to the
        // parent is what lets the caller follow it — the Studio switched its own
        // state and the detail page navigated, and both were doing the same thing.
        const next = await api.ai.retryJob(jobId)
        onRetried?.(next.id)
    })

    const doApply = (sections: string[]) => act('apply', async () => {
        const result = await api.ai.applyJob(jobId, sections)
        setJob(await api.ai.getJob(jobId))
        if (isNew) onCreated?.(result.problem_slug)
        onApplied?.()
    })

    if (loading) {
        return <p className="text-sm text-muted">Loading job…</p>
    }
    if (!job) {
        return <p className="text-sm text-bad">{error || 'Job not found.'}</p>
    }

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
                <AIJobStatusBadge status={job.status} />
                <span className="font-mono text-xs text-muted-soft">{job.id.slice(0, 8)}</span>
                <span className="text-xs text-muted-soft">·</span>
                <span className="text-xs text-muted capitalize">{job.artifact_type.replaceAll('_', ' ')}</span>
            </div>

            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <Stat label="Progress" value={`${job.progress}%`} />
                <Stat label="Stage" value={job.stage.replaceAll('_', ' ')} />
                <Stat label="Tokens" value={totalTokens(job)} />
                <Stat label="Elapsed" value={elapsed(job)} />
            </div>

            {isJobActive(job) && (
                <div className="rounded-xl border border-hairline bg-surface p-4">
                    <div className="h-2 rounded bg-surface-3">
                        <div className="h-2 rounded bg-brand transition-all" style={{ width: `${job.progress}%` }} />
                    </div>
                    {job.heartbeat_at && (
                        <p className="mt-1.5 text-xs text-muted">
                            Last heartbeat: {new Date(job.heartbeat_at).toLocaleString()}
                        </p>
                    )}
                </div>
            )}

            {error && (
                <div className="rounded border border-bad/40 bg-bad/10 p-3 text-sm text-bad">{error}</div>
            )}

            {job.error_message && (
                <div className="rounded border border-bad/40 bg-bad/10 p-4">
                    <p className="text-sm font-medium text-bad">
                        {job.error_code || 'Error'}: {job.error_message}
                    </p>
                </div>
            )}

            <div className="flex flex-wrap gap-2">
                {isJobActive(job) && (
                    <Button variant="danger" size="sm" onClick={doCancel} disabled={busy === 'cancel'}>
                        {busy === 'cancel' ? 'Cancelling…' : 'Cancel job'}
                    </Button>
                )}
                {canRetryJob(job) && (
                    <Button size="sm" onClick={doRetry} disabled={busy === 'retry'}>
                        {busy === 'retry' ? 'Starting…' : 'Retry'}
                    </Button>
                )}
                {(canRetryJob(job) || job.status === 'ready') && (
                    <Button variant="secondary" size="sm" onClick={doDiscard} disabled={busy === 'discard'}>
                        {busy === 'discard' ? 'Discarding…' : 'Discard'}
                    </Button>
                )}
            </div>

            {job.validation_report && (
                <Card>
                    <CardHeader
                        title="Validation report"
                        description={job.validation_report.valid ? 'All gates passed.' : 'Some gates failed.'}
                    />
                    <CardBody>
                        <AIValidationList report={job.validation_report} />
                    </CardBody>
                </Card>
            )}

            {job.status === 'ready' && job.proposal && (
                <Card>
                    <CardHeader
                        title={isNew ? 'Review the generated problem' : 'Proposal — select sections to apply'}
                        description="Only the checked sections are written to the problem."
                    />
                    <CardBody>
                        <AIProposalReview
                            proposal={job.proposal}
                            artifactType={job.artifact_type}
                            requireStatement={isNew}
                            applyLabel={isNew ? 'Create problem' : 'Apply selected sections'}
                            busy={busy === 'apply'}
                            onApply={doApply}
                        />
                    </CardBody>
                </Card>
            )}

            {job.status === 'applied' && job.proposal && (
                <Card>
                    <CardHeader title="Proposal" description="Already applied." />
                    <CardBody>
                        <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-code px-3 py-2 font-mono text-xs text-code-ink">
                            {JSON.stringify(job.proposal, null, 2)}
                        </pre>
                    </CardBody>
                </Card>
            )}

            <Card>
                <CardHeader title="Brief" description="The generation request that created this job." />
                <CardBody>
                    <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-code px-3 py-2 font-mono text-xs text-code-ink">
                        {JSON.stringify(job.normalized_brief, null, 2)}
                    </pre>
                </CardBody>
            </Card>

            {/* Kept behind a disclosure rather than dropped: this is what an
                operator needs when a job fails for a reason the message does not
                name, and it was the only part of the old detail page with no
                equivalent anywhere else. */}
            <div className="rounded-xl border border-hairline bg-surface">
                <button
                    type="button"
                    onClick={() => setShowRaw((v) => !v)}
                    className="w-full cursor-pointer px-5 py-3 text-left text-sm font-medium text-ink-soft hover:text-ink"
                >
                    {showRaw ? 'Hide' : 'Show'} raw job metadata
                </button>
                {showRaw && (
                    <pre className="max-h-96 overflow-auto px-5 pb-4 font-mono text-xs text-muted">
                        {JSON.stringify(job, null, 2)}
                    </pre>
                )}
            </div>
        </div>
    )
}
