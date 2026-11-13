import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { useLanguages } from '../../lib/useLanguages'
import { useConfirm } from '../ConfirmDialog'
import { useToast } from '../Toast'
import { EmptyState } from '../EmptyState'
import { SubmissionTable } from '../SubmissionTable'
import { Button } from '../ui/Button'
import { Field, Input, Select } from '../ui/Field'
import { Skeleton } from '../ui/PageHeader'
import { isPendingVerdict, verdictMeta } from '../ui/verdict'
import type { Submission } from '../../types/api'

/**
 * A problem's submissions, from the setter's side.
 *
 * This is the surface the authoring gate exists for: the setter sees who
 * submitted what, can open any row's source, and can rejudge a single row or
 * the whole problem after fixing test data.
 */

const PAGE_SIZE = 50
// 'skipped' is included: a voided submission is a real stored status, and a
// setter needs to be able to find the ones they voided.
const VERDICTS = ['ac', 'wa', 'tle', 'mle', 're', 'ce', 'se', 'skipped', 'pending', 'judging']

interface Filters {
    handle: string
    status: string
    language: string
}

const EMPTY_FILTERS: Filters = { handle: '', status: '', language: '' }

export default function SubmissionsTab({ problemSlug }: { problemSlug: string }) {
    const confirm = useConfirm()
    const toast = useToast()
    // The judge's own languages, so the filter offers what can actually be submitted.
    const { options: languageOptions } = useLanguages()
    const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS)
    const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
    const [rows, setRows] = useState<Submission[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [busyIds, setBusyIds] = useState<Set<string>>(new Set())
    const [bulkBusy, setBulkBusy] = useState(false)
    const [pollTick, setPollTick] = useState(0)

    useEffect(() => {
        let cancelled = false
        api.problems.submissions(problemSlug, offset, PAGE_SIZE, {
            handle: applied.handle || undefined,
            status: applied.status || undefined,
            language: applied.language || undefined,
        }).then(d => {
            if (cancelled) return
            setRows(d.data || [])
            setTotal(d.total || 0)
            setError(null)
        }).catch(err => {
            if (cancelled) return
            setRows([])
            setTotal(0)
            setError(errorMessage(err) || 'Could not load submissions')
        }).finally(() => {
            if (!cancelled) setLoading(false)
        })
        return () => { cancelled = true }
    }, [problemSlug, offset, applied, pollTick])

    // Keep the table live while anything on the page is still judging.
    const hasPending = rows.some(r => isPendingVerdict(r.status))
    useEffect(() => {
        if (!hasPending) return
        const timer = setTimeout(() => setPollTick(n => n + 1), 3000)
        return () => clearTimeout(timer)
    }, [hasPending, pollTick])

    const rejudgeOne = async (row: Submission) => {
        const ok = await confirm({
            title: 'Rejudge submission',
            message: `Rejudge ${row.username || 'this user'}'s submission ${row.id.slice(0, 8)}? The previous verdict is discarded.`,
            confirmLabel: 'Rejudge',
        })
        if (!ok) return
        setBusyIds(prev => new Set(prev).add(row.id))
        try {
            await api.problems.rejudgeSubmissions(problemSlug, { submission_id: row.id })
            toast.success('Rejudge queued')
            setPollTick(n => n + 1)
        } catch (err) {
            toast.error(errorMessage(err) || 'Rejudge failed')
        } finally {
            setBusyIds(prev => {
                const next = new Set(prev)
                next.delete(row.id)
                return next
            })
        }
    }

    const rejudgeAll = async () => {
        const ok = await confirm({
            title: 'Rejudge all submissions',
            message: 'Rejudge every judged submission of this problem, ignoring the filters above? Use this after fixing test data; it queues at background priority.',
            confirmLabel: 'Rejudge all',
            variant: 'danger',
        })
        if (!ok) return
        setBulkBusy(true)
        try {
            const res = await api.problems.rejudgeSubmissions(problemSlug, {})
            toast.success(`Queued ${res.enqueued.toLocaleString()} submission${res.enqueued === 1 ? '' : 's'}`)
            setPollTick(n => n + 1)
        } catch (err) {
            toast.error(errorMessage(err) || 'Rejudge failed')
        } finally {
            setBulkBusy(false)
        }
    }

    const apply = (e: React.FormEvent) => {
        e.preventDefault()
        setOffset(0)
        setApplied(filters)
    }

    return (
        <div className="space-y-4">
            <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <h2 className="text-lg font-bold text-ink">Submissions</h2>
                    <p className="text-xs text-muted-soft">
                        {total > 0
                            ? `${total.toLocaleString()} submission${total === 1 ? '' : 's'} on this problem`
                            : 'No submissions yet'}
                    </p>
                </div>
                <Button
                    variant="secondary"
                    size="sm"
                    onClick={rejudgeAll}
                    disabled={bulkBusy || total === 0}
                >
                    <RefreshCw className={`h-3.5 w-3.5 ${bulkBusy ? 'animate-spin' : ''}`} />
                    Rejudge all
                </Button>
            </div>

            <form onSubmit={apply} className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <Field label="Handle" htmlFor="setter-filter-handle">
                    <Input
                        id="setter-filter-handle"
                        placeholder="username"
                        value={filters.handle}
                        onChange={e => setFilters(f => ({ ...f, handle: e.target.value }))}
                    />
                </Field>
                <Field label="Verdict" htmlFor="setter-filter-status">
                    <Select
                        id="setter-filter-status"
                        value={filters.status}
                        onChange={e => setFilters(f => ({ ...f, status: e.target.value }))}
                    >
                        <option value="">Any verdict</option>
                        {VERDICTS.map(v => (
                            <option key={v} value={v}>{verdictMeta(v).label}</option>
                        ))}
                    </Select>
                </Field>
                <Field label="Language" htmlFor="setter-filter-language">
                    <Select
                        id="setter-filter-language"
                        value={filters.language}
                        onChange={e => setFilters(f => ({ ...f, language: e.target.value }))}
                    >
                        <option value="">Any language</option>
                        {languageOptions.map(o => (
                            <option key={o.value} value={o.value}>{o.label}</option>
                        ))}
                    </Select>
                </Field>
                <div className="flex items-end gap-2">
                    <Button type="submit" size="sm">Filter</Button>
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => { setFilters(EMPTY_FILTERS); setApplied(EMPTY_FILTERS); setOffset(0) }}
                    >
                        Reset
                    </Button>
                </div>
            </form>

            {error && (
                <div role="alert" className="rounded-lg border border-bad/30 bg-bad/10 p-3 text-sm text-bad">
                    {error}
                </div>
            )}

            {loading && rows.length === 0 ? (
                <div className="space-y-2">
                    {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-11 w-full" />)}
                </div>
            ) : rows.length === 0 ? (
                <div className="rounded-xl border border-hairline bg-surface">
                    <EmptyState
                        text="No submissions yet"
                        description="Solutions appear here as soon as people submit to this problem."
                    />
                </div>
            ) : (
                <SubmissionTable rows={rows} showUser onRejudge={rejudgeOne} busyIds={busyIds} />
            )}

            <nav aria-label="Pagination" className="flex items-center justify-between text-xs text-muted">
                <span className="tnum">
                    {total === 0
                        ? 'No results'
                        : `${offset + 1}–${Math.min(offset + PAGE_SIZE, total)} of ${total.toLocaleString()}`}
                </span>
                <div className="flex gap-2">
                    <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                        disabled={offset === 0}
                    >
                        Previous
                    </Button>
                    <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setOffset(offset + PAGE_SIZE)}
                        disabled={offset + PAGE_SIZE >= total}
                    >
                        Next
                    </Button>
                </div>
            </nav>
        </div>
    )
}
