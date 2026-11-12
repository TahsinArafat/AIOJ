import { Link } from 'react-router-dom'
import { RefreshCw } from 'lucide-react'
import { languageName } from '../lib/languages'
import { useLanguages } from '../lib/useLanguages'
import { Button } from './ui/Button'
import { Table, TableShell, Td, Th, THead, Tr } from './ui/Table'
import { VerdictBadge } from './ui/VerdictBadge'
import { isPendingVerdict } from './ui/verdict'
import type { Submission } from '../types/api'

/**
 * One rendering of the submission table, shared by the site-wide status page
 * and a problem's setter view.
 *
 * `can_view` decides link vs. plain text: the submission API returns source
 * code to its owner, admins and the problem's setters only, so linking a row
 * the viewer cannot open would land them on a 403.
 */

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
    busyIds,
}: {
    rows: Submission[]
    showProblem?: boolean
    showUser?: boolean
    /** Omit to hide the action column entirely. */
    onRejudge?: (row: Submission) => void
    busyIds?: Set<string>
}) {
    // Resolve display names against the judge's own list, so a language an
    // admin renamed in the panel shows its current name here too.
    const { languages } = useLanguages()
    // The inline failure row below a System Error spans every column.
    const columns = 5 + (showUser ? 1 : 0) + (showProblem ? 1 : 0) + (onRejudge ? 1 : 0)

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
                        {onRejudge && <Th className="w-24 text-right">Actions</Th>}
                    </tr>
                </THead>
                <tbody>
                    {rows.flatMap(row => {
                        const pending = isPendingVerdict(row.status)
                        const result = (
                            <Tr key={row.id}>
                                <Td>
                                    {row.can_view ? (
                                        <Link
                                            to={`/submissions/${row.id}`}
                                            className="tnum font-mono text-xs text-brand hover:underline"
                                        >
                                            {row.id.slice(0, 8)}
                                        </Link>
                                    ) : (
                                        <span
                                            className="tnum font-mono text-xs text-muted-soft"
                                            title="Source code is private"
                                        >
                                            {row.id.slice(0, 8)}
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
                                        {row.problem_slug ? (
                                            <Link to={`/problems/${row.problem_slug}`} className="text-brand hover:underline">
                                                {row.problem_title || row.problem_slug}
                                            </Link>
                                        ) : (
                                            <span className="font-mono text-muted-soft">{row.problem_id?.slice(0, 8)}</span>
                                        )}
                                    </Td>
                                )}
                                <Td className="text-xs text-ink-soft">{languageName(row.language, languages)}</Td>
                                <Td>
                                    <VerdictBadge status={row.status} />
                                </Td>
                                <Td className="tnum hidden text-right text-xs text-muted sm:table-cell">
                                    {pending || row.status === 'ce' ? '—' : formatTime(row.time_used)}
                                </Td>
                                <Td className="tnum hidden text-right text-xs text-muted md:table-cell">
                                    {pending || row.status === 'ce' ? '—' : formatMemory(row.memory_used)}
                                </Td>
                                <Td className="hidden text-right text-xs text-muted-soft lg:table-cell">
                                    {formatWhen(row.created_at)}
                                </Td>
                                {onRejudge && (
                                    <Td className="text-right">
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            disabled={pending || busyIds?.has(row.id)}
                                            onClick={() => onRejudge(row)}
                                            title={pending ? 'Already queued for judging' : 'Rejudge this submission'}
                                        >
                                            <RefreshCw className="h-3.5 w-3.5" />
                                            <span className="sr-only">Rejudge {row.id.slice(0, 8)}</span>
                                        </Button>
                                    </Td>
                                )}
                            </Tr>
                        )
                        // A system error is worth reading where it happened: a
                        // setter scanning the list should not have to open every
                        // failed row to see the same crash text.
                        if (row.status !== 'se' || !row.judge_error) return [result]
                        return [result, (
                            <tr key={`${row.id}-error`} className="border-b border-hairline last:border-0 bg-bad/5">
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
