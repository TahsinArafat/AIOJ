import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { errorMessage } from '../lib/errors'
import { useDocumentTitle } from '../lib/documentTitle'
import { useLanguages } from '../lib/useLanguages'
import { EmptyState } from '../components/EmptyState'
import { SubmissionTable } from '../components/SubmissionTable'
import { Button } from '../components/ui/Button'
import { Field, Input, Select } from '../components/ui/Field'
import { PageHeader, Skeleton } from '../components/ui/PageHeader'
import { isPendingVerdict, verdictMeta } from '../components/ui/verdict'
import type { Submission } from '../types/api'

/**
 * The site-wide submission log ("status" page).
 *
 * Rows the viewer may not open come back without `can_view`, so the table
 * renders their id as plain text instead of a link into a 403.
 */

const PAGE_SIZE = 50

// Statuses the API matches exactly. 'skipped' is one of them, though no judge
// produces it: an organizer sets it by voiding a submission.
const VERDICTS = ['ac', 'wa', 'tle', 'mle', 're', 'ce', 'se', 'skipped', 'pending', 'judging']

interface Filters {
    handle: string
    problem: string
    status: string
    language: string
}

const EMPTY_FILTERS: Filters = { handle: '', problem: '', status: '', language: '' }

export default function AllSubmissions() {
    useDocumentTitle('Status')
    // The judge's own languages, so the filter matches what can actually be submitted.
    const { options: languageOptions } = useLanguages()
    const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS)
    const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
    const [rows, setRows] = useState<Submission[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    // Bumped by the poll timer to re-run the fetch effect.
    const [pollTick, setPollTick] = useState(0)

    useEffect(() => {
        let cancelled = false
        api.submissions.listAll({
            offset,
            limit: PAGE_SIZE,
            handle: applied.handle || undefined,
            problem: applied.problem || undefined,
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
    }, [offset, applied, pollTick])

    // A verdict that is still in flight will change on its own; refresh until
    // the page has no pending rows.
    const hasPending = rows.some(r => isPendingVerdict(r.status))
    useEffect(() => {
        if (!hasPending) return
        const timer = setTimeout(() => setPollTick(n => n + 1), 3000)
        return () => clearTimeout(timer)
    }, [hasPending, pollTick])

    const apply = (e: React.FormEvent) => {
        e.preventDefault()
        setOffset(0)
        setApplied(filters)
    }

    const reset = () => {
        setFilters(EMPTY_FILTERS)
        setApplied(EMPTY_FILTERS)
        setOffset(0)
    }

    return (
        <div>
            <PageHeader
                title="Status"
                description="Every submission on the platform. Open a row to read its source if you have access to it."
            />

            <form
                onSubmit={apply}
                className="mb-4 grid grid-cols-1 gap-3 rounded-xl border border-hairline bg-surface p-4 sm:grid-cols-2 lg:grid-cols-5"
            >
                <Field label="Handle" htmlFor="filter-handle">
                    <Input
                        id="filter-handle"
                        placeholder="username"
                        value={filters.handle}
                        onChange={e => setFilters(f => ({ ...f, handle: e.target.value }))}
                    />
                </Field>
                <Field label="Problem" htmlFor="filter-problem">
                    <Input
                        id="filter-problem"
                        placeholder="problem-slug"
                        value={filters.problem}
                        onChange={e => setFilters(f => ({ ...f, problem: e.target.value }))}
                    />
                </Field>
                <Field label="Verdict" htmlFor="filter-status">
                    <Select
                        id="filter-status"
                        value={filters.status}
                        onChange={e => setFilters(f => ({ ...f, status: e.target.value }))}
                    >
                        <option value="">Any verdict</option>
                        {VERDICTS.map(v => (
                            <option key={v} value={v}>{verdictMeta(v).label}</option>
                        ))}
                    </Select>
                </Field>
                <Field label="Language" htmlFor="filter-language">
                    <Select
                        id="filter-language"
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
                    <Button type="button" variant="ghost" size="sm" onClick={reset}>Reset</Button>
                </div>
            </form>

            {error && (
                <div role="alert" className="mb-4 rounded-lg border border-bad/30 bg-bad/10 p-3 text-sm text-bad">
                    {error}
                </div>
            )}

            {loading && rows.length === 0 ? (
                <div className="space-y-2">
                    {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-11 w-full" />)}
                </div>
            ) : rows.length === 0 ? (
                <div className="rounded-xl border border-hairline bg-surface">
                    <EmptyState
                        text="No submissions match"
                        description="Try a different handle, problem or verdict."
                    />
                </div>
            ) : (
                <SubmissionTable rows={rows} showProblem showUser />
            )}

            <nav aria-label="Pagination" className="mt-4 flex items-center justify-between text-xs text-muted">
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
