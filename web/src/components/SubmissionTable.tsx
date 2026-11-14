import { Link } from 'react-router-dom'
import { RefreshCw } from 'lucide-react'
import { languageName } from '../lib/languages'
import { useLanguages } from '../lib/useLanguages'
import { Button } from './ui/Button'
import { Table, TableShell, Td, Th, THead, Tr } from './ui/Table'
import { VerdictBadge } from './ui/VerdictBadge'
import { isPendingVerdict, isSkippedVerdict } from './ui/verdict'

/**
 * One rendering of a submission table, shared by the site-wide status page and
 * both organizer tabs (a problem's and a round's).
 *
 * The row type is structural rather than `Submission` so the contest listing,
 * which is judge-aware and returns a narrower row, renders through the same
 * table without a cast. Everything the table draws is named here.
 */
export interface SubmissionRowView {
    id?: string
    created_at: string
    status: string
    username?: string
    user_id?: string
    contest_id?: string
    language?: string
    problem_id?: string
    problem_slug?: string
    problem_title?: string
    problem_index?: string
    time_used?: number
    memory_used?: number
    judge_error?: string
    /**
     * Per-viewer capabilities, resolved by the server from the same
     * accountability rule the action endpoints enforce. The table offers an
     * action only where the flag says the API would accept it, so a row can
     * never show a button that comes back 403.
     */
    can_view?: boolean
    can_rejudge?: boolean
    can_skip?: boolean
}

