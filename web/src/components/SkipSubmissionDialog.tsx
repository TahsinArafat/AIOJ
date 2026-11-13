import { useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Button } from './ui/Button'
import { Field, Input } from './ui/Field'

/**
 * The reason prompt for voiding a submission.
 *
 * The server refuses a skip with no reason, and it is right to: a void nobody
 * can explain later is worse than the submission it removed. `window.prompt`
 * would be one line, but it cannot be styled, cannot be focused properly and
 * cannot show what is being voided — and this is a destructive, attributable
 * action on someone's contest result, so it deserves a real dialog.
 *
 * The caller passes a `key` per target row, so switching rows remounts this and
 * the reason field is empty again. That is deliberate: resetting the field in an
 * effect would be a cascading render, and would still fire when the dialog is
 * closed.
 */
export function SkipSubmissionDialog({
    open,
    // What the row currently is, so the dialog can say what is being voided
    // rather than asking the organizer to remember which row they clicked.
    was,
    username,
    busy,
    onCancel,
    onConfirm,
}: {
    open: boolean
    was?: string
    username?: string
    busy?: boolean
    onCancel: () => void
    onConfirm: (reason: string) => void
}) {
    const [reason, setReason] = useState('')

    if (!open) return null

    const trimmed = reason.trim()

    return (
        <div
            className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/50 p-4"
            role="dialog"
            aria-modal="true"
            aria-labelledby="skip-submission-title"
            onKeyDown={e => {
                if (e.key === 'Escape') onCancel()
            }}
        >
            <form
                className="relative w-full max-w-lg rounded-lg bg-surface p-6 shadow-lg"
                onSubmit={e => {
                    e.preventDefault()
                    if (trimmed) onConfirm(trimmed)
                }}
            >
                <div className="mb-4 flex items-center gap-2 border-b border-hairline pb-3">
                    <AlertTriangle className="h-5 w-5 text-warn" />
                    <h3 id="skip-submission-title" className="text-lg font-semibold text-ink">
                        Void this submission
                    </h3>
                </div>

                <p className="text-sm leading-relaxed text-muted">
                    {username ? <span className="font-medium text-ink-soft">{username}</span> : 'This submission'}
                    {was ? <> currently stands at <span className="font-mono uppercase text-ink-soft">{was}</span>.</> : '.'}
                    {' '}Voiding it removes the attempt from the round: it stops counting and stops scoring.
                    The verdict the code earned is kept on the record.
                </p>

                <div className="mt-4">
                    <Field
                        label="Reason"
                        hint="Required. Recorded with your name in the audit log, so the decision can be defended later."
                        htmlFor="skip-submission-reason"
                    >
                        <Input
                            id="skip-submission-reason"
                            // The dialog exists to collect this one field, and it
                            // is opened deliberately, so focusing it is safe.
                            autoFocus
                            value={reason}
                            maxLength={280}
                            placeholder="e.g. Resubmitted from a second account."
                            onChange={e => setReason(e.target.value)}
                        />
                    </Field>
                </div>

                <div className="mt-5 flex justify-end gap-2">
                    <Button type="button" variant="secondary" size="sm" onClick={onCancel} disabled={busy}>
                        Cancel
                    </Button>
                    <Button type="submit" size="sm" disabled={!trimmed || busy}>
                        {busy ? 'Voiding…' : 'Void submission'}
                    </Button>
                </div>
            </form>
        </div>
    )
}

export default SkipSubmissionDialog
