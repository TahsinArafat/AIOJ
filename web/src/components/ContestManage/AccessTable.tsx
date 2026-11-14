import {
    AUDIENCE_LABELS,
    SURFACES,
    clearSurface,
    isInherited,
    publicSurfaces,
    setSurface,
    surfaceAudience,
} from '../../lib/contestAccess'
import type { AccessAudience, AccessConfig } from '../../lib/contestAccess'

/**
 * Who can access what — the per-surface audience table.
 *
 * Replaces the single "Scoreboard Visibility" select, which was the first surface
 * to need its own audience and turned out not to be the last: a round's problem
 * list, statements, booklet, submit path and board are five independent answers,
 * and one dropdown could only express the board's.
 *
 * Rows, not a matrix of checkboxes: each surface has a small closed set of
 * audiences, and a select per row keeps the vocabulary visible next to the
 * surface it applies to. The "inherited" marker matters — an inherited audience
 * and one stored on this round look identical otherwise, and a setter who cannot
 * tell which rows they changed will not dare clear one.
 *
 * Extracted from ContestManage's settings tab, which had grown to 768 lines with
 * this table inline. The state stays in the parent: the table is one field of the
 * form, and the form is saved as one PUT.
 */
export function AccessTable({
    accessConfig,
    listed,
    onChange,
}: {
    accessConfig: AccessConfig
    /** Whether the round is listed, which the summary line below defers to. */
    listed: boolean
    /** Applied to the config: set one surface, or clear it back to inherited. */
    onChange: (next: AccessConfig) => void
}) {
    return (
        <div>
            <label className="block text-sm font-medium text-ink-soft mb-2">Who can access what</label>
            <div className="rounded-lg border border-hairline-strong overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-surface-2">
                        <tr>
                            <th className="text-left px-3 py-2 font-medium text-muted">Part of the round</th>
                            <th className="text-left px-3 py-2 font-medium text-muted w-56">Who can access it</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-hairline">
                        {SURFACES.map((surface) => {
                            const inherited = isInherited(accessConfig, surface.key)
                            const audience = surfaceAudience(accessConfig, surface.key)
                            // Surfacing the resolved value on an inherited row is the
                            // point: "inherited" alone does not tell the setter
                            // whether the surface is currently open or closed.
                            const effective = inherited
                                ? `Inherited — ${AUDIENCE_LABELS[audience].toLowerCase()}`
                                : AUDIENCE_LABELS[audience]
                            return (
                                <tr key={surface.key}>
                                    <td className="px-3 py-2.5 align-top">
                                        <div className="flex items-center gap-2">
                                            <span className="font-medium text-ink">{surface.label}</span>
                                            {inherited && (
                                                <span className="text-[11px] px-1.5 py-0.5 rounded bg-surface-2 text-muted-soft">
                                                    inherited
                                                </span>
                                            )}
                                            {!inherited && surface.key !== 'participate' && (
                                                <button
                                                    type="button"
                                                    onClick={() => onChange(clearSurface(accessConfig, surface.key))}
                                                    className="text-[11px] text-muted-soft hover:text-brand underline"
                                                    aria-label={`Reset ${surface.label} to inherited`}
                                                >
                                                    reset
                                                </button>
                                            )}
                                        </div>
                                        <p className="text-xs text-muted-soft mt-0.5">{surface.hint}</p>
                                    </td>
                                    <td className="px-3 py-2.5 align-top">
                                        <select
                                            aria-label={`${surface.label} access`}
                                            value={accessConfig[surface.key] ?? ''}
                                            onChange={e => {
                                                const v = e.target.value
                                                onChange(v === ''
                                                    ? clearSurface(accessConfig, surface.key)
                                                    : setSurface(accessConfig, surface.key, v as AccessAudience))
                                            }}
                                            className="w-full border border-hairline-strong rounded-lg px-2.5 py-1.5 text-sm focus:ring-2 focus-visible:outline-[var(--ring)] focus:border-brand bg-surface"
                                        >
                                            <option value="">Same as the round ({AUDIENCE_LABELS[surfaceAudience(accessConfig, 'participate')].toLowerCase()})</option>
                                            <option value="open">Anyone who can open the round</option>
                                            {/* Not offered for submit: requiring an entrant in order to
                                                submit deadlocks the first submission of a round, because
                                                submitting is one of the two ways to become one. The server
                                                answers 400 for it, so showing it would be showing an option
                                                that cannot be saved. */}
                                            {surface.allowsParticipants && <option value="participants">Participants only</option>}
                                            <option value="public">Anyone</option>
                                            {surface.allowsStaff && <option value="staff">Organizers only</option>}
                                        </select>
                                        <p className="text-[11px] text-muted-soft mt-1">
                                            {surface.key === 'participate'
                                                ? 'The base rule. Surfaces left on "same as the round" follow this.'
                                                : inherited
                                                    ? `Currently ${AUDIENCE_LABELS[audience].toLowerCase()}, following the round's setting.`
                                                    : effective + '.'}
                                        </p>
                                    </td>
                                </tr>
                            )
                        })}
                    </tbody>
                </table>
            </div>
            <p className="text-xs text-muted-soft mt-2">
                {(() => {
                    const open = publicSurfaces(accessConfig)
                    // Whether the round is discoverable is a separate setting (List
                    // in the public contests list), so this line says what the
                    // audiences do and defers the listing question to the control
                    // that owns it. Before that control existed this text asserted
                    // the round stayed out of the list, which was true only because
                    // publishing and listing were the same bit.
                    const reach = open.length === 0
                        ? 'Nothing here is public: only the people who can open the round reach any of it.'
                        : `Anyone with the link can reach: ${open.map(s => s.label.toLowerCase()).join(', ')}.`
                    return listed
                        ? `${reach} This round is listed, so it also appears in the contest list and in search.`
                        : `${reach} This round is unlisted: reachable by its direct link, and not in the contest list or search.`
                })()}
            </p>
        </div>
    )
}