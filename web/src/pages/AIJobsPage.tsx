import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { RefreshCw } from 'lucide-react'
import { api } from '../lib/api'
import type { AIGenerationJob } from '../types/api'
import { PageHeader, Skeleton } from '../components/ui/PageHeader'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Table, TableShell, Td, Th, THead } from '../components/ui/Table'
import { EmptyState } from '../components/EmptyState'
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

const STATUSES = ['', 'queued', 'running', 'ready', 'validation_failed', 'failed', 'cancelled', 'applied', 'discarded']

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
        return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`
    }
    if (job.started_at && (job.status === 'running' || job.status === 'queued')) {
        const ms = Date.now() - new Date(job.started_at).getTime()
        return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`
    }
    if (job.latency_ms > 0) return `${(job.latency_ms / 1000).toFixed(1)}s`
    return '—'
}

export default function AIJobsPage() {
    const role = decodeRole()
    const isAdmin = role === 'admin'
    const navigate = useNavigate()

    const [jobs, setJobs] = useState<AIGenerationJob[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [filterStatus, setFilterStatus] = useState('')
    const limit = 50

    // Admin-only: actor filter
    const [actorFilter, setActorFilter] = useState('')

    const load = () => {
        setLoading(true)
        setError('')
        const req = isAdmin
            ? api.ai.adminListJobs({ offset, limit, status: filterStatus || undefined, actor_id: actorFilter || undefined })
            : api.ai.listJobs(offset, limit)
        req.then(r => {
            setJobs(r.data || [])
            setTotal(r.total || 0)
        }).catch(e => setError(errorMessage(e))).finally(() => setLoading(false))
    }

    useEffect(() => { load() }, [offset, filterStatus, actorFilter]) // eslint-disable-line react-hooks/exhaustive-deps

    // Poll if any jobs are active
    useEffect(() => {
        const hasActive = jobs.some(j => j.status === 'queued' || j.status === 'running')
        if (!hasActive) return
        const id = window.setInterval(load, 3000)
        return () => window.clearInterval(id)
    }, [jobs]) // eslint-disable-line react-hooks/exhaustive-deps

    if (!role || (role !== 'admin' && role !== 'setter')) {
        return (
            <div className="mx-auto mt-20 max-w-md text-center text-muted">
                <p className="text-lg font-semibold">Access restricted</p>
                <p className="text-sm mt-1">Only admins and setters can view AI generation jobs.</p>
            </div>
        )
    }

    return (
        <div>
            <PageHeader
                eyebrow={<Link to="/setter" className="hover:text-ink">← Setter workspace</Link>}
                title="AI Generation Jobs"
                description={
                    isAdmin
                        ? 'All AI problem generation jobs across all setters. Admins can force-cancel any running or queued job.'
                        : 'Your AI problem generation jobs. Click a row to see the full status and proposal.'
                }
                actions={
                    <Button variant="secondary" size="sm" onClick={load}>
                        <RefreshCw className="h-3.5 w-3.5" />
                        Refresh
                    </Button>
                }
            />

            {/* Filters */}
            <div className="mb-4 flex flex-wrap gap-3">
                <label className="text-sm text-muted-soft">
                    Status
                    <select
                        value={filterStatus}
                        onChange={e => { setFilterStatus(e.target.value); setOffset(0) }}
                        className="ml-2 rounded border border-hairline-strong bg-surface px-2 py-1 text-xs text-ink"
                    >
                        {STATUSES.map(s => (
                            <option key={s} value={s}>{s || 'All statuses'}</option>
                        ))}
                    </select>
                </label>
                {isAdmin && (
                    <label className="text-sm text-muted-soft">
                        Actor ID
                        <input
                            type="text"
                            value={actorFilter}
                            placeholder="UUID or empty for all"
                            onChange={e => { setActorFilter(e.target.value); setOffset(0) }}
                            className="ml-2 rounded border border-hairline-strong bg-surface px-2 py-1 text-xs text-ink w-72"
                        />
                    </label>
                )}
            </div>

            {error && (
                <div className="mb-4 rounded border border-bad/40 bg-bad/10 p-3 text-sm text-bad">{error}</div>
            )}

            {loading && jobs.length === 0 ? (
                <div className="space-y-2">
                    {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
                </div>
            ) : jobs.length === 0 ? (
                <div className="rounded-xl border border-hairline bg-surface">
                    <EmptyState
                        text="No AI generation jobs"
                        description="AI problem generation jobs will appear here once a setter kicks one off from the AI Studio tab."
                    />
                </div>
            ) : (
                <TableShell>
                    <Table>
                        <THead>
                            <tr>
                                <Th>Job ID</Th>
                                {isAdmin && <Th>Actor</Th>}
                                <Th>Artifact</Th>
                                <Th>Status</Th>
                                <Th>Progress</Th>
                                <Th className="hidden sm:table-cell">Tokens</Th>
                                <Th className="hidden md:table-cell">Time</Th>
                                <Th className="hidden lg:table-cell">Created</Th>
                                <Th className="text-right">Actions</Th>
                            </tr>
                        </THead>
                        <tbody>
                            {jobs.map(job => (
                                <tr
                                    key={job.id}
                                    className="cursor-pointer border-b border-hairline last:border-0 transition-colors hover:bg-surface-2"
                                    onClick={() => navigate(`/setter/ai/jobs/${job.id}`)}
                                >
                                    <Td>
                                        <span className="font-mono text-xs text-brand">{job.id.slice(0, 8)}</span>
                                        {job.problem_id && (
                                            <span className="ml-1 text-xs text-muted-soft">· problem</span>
                                        )}
                                    </Td>
                                    {isAdmin && (
                                        <Td>
                                            <span className="font-mono text-xs text-muted">{job.actor_id.slice(0, 8)}</span>
                                        </Td>
                                    )}
                                    <Td>
                                        <span className="text-xs text-ink-soft">{job.artifact_type.replaceAll('_', ' ')}</span>
                                    </Td>
                                    <Td>
                                        <StatusBadge status={job.status} />
                                    </Td>
                                    <Td>
                                        <div className="flex items-center gap-2">
                                            <div className="h-1.5 w-16 overflow-hidden rounded-full bg-surface-3">
                                                <div
                                                    className={`h-1.5 rounded-full ${job.status === 'failed' || job.status === 'validation_failed' ? 'bg-bad' : 'bg-brand'}`}
                                                    style={{ width: `${job.progress}%` }}
                                                />
                                            </div>
                                            <span className="tnum text-xs text-muted">{job.progress}%</span>
                                        </div>
                                    </Td>
                                    <Td className="hidden tnum text-xs text-muted sm:table-cell">
                                        {job.prompt_tokens + job.completion_tokens > 0
                                            ? (job.prompt_tokens + job.completion_tokens).toLocaleString()
                                            : '—'}
                                    </Td>
                                    <Td className="hidden tnum text-xs text-muted md:table-cell">
                                        {elapsed(job)}
                                    </Td>
                                    <Td className="hidden text-xs text-muted-soft lg:table-cell">
                                        {new Date(job.created_at).toLocaleString()}
                                    </Td>
                                    <Td className="text-right" onClick={e => e.stopPropagation()}>
                                        {(job.status === 'queued' || job.status === 'running') && (
                                            <Button
                                                variant="danger"
                                                size="sm"
                                                onClick={e => {
                                                    e.stopPropagation()
                                                    const req = isAdmin
                                                        ? api.ai.adminCancelJob(job.id)
                                                        : api.ai.cancelJob(job.id)
                                                    req.then(() => load()).catch(() => { })
                                                }}
                                            >
                                                Cancel
                                            </Button>
                                        )}
                                        {(job.status === 'failed' || job.status === 'validation_failed' || job.status === 'cancelled') && (
                                            <Button
                                                variant="secondary"
                                                size="sm"
                                                onClick={e => {
                                                    e.stopPropagation()
                                                    api.ai.retryJob(job.id).then(() => load()).catch(() => { })
                                                }}
                                            >
                                                Retry
                                            </Button>
                                        )}
                                    </Td>
                                </tr>
                            ))}
                        </tbody>
                    </Table>
                </TableShell>
            )}

            {/* Pagination */}
            <nav className="mt-4 flex items-center justify-between text-xs text-muted">
                <span className="tnum">
                    {total === 0
                        ? 'No results'
                        : `${offset + 1}–${Math.min(offset + limit, total)} of ${total.toLocaleString()}`}
                </span>
                <div className="flex gap-2">
                    <Button
                        variant="secondary"
                        size="sm"
                        disabled={offset === 0}
                        onClick={() => setOffset(Math.max(0, offset - limit))}
                    >
                        Previous
                    </Button>
                    <Button
                        variant="secondary"
                        size="sm"
                        disabled={offset + limit >= total}
                        onClick={() => setOffset(offset + limit)}
                    >
                        Next
                    </Button>
                </div>
            </nav>
        </div>
    )
}
