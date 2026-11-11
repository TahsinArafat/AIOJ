interface SettingsTabProps {
  /** Review state: draft | pending | published | rejected. */
  status: string
  /** Reviewer's note, shown back to the author after a rejection. */
  reviewNote?: string
  visible: boolean
  saving: boolean
  onSubmitForReview: () => void
  onDelete: () => void
}

const STATUS_COPY: Record<string, { label: string; tone: string; blurb: string }> = {
  draft: {
    label: 'Draft',
    tone: 'bg-surface-2 text-muted',
    blurb:
      'Only you and your collaborators can see this problem. Submit it for review when the statement and tests are ready.',
  },
  pending: {
    label: 'Awaiting review',
    tone: 'bg-warn/12 text-warn',
    blurb: 'An admin is reviewing this problem. It stays private until they publish it.',
  },
  published: {
    label: 'Published',
    tone: 'bg-ok/12 text-ok',
    blurb: 'This problem is in the public archive and visible to solvers.',
  },
  rejected: {
    label: 'Rejected',
    tone: 'bg-bad/12 text-bad',
    blurb: 'An admin sent this back. Fix what they noted, then resubmit.',
  },
}

export default function SettingsTab({
  status,
  reviewNote,
  visible,
  saving,
  onSubmitForReview,
  onDelete,
}: SettingsTabProps) {
  const copy = STATUS_COPY[status] ?? STATUS_COPY.draft
  const canSubmit = status === 'draft' || status === 'rejected'

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-bold text-ink border-b pb-2 mb-4">Workspace Settings</h2>

      <div className="space-y-3">
        <div className="flex items-center gap-3">
          <span className="text-sm font-medium text-ink-soft">Publishing</span>
          <span className={`px-2 py-1 rounded text-xs font-medium ${copy.tone}`}>{copy.label}</span>
          {visible && status === 'published' && (
            <span className="text-xs text-muted-soft">in the archive</span>
          )}
        </div>

        <p className="text-xs text-muted-soft">{copy.blurb}</p>

        {status === 'rejected' && reviewNote && (
          <div className="border border-bad/30 bg-bad/12 rounded-lg px-3 py-2">
            <p className="text-xs font-semibold text-bad mb-1">Reviewer's note</p>
            <p className="text-xs text-ink-soft whitespace-pre-wrap">{reviewNote}</p>
          </div>
        )}

        {/* Publishing is an admin decision: an author's action is to submit, not
            to flip a visibility switch. The old checkbox here sent `visible`,
            which the API now ignores for non-admins — it looked like it worked
            and did nothing. */}
        {canSubmit && (
          <button
            type="button"
            onClick={onSubmitForReview}
            disabled={saving}
            className="bg-brand text-on-brand px-4 py-2 rounded text-sm font-semibold hover:bg-brand-hover transition-colors cursor-pointer disabled:opacity-50"
          >
            {status === 'rejected' ? 'Resubmit for review' : 'Submit for review'}
          </button>
        )}

        {status === 'pending' && (
          <p className="text-xs text-muted-soft">
            You'll be notified when an admin decides. Withdrawing is not supported yet — ask an admin
            if you need to change it.
          </p>
        )}
      </div>

      <hr className="border-hairline" />

      <div className="space-y-2">
        <h3 className="text-sm font-bold text-bad uppercase tracking-wider">Danger Zone</h3>
        <p className="text-xs text-muted-soft">
          Deleting the problem will discard all statements, submissions history, and testcase files
          from the server sandbox. This operation cannot be undone.
        </p>
        <button
          onClick={onDelete}
          className="bg-danger-fill text-white px-4 py-2 rounded text-sm font-semibold hover:bg-danger-fill-hover transition-colors cursor-pointer"
        >
          Delete Problem
        </button>
      </div>
    </div>
  )
}
