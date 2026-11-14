import { Link } from 'react-router-dom'
import { Shield } from 'lucide-react'
import type { Contest } from '../../types/api'

/**
 * The About tab: what this round is, who runs it, and what it offers.
 *
 * Extracted from ContestDetail, which had grown to 1204 lines in one component
 * with all five tabs inline. About was the one whose data was self-contained — it
 * reads only the contest row — so it moves without dragging the page's other
 * twenty pieces of state along.
 *
 * Deliberately NOT a second copy of the sidebar's Contest Info box. That one
 * carries the schedule, format and problem count; this one names the
 * capabilities, which had no home anywhere. Only the features that are ON are
 * listed, because a row reading "upsolving disabled" is a list of everything that
 * is not there.
 */

/** The fields this tab reads from a contest row. */
export type ContestAbout = Pick<
    Contest,
    | 'title'
    | 'description'
    | 'pdf_enabled'
    | 'registration_required'
    | 'statement_hidden'
    | 'upsolving_enabled'
    | 'virtual_contest_enabled'
    | 'judges'
>

export function ContestAboutTab({ contest }: { contest: ContestAbout }) {
    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-xl font-bold text-ink mb-4">About this round</h2>
                {contest.description?.trim() ? (
                    <p className="text-sm text-ink-soft whitespace-pre-wrap leading-relaxed">
                        {contest.description}
                    </p>
                ) : (
                    <p className="text-sm text-muted-soft">
                        The organizer has not added a description.
                    </p>
                )}
            </div>

            {/* Who runs it. Absent from the payload entirely when the caller may
                not read the round as content, which is the server's call — the
                page does not decide who may see the roster. */}
            {contest.judges !== undefined && (
                <div>
                    <h3 className="text-sm font-semibold text-ink uppercase tracking-wide mb-3">
                        Organizers
                    </h3>
                    {contest.judges.length === 0 ? (
                        <p className="text-sm text-muted-soft">No organizers are listed.</p>
                    ) : (
                        <ul className="flex flex-wrap gap-2">
                            {contest.judges.map(j => (
                                <li
                                    key={j.user_id || j.username}
                                    className="flex items-center gap-2 rounded-lg border border-hairline bg-surface px-3 py-1.5"
                                >
                                    <Shield className="w-3.5 h-3.5 text-muted-soft" />
                                    {j.username ? (
                                        <Link
                                            to={`/user/${j.username}`}
                                            className="text-sm font-medium text-brand hover:underline"
                                        >
                                            {j.username}
                                        </Link>
                                    ) : (
                                        <span className="text-sm text-ink-soft">unknown</span>
                                    )}
                                    <span className="text-[11px] uppercase tracking-wide text-muted-soft">
                                        {j.access_level === 'manager' ? 'manager' : 'judge'}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            )}

            {/* What the round offers. Only the features that are ON are listed. */}
            <div>
                <h3 className="text-sm font-semibold text-ink uppercase tracking-wide mb-3">
                    Features
                </h3>
                <ul className="flex flex-wrap gap-2">
                    {[
                        { on: contest.pdf_enabled !== false, label: 'Printable booklet' },
                        { on: contest.statement_hidden === true, label: 'Statements hidden (onsite)' },
                        { on: contest.upsolving_enabled === true, label: 'Upsolving' },
                        { on: contest.virtual_contest_enabled === true, label: 'Virtual participation' },
                    ].filter(f => f.on).map(f => (
                        <li
                            key={f.label}
                            className="rounded-full border border-hairline bg-surface px-3 py-1 text-xs text-ink-soft"
                        >
                            {f.label}
                        </li>
                    ))}
                    {contest.registration_required && (
                        <li className="rounded-full border border-hairline bg-surface px-3 py-1 text-xs text-ink-soft">
                            Registration required
                        </li>
                    )}
                </ul>
            </div>
        </div>
    )
}