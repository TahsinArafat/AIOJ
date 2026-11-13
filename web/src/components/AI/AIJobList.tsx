import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { api } from '../../lib/api'
import type { AIGenerationJob } from '../../types/api'
import { JOB_STATUSES, canRetryJob, elapsed, isJobActive, statusLabel, totalTokens } from '../../lib/aiJobs'
import { errorMessage } from '../../lib/errors'
import { Button } from '../ui/Button'
import { Skeleton } from '../ui/PageHeader'
import { Table, TableShell, Td, Th, THead } from '../ui/Table'
import { EmptyState } from '../EmptyState'
import AIJobStatusBadge from './AIJobStatusBadge'

type Props = {
    /** Admin mode lists every setter's jobs and may force-cancel any of them. */
    isAdmin: boolean
    /**
     * Which job to render as open, or null for none. Controlled by the parent so
     * the selected job and the list are one piece of state rather than two that
     * can disagree.
     */
    selectedId: string | null
    onSelect: (id: string | null) => void
    /** Bumped by the parent to force a refetch after an action elsewhere. */
    refreshKey?: number
    /** Renders the open job. Given the id so the parent owns what "open" means. */
    renderDetail?: (id: string) => React.ReactNode
}

/**
 * The list of generation jobs, shared by AI Studio and the admin tab.
 *
 * Both previously had their own: AIJobsPage served the admin tab and a standalone
 * route, while AI Studio fetched the same endpoint a second time just to tell the
 * operator a slot was occupied. One component means one poll and one place where
 * "is this job still going" is decided.
 */
