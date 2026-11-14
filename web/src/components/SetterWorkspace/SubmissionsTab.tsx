import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { useLanguages } from '../../lib/useLanguages'
import { EmptyState } from '../EmptyState'
import { SubmissionPanel } from '../SubmissionPanel'
import { Button } from '../ui/Button'
import { Field, Input, Select } from '../ui/Field'
import { Skeleton } from '../ui/PageHeader'
import { isPendingVerdict, verdictMeta } from '../ui/verdict'
import type { Submission } from '../../types/api'

/**
 * A problem's submissions, from the setter's side.
 *
 * This is the surface the authoring gate exists for: the setter sees who
 * submitted what, can open any row's source, and can void or rejudge a row (or
 * the whole problem) after fixing test data. The actions themselves live in
 * SubmissionPanel, which the round's tab mounts too.
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

export default function SubmissionsTab({ problemSlug, problemId }: { problemSlug: string; problemId: string }) {
    // The judge's own languages, so the filter offers what can actually be submitted.
    const { options: languageOptions } = useLanguages()
    const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS)
    const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
    const [rows, setRows] = useState<Submission[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    // Bumping this refetches: the poll below uses it, and so does the panel
    // after an action lands, so a void or a rejudge is reflected immediately.
    const [reloadTick, setReloadTick] = useState(0)

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
    }, [problemSlug, offset, applied, reloadTick])

    // Keep the table live while anything on the page is still judging.
    const hasPending = rows.some(r => isPendingVerdict(r.status))
    useEffect(() => {
        if (!hasPending) return
        const timer = setTimeout(() => setReloadTick(n => n + 1), 3000)
        return () => clearTimeout(timer)
    }, [hasPending, reloadTick])

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
                <SubmissionPanel
                    rows={rows}
                    onChanged={() => setReloadTick(n => n + 1)}
                    // A batch rejudge covers the whole problem, not just the page
                    // and not the filters — which is why the confirmation says so.
                    rejudgeAllBody={{ problem_id: problemId }}
                    rejudgeAllHint="this problem"
                />
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
