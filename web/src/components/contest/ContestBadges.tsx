import { Badge, type BadgeTone } from '../ui/Badge'

/**
 * Contest status / format / type pills.
 *
 * These existed three times with three different vocabularies: ContestList knew
 * `acm|oi|ioi|cf|atcoder`, ContestDetail knew `ioi|icpc|cf` for `format` *and*
 * `educational|open` for `type`, and both had their own status colours. A
 * contest therefore looked different depending on which page you were on, and
 * adding a format meant editing every copy. They live here now, keyed off the
 * values the API actually returns.
 */

export type ContestStatus = 'upcoming' | 'running' | 'frozen' | 'ended'

const STATUS: Record<ContestStatus, { label: string; tone: BadgeTone }> = {
    upcoming: { label: 'Upcoming', tone: 'brand' },
    running: { label: 'Running', tone: 'ok' },
    frozen: { label: 'Frozen', tone: 'warn' },
    ended: { label: 'Finished', tone: 'neutral' },
}

export function ContestStatusBadge({ status }: { status: ContestStatus }) {
    const s = STATUS[status]
    return <Badge tone={s.tone} dot={status === 'running'}>{s.label}</Badge>
}

const FORMAT: Record<string, { label: string; tone: BadgeTone }> = {
    acm: { label: 'ACM', tone: 'brand' },
    icpc: { label: 'ICPC', tone: 'brand' },
    oi: { label: 'OI', tone: 'neutral' },
    ioi: { label: 'IOI', tone: 'ok' },
    cf: { label: 'CF', tone: 'warn' },
    atcoder: { label: 'AtCoder', tone: 'brand' },
}

export function ContestFormatBadge({ format }: { format?: string | null }) {
    if (!format) return null
    const key = format.toLowerCase()
    const f = FORMAT[key]
    return <Badge tone={f?.tone ?? 'neutral'}>{f?.label ?? format.toUpperCase()}</Badge>
}

const TYPE: Record<string, { label: string; tone: BadgeTone }> = {
    educational: { label: 'Educational', tone: 'ok' },
    open: { label: 'Open', tone: 'brand' },
    gym: { label: 'Gym', tone: 'neutral' },
}

/** `standard` (and the scoring formats) carry no extra meaning, so no pill. */
const SILENT_TYPES = new Set(['standard', 'acm', 'oi', 'ioi', 'practice', ''])

export function ContestTypeBadge({ type }: { type?: string | null }) {
    if (!type || SILENT_TYPES.has(type.toLowerCase())) return null
    const key = type.toLowerCase()
    const t = TYPE[key]
    return <Badge tone={t?.tone ?? 'accent'}>{t?.label ?? type}</Badge>
}
