import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { languageName } from '../lib/languages'
import { EmptyState } from '../components/EmptyState'
import { Button } from '../components/ui/Button'
import { PageHeader, Skeleton } from '../components/ui/PageHeader'
import { Table, TableShell, Td, Th, THead, Tr } from '../components/ui/Table'
import { VerdictBadge } from '../components/ui/VerdictBadge'
import { isPendingVerdict } from '../components/ui/verdict'

interface SubRow {
    id: string
    language?: string
    status: string
    time_used?: number
    memory_used: number
    created_at?: string
}

/** Machine language keys (`cpp-gpp-64`) are not user-facing names. */
function formatTime(ms?: number): string {
    if (!ms || ms <= 0) return '—'
    return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`
}

function formatMemory(kb: number): string {
    if (!kb || kb <= 0) return '—'
    return kb >= 1024 ? `${(kb / 1024).toFixed(0)} MB` : `${kb} KB`
}

export default function Submissions() {
    const [subs, setSubs] = useState<SubRow[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const [loading, setLoading] = useState(true)
    const limit = 20

    useEffect(() => {
        let cancelled = false
        api.submissions.list(offset, limit).then(d => {
            if (cancelled) return
            setSubs(d.data || [])
            setTotal(d.total || 0)
        }).catch(() => {
            if (!cancelled) setSubs([])
        }).finally(() => {
            if (!cancelled) setLoading(false)
        })
        return () => { cancelled = true }
    }, [offset])

    return (
        <div>
            <PageHeader
                title="My Submissions"
                description={total > 0 ? `${total.toLocaleString()} submissions on record` : 'Your judged solutions'}
            />

            {loading && subs.length === 0 ? (
                <div className="space-y-2">
                    {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-11 w-full" />)}
                </div>
            ) : subs.length === 0 ? (
                <div className="rounded-xl border border-hairline bg-surface">
                    <EmptyState
                        text="No submissions yet"
                        description="Submissions appear here as soon as solutions are judged."
                    />
                </div>
            ) : (
                <TableShell>
                    <Table>
                        <THead>
                            <tr>
                                <Th className="w-24">ID</Th>
                                <Th>Language</Th>
                                <Th className="w-44">Verdict</Th>
                                <Th className="hidden text-right sm:table-cell">Time</Th>
                                <Th className="hidden text-right md:table-cell">Memory</Th>
                                <Th className="hidden w-24 text-right lg:table-cell">Submitted</Th>
                            </tr>
                        </THead>
                        <tbody>
                            {subs.map(s => {
                                const pending = isPendingVerdict(s.status)
                                return (
                                    <Tr key={s.id}>
                                        <Td>
                                            <Link
                                                to={`/submissions/${s.id}`}
                                                className="tnum font-mono text-xs text-brand hover:underline"
                                            >
                                                {s.id?.slice(0, 8)}
                                            </Link>
                                        </Td>
                                        <Td className="text-xs text-ink-soft">
                                            {languageName(s.language)}
                                        </Td>
                                        <Td>
                                            <VerdictBadge status={s.status} />
                                        </Td>
                                        <Td className="tnum hidden text-right text-xs text-muted sm:table-cell">
                                            {pending || s.status === 'ce' ? '—' : formatTime(s.time_used)}
                                        </Td>
                                        <Td className="tnum hidden text-right text-xs text-muted md:table-cell">
                                            {pending || s.status === 'ce' ? '—' : formatMemory(s.memory_used)}
                                        </Td>
                                        <Td className="hidden text-right text-xs text-muted-soft lg:table-cell">
                                            {s.created_at ? new Date(s.created_at).toLocaleDateString() : '—'}
                                        </Td>
                                    </Tr>
                                )
                            })}
                        </tbody>
                    </Table>
                </TableShell>
            )}

            <nav aria-label="Pagination" className="mt-4 flex items-center justify-between text-xs text-muted">
                <span className="tnum">
                    {total === 0 ? 'No results' : `${offset + 1}–${Math.min(offset + limit, total)} of ${total.toLocaleString()}`}
                </span>
                <div className="flex gap-2">
                    <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setOffset(Math.max(0, offset - limit))}
                        disabled={offset === 0}
                    >
                        Previous
                    </Button>
                    <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setOffset(offset + limit)}
                        disabled={offset + limit >= total}
                    >
                        Next
                    </Button>
                </div>
            </nav>
        </div>
    )
}