export default function AIJobList({ isAdmin, selectedId, onSelect, refreshKey = 0, renderDetail }: Props) {
    const [jobs, setJobs] = useState<AIGenerationJob[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [filterStatus, setFilterStatus] = useState('')
    const [actorFilter, setActorFilter] = useState('')
    const limit = 50

    const load = () => {
        setLoading(true)
        setError('')
        const req = isAdmin
            ? api.ai.adminListJobs({ offset, limit, status: filterStatus || undefined, actor_id: actorFilter || undefined })
            : api.ai.listJobs(offset, limit)
        req.then((r) => {
            setJobs(r.data || [])
            setTotal(r.total || 0)
        }).catch((e) => setError(errorMessage(e))).finally(() => setLoading(false))
    }

    // deferred a microtask: the spinner lands pre-paint (react-hooks/set-state-in-effect)
    // eslint-disable-next-line react-hooks/exhaustive-deps
    useEffect(() => { queueMicrotask(load) }, [offset, filterStatus, actorFilter, refreshKey, isAdmin])

    // Poll only while something is actually in flight. A settled list is a static
    // table, and polling it forever was how the admin tab kept a request open on a
    // page nobody was watching.
    const hasActive = jobs.some(isJobActive)
    useEffect(() => {
        if (!hasActive) return
        const id = window.setInterval(load, 3000)
        return () => window.clearInterval(id)
    }, [hasActive]) // eslint-disable-line react-hooks/exhaustive-deps

    const detail = selectedId && renderDetail ? renderDetail(selectedId) : null

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-end gap-3">
                <label className="text-sm text-muted-soft">
                    Status
                    <select
                        value={filterStatus}
                        onChange={(e) => { setFilterStatus(e.target.value); setOffset(0) }}
                        className="ml-2 rounded border border-hairline-strong bg-surface px-2 py-1 text-xs text-ink"
                    >
                        <option value="">All statuses</option>
                        {JOB_STATUSES.map((s) => (
                            <option key={s} value={s}>{statusLabel(s)}</option>
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
                            onChange={(e) => { setActorFilter(e.target.value); setOffset(0) }}
                            className="ml-2 w-72 rounded border border-hairline-strong bg-surface px-2 py-1 text-xs text-ink"
                        />
                    </label>
                )}
                <Button variant="secondary" size="sm" onClick={load}>
                    <RefreshCw className="h-3.5 w-3.5" />
                    Refresh
                </Button>
            </div>

            {error && (
                <div className="rounded border border-bad/40 bg-bad/10 p-3 text-sm text-bad">{error}</div>
            )}

            {loading && jobs.length === 0 ? (
                <div className="space-y-2">
                    {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
                </div>
            ) : jobs.length === 0 ? (
                <div className="rounded-xl border border-hairline bg-surface">
                    <EmptyState
                        text="No AI generation jobs"
                        description="Jobs appear here once a setter starts one from the panel above or from a problem's AI Studio tab."
                    />
                </div>
            ) : (
                <TableShell>
                    <Table>
                        <THead>
                            <tr>
                                <Th>Job</Th>
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
                            {jobs.map((job) => {
                                const open = selectedId === job.id
                                return (
                                    <tr
                                        key={job.id}
                                        className={`cursor-pointer border-b border-hairline last:border-0 transition-colors ${open ? 'bg-surface-2' : 'hover:bg-surface-2'}`}
                                        onClick={() => onSelect(open ? null : job.id)}
                                    >
                                        <Td>
                                            <span className="font-mono text-xs text-brand">{job.id.slice(0, 8)}</span>
                                            {job.problem_id && <span className="ml-1 text-xs text-muted-soft">· problem</span>}
                                        </Td>
                                        {isAdmin && (
                                            <Td><span className="font-mono text-xs text-muted">{job.actor_id.slice(0, 8)}</span></Td>
                                        )}
                                        <Td><span className="text-xs text-ink-soft">{statusLabel(job.artifact_type)}</span></Td>
                                        <Td><AIJobStatusBadge status={job.status} /></Td>
                                        <Td>
                                            <div className="flex items-center gap-2">
                                                <div className="h-1.5 w-16 overflow-hidden rounded-full bg-surface-3">
                                                    <div
                                                        className={`h-1.5 rounded-full ${canRetryJob(job) ? 'bg-bad' : 'bg-brand'}`}
                                                        style={{ width: `${job.progress}%` }}
                                                    />
                                                </div>
                                                <span className="tnum text-xs text-muted">{job.progress}%</span>
                                            </div>
                                        </Td>
                                        <Td className="hidden tnum text-xs text-muted sm:table-cell">{totalTokens(job)}</Td>
                                        <Td className="hidden tnum text-xs text-muted md:table-cell">{elapsed(job)}</Td>
                                        <Td className="hidden text-xs text-muted-soft lg:table-cell">
                                            {new Date(job.created_at).toLocaleString()}
                                        </Td>
                                        <Td className="text-right" onClick={(e) => e.stopPropagation()}>
                                            <div className="flex justify-end gap-2">
                                                {isJobActive(job) && (
                                                    <Button
                                                        variant="danger"
                                                        size="sm"
                                                        onClick={() => {
                                                            const req = isAdmin ? api.ai.adminCancelJob(job.id) : api.ai.cancelJob(job.id)
                                                            req.then(load).catch((e) => setError(errorMessage(e)))
                                                        }}
                                                    >
                                                        Cancel
                                                    </Button>
                                                )}
                                                {canRetryJob(job) && (
                                                    <Button
                                                        variant="secondary"
                                                        size="sm"
                                                        onClick={() => api.ai.retryJob(job.id).then(load).catch((e) => setError(errorMessage(e)))}
                                                    >
                                                        Retry
                                                    </Button>
                                                )}
                                            </div>
                                        </Td>
                                    </tr>
                                )
                            })}
                        </tbody>
                    </Table>
                </TableShell>
            )}

            {detail}

            <nav className="flex items-center justify-between text-xs text-muted">
                <span className="tnum">
                    {total === 0 ? 'No results' : `${offset + 1}–${Math.min(offset + limit, total)} of ${total.toLocaleString()}`}
                </span>
                <div className="flex gap-2">
                    <Button variant="secondary" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>
                        Previous
                    </Button>
                    <Button variant="secondary" size="sm" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>
                        Next
                    </Button>
                </div>
            </nav>
        </div>
    )
}
