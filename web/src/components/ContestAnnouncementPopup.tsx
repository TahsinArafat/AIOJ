import { AlertTriangle, Megaphone, X } from 'lucide-react'

/**
 * The on-screen popup for a freshly posted contest announcement.
 *
 * A toast is transient and easy to miss while a competitor is typing code; a
 * posted announcement is the one message an organizer needs seen immediately.
 * This is a fixed panel rather than a modal on purpose: it must never cover the
 * editor, because the round is running while it is read.
 *
 * Rendered by the hook's consumer (the contest layout) so it appears on every
 * contest page, not only on the announcements tab.
 */
export interface ContestAnnouncementPopupProps {
    content: string
    author?: string
    onDismiss: () => void
    onViewAll: () => void
}

export function ContestAnnouncementPopup({
    content, author, onDismiss, onViewAll,
}: ContestAnnouncementPopupProps) {
    return (
        <div
            role="alert"
            aria-live="polite"
            aria-label="New contest announcement"
            className="fixed bottom-4 left-1/2 z-50 w-[min(92vw,32rem)] -translate-x-1/2
                       rounded-xl border border-warn/40 bg-surface shadow-lg"
        >
            <div className="flex items-start gap-3 p-4">
                <span className="mt-0.5 flex-shrink-0 text-warn">
                    <Megaphone className="h-5 w-5" aria-hidden="true" />
                </span>
                <div className="min-w-0 flex-1">
                    <p className="text-sm font-semibold text-ink">New announcement</p>
                    <p className="mt-1 max-h-40 overflow-y-auto whitespace-pre-wrap break-words text-sm text-ink-soft">
                        {content}
                    </p>
                    {author && <p className="mt-1 text-xs text-muted">Posted by {author}</p>}
                </div>
                <button
                    type="button"
                    onClick={onDismiss}
                    aria-label="Dismiss announcement"
                    className="flex-shrink-0 rounded p-1 text-muted transition-colors hover:bg-surface-2 hover:text-ink"
                >
                    <X className="h-4 w-4" aria-hidden="true" />
                </button>
            </div>
            <div className="flex items-center justify-between gap-3 border-t border-hairline px-4 py-2.5">
                <span className="flex items-center gap-1.5 text-xs text-muted">
                    <AlertTriangle className="h-3.5 w-3.5 text-warn" aria-hidden="true" />
                    Judges may post rulings that change scoring.
                </span>
                <button
                    type="button"
                    onClick={onViewAll}
                    className="flex-shrink-0 text-xs font-medium text-brand transition-colors hover:text-brand/80"
                >
                    View all
                </button>
            </div>
        </div>
    )
}