import { useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { api } from '../lib/api'
import { errorMessage } from '../lib/errors'
import { useConfirm } from './ConfirmDialog'
import { useToast } from './Toast'
import { SkipSubmissionDialog } from './SkipSubmissionDialog'
import { SubmissionTable } from './SubmissionTable'
import type { SubmissionRowView } from './SubmissionTable'
import { Button } from './ui/Button'
import { isPendingVerdict, isSkippedVerdict } from './ui/verdict'

/**
 * The one place a submission is rejudged or voided.
 *
 * Both organizer tabs mount this — the problem's setter view and the round's
 * view — over rows each fetches its own way. What is shared is the behaviour:
 * the confirmations, the void dialog, the busy state, and the endpoint called.
 * Previously each tab had its own subset (the problem's could rejudge, the
 * round's could void), which is exactly how a submission came to be actionable
 * in one place and not the other.
 *
 * Which action is offered on which row is the server's call, carried on the row
 * as `can_rejudge`/`can_skip`; this component never decides that for itself.
 */
export function SubmissionPanel({
    rows,
    onChanged,
    rejudgeAllBody,
    rejudgeAllHint,
}: {
    rows: SubmissionRowView[]
    /** Called after an action lands, so the caller can refetch its page. */
    onChanged: () => void
    /**
     * The scope a batch rejudge covers, or null to hide the batch action. The
     * rows are what the caller fetched; this names the same set server-side.
     */
    rejudgeAllBody: { problem_id?: string; contest_id?: string } | null
    /** What the batch confirmation calls the set, e.g. "this problem". */
    rejudgeAllHint: string
}) {
    const confirm = useConfirm()
    const toast = useToast()
    const [busyIds, setBusyIds] = useState<Set<string>>(new Set())
    const [bulkBusy, setBulkBusy] = useState(false)
    // The row awaiting a reason, and the in-flight state of the void.
    const [voiding, setVoiding] = useState<SubmissionRowView | null>(null)
    const [voidBusy, setVoidBusy] = useState(false)

    const rejudgeOne = async (row: SubmissionRowView) => {
        if (!row.id) return
        const ok = await confirm({
            title: 'Rejudge submission',
            message: `Rejudge ${row.username || 'this user'}'s submission ${row.id.slice(0, 8)}? The previous verdict is discarded.`,
            confirmLabel: 'Rejudge',
        })
        if (!ok) return
        setBusyIds(prev => new Set(prev).add(row.id as string))
        try {
            await api.submissions.rejudge({ submission_id: row.id })
            toast.success('Rejudge queued')
            onChanged()
        } catch (err) {
            toast.error(errorMessage(err) || 'Rejudge failed')
        } finally {
            setBusyIds(prev => {
                const next = new Set(prev)
                next.delete(row.id as string)
                return next
            })
        }
    }

    const confirmVoid = async (reason: string) => {
        if (!voiding?.id) return
        setVoidBusy(true)
        try {
            await api.submissions.skip(voiding.id, reason)
            toast.success('Submission voided')
            setVoiding(null)
            onChanged()
        } catch (err) {
            toast.error(errorMessage(err) || 'Could not void the submission')
        } finally {
            setVoidBusy(false)
        }
    }

    const rejudgeAll = async () => {
        if (!rejudgeAllBody) return
        const ok = await confirm({
            title: 'Rejudge all submissions',
            message: `Rejudge every judged submission of ${rejudgeAllHint}, ignoring the filters above? Use this after fixing test data; it queues at background priority.`,
            confirmLabel: 'Rejudge all',
            variant: 'danger',
        })
        if (!ok) return
        setBulkBusy(true)
        try {
            const res = await api.submissions.rejudge(rejudgeAllBody)
            toast.success(`Queued ${res.enqueued.toLocaleString()} submission${res.enqueued === 1 ? '' : 's'}`)
            onChanged()
        } catch (err) {
            toast.error(errorMessage(err) || 'Rejudge failed')
        } finally {
            setBulkBusy(false)
        }
    }

    // Anything to void or rejudge that the judge is not currently holding.
    const actionable = rows.filter(r =>
        (r.can_rejudge || r.can_skip) && !isPendingVerdict(r.status) && !isSkippedVerdict(r.status))

    return (
        <div className="space-y-4">
            {(rejudgeAllBody || actionable.length > 0) && (
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-xs text-muted-soft">
                        {actionable.length > 0
                            ? `${actionable.length} of ${rows.length} row${rows.length === 1 ? '' : 's'} can be voided or rejudged. Voiding removes the attempt from the round; the verdict the code earned is kept on the record.`
                            : 'The judge is still working through these rows.'}
                    </p>
                    {rejudgeAllBody && (
                        <Button
                            variant="secondary"
                            size="sm"
                            onClick={rejudgeAll}
                            disabled={bulkBusy || rows.length === 0}
                        >
                            <RefreshCw className={`h-3.5 w-3.5 ${bulkBusy ? 'animate-spin' : ''}`} />
                            Rejudge all
                        </Button>
                    )}
                </div>
            )}

            <SubmissionTable
                rows={rows}
                showUser
                showProblem={!rejudgeAllBody?.problem_id}
                onRejudge={rejudgeOne}
                onSkip={setVoiding}
                busyIds={busyIds}
            />

            <SkipSubmissionDialog
                // Keyed per row so the reason field resets when a different
                // submission is picked, without an effect.
                key={voiding?.id ?? 'none'}
                open={voiding !== null}
                was={voiding?.status}
                username={voiding?.username}
                busy={voidBusy}
                onCancel={() => setVoiding(null)}
                onConfirm={confirmVoid}
            />
        </div>
    )
}

export default SubmissionPanel