function formatTime(ms?: number): string {
    if (!ms || ms <= 0) return '—'
    return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`
}

function formatMemory(kb?: number): string {
    if (!kb || kb <= 0) return '—'
    return kb >= 1024 ? `${(kb / 1024).toFixed(0)} MB` : `${kb} KB`
}

function formatWhen(iso?: string): string {
    if (!iso) return '—'
    return new Date(iso).toLocaleString(undefined, {
        month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
    })
}

export function SubmissionTable({
    rows,
    showProblem = false,
    showUser = false,
    onRejudge,
    onSkip,
    busyIds,
}: {
    rows: SubmissionRowView[]
    showProblem?: boolean
    showUser?: boolean
    /** Omit to hide the action column entirely. */
    onRejudge?: (row: SubmissionRowView) => void
    onSkip?: (row: SubmissionRowView) => void
    busyIds?: Set<string>
}) {
    // Resolve display names against the judge's own list, so a language an
    // admin renamed in the panel shows its current name here too.
    const { languages } = useLanguages()
    const hasActions = !!(onRejudge || onSkip)
    // The inline failure row below a System Error spans every column.
    const columns = 5 + (showUser ? 1 : 0) + (showProblem ? 1 : 0) + (hasActions ? 1 : 0)

    return (
        <TableShell>
            <Table>
                <THead>
                    <tr>
                        <Th className="w-24">ID</Th>
                        {showUser && <Th>Who</Th>}
                        {showProblem && <Th>Problem</Th>}
                        <Th>Language</Th>
                        <Th className="w-40">Verdict</Th>
                        <Th className="hidden text-right sm:table-cell">Time</Th>
                        <Th className="hidden text-right md:table-cell">Memory</Th>
                        <Th className="hidden w-40 text-right lg:table-cell">When</Th>
                        {hasActions && <Th className="w-40 text-right">Actions</Th>}
                    </tr>
                </THead>
                <tbody>
                    {rows.flatMap(row => {
                        const pending = isPendingVerdict(row.status)
                        // A voided submission has no verdict to recompute: the
                        // judge refuses to overwrite it, so offering either
                        // action would only fail. The store search skips it too,
                        // so a bulk rejudge would leave it alone either way.
                        const voided = isSkippedVerdict(row.status)
                        const id = row.id || ''
                        const short = id.slice(0, 8)
                        const result = (
                            <Tr key={id}>
                                <Td>
                                    {row.can_view !== false && id ? (
                                        <Link
                                            to={`/submissions/${id}`}
                                            className="tnum font-mono text-xs text-brand hover:underline"
                                        >
                                            {short}
                                        </Link>
                                    ) : (
                                        <span
                                            className="tnum font-mono text-xs text-muted-soft"
                                            title="Source code is private"
                                        >
                                            {short || '—'}
                                        </span>
                                    )}
                                </Td>
                                {showUser && (
                                    <Td className="text-xs">
                                        {row.username ? (
                                            <Link to={`/user/${row.username}`} className="text-brand hover:underline">
                                                {row.username}
                                            </Link>
                                        ) : (
                                            <span className="font-mono text-muted-soft">{row.user_id?.slice(0, 8)}</span>
                                        )}
                                    </Td>
                                )}
                                {showProblem && (
                                    <Td className="text-xs">
                                        {/* The round's letter is how a problem is identified
                                            inside a round; a uuid is unreadable at the
                                            desk. Fall back to the problem's own identity
                                            when the row has no letter. */}
                                        {row.problem_index ? (
                                            <span className="font-mono text-xs" title={row.problem_title || row.problem_slug}>
                                                {row.problem_index}
                                            </span>
                                        ) : row.problem_slug ? (
                                            <Link to={`/problems/${row.problem_slug}`} className="text-brand hover:underline">
                                                {row.problem_title || row.problem_slug}
                                            </Link>
                                        ) : (
                                            <span className="font-mono text-muted-soft">{row.problem_id?.slice(0, 8)}</span>
                                        )}
                                    </Td>
                                )}
                                <Td className="text-xs text-ink-soft">{languageName(row.language || '', languages)}</Td>
                                <Td>
                                    <VerdictBadge status={row.status} />
                                </Td>
                                <Td className="tnum hidden text-right text-xs text-muted sm:table-cell">
                                    {pending || voided || row.status === 'ce' ? '—' : formatTime(row.time_used)}
                                </Td>
                                <Td className="tnum hidden text-right text-xs text-muted md:table-cell">
                                    {pending || voided || row.status === 'ce' ? '—' : formatMemory(row.memory_used)}
                                </Td>
                                <Td className="hidden text-right text-xs text-muted-soft lg:table-cell">
                                    {formatWhen(row.created_at)}
                                </Td>
                                {hasActions && (
                                    <Td className="text-right">
                                        {voided ? (
                                            <span className="text-xs text-muted-soft">Voided</span>
                                        ) : pending ? (
                                            <span className="text-xs text-muted-soft">Judging…</span>
                                        ) : (
                                            <span className="inline-flex items-center gap-3">
                                                {onRejudge && row.can_rejudge && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        disabled={busyIds?.has(id)}
                                                        onClick={() => onRejudge(row)}
                                                        title="Rejudge this submission"
                                                    >
                                                        <RefreshCw className="h-3.5 w-3.5" />
                                                        <span className="sr-only">Rejudge {short}</span>
                                                    </Button>
                                                )}
                                                {onSkip && row.can_skip && (
                                                    <button
                                                        type="button"
                                                        onClick={() => onSkip(row)}
                                                        className="text-xs text-warn hover:underline"
                                                        title="Void this submission"
                                                    >
                                                        Void
                                                    </button>
                                                )}
                                            </span>
                                        )}
                                    </Td>
                                )}
                            </Tr>
                        )
                        // A system error is worth reading where it happened: a
                        // setter scanning the list should not have to open every
                        // failed row to see the same crash text.
                        if (row.status !== 'se' || !row.judge_error) return [result]
                        return [result, (
                            <tr key={`${id}-error`} className="border-b border-hairline last:border-0 bg-bad/5">
                                <td colSpan={columns} className="px-4 py-2">
                                    <pre className="overflow-x-auto font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-bad">
                                        {row.judge_error}
                                    </pre>
                                </td>
                            </tr>
                        )]
                    })}
                </tbody>
            </Table>
        </TableShell>
    )
}

export default SubmissionTable
