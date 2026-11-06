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
    return (
        <Badge
            tone={meta.tone}
            dot={isPendingVerdict(status)}
            className={className}
            // The full label stays reachable when the table renders the
            // abbreviation.
            label={short ? meta.label : undefined}
        >
            {short ? meta.short : meta.label}
        </Badge>
    )
}
