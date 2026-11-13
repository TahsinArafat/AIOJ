import { Badge } from './Badge'
import { isPendingVerdict, verdictMeta } from './verdict'

/**
 * The single rendering of a judge verdict. Verdicts are the most-scanned text
 * in an online judge, so they get one component instead of five copies.
 */
export function VerdictBadge({
    status,
    short = false,
    className = '',
}: {
    status: string | undefined | null
    /** Use the abbreviation (AC/WA/TLE) for dense tables. */
    short?: boolean
    className?: string
}) {
    const meta = verdictMeta(status)
    // A voided submission keeps its label but is struck through: it is a ruling
    // that was made, not a result, so it must be legible and yet not read as
    // something the code achieved.
    const struck = meta.struck ? 'line-through opacity-70' : ''
    return (
        <Badge
            tone={meta.tone}
            dot={isPendingVerdict(status)}
            className={`${struck} ${className}`.trim()}
            // The full label stays reachable when the table renders the
            // abbreviation.
            label={short ? meta.label : undefined}
        >
            {short ? meta.short : meta.label}
        </Badge>
    )
}
