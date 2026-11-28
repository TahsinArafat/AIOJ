import { Clock, Hourglass, Timer } from 'lucide-react'
import { clockUrgency, formatCountdown, type ClockUrgency, type ContestClock } from '../lib/contestTime'

/**
 * The contest clock, shared by every surface that shows one.
 *
 * The contest page, the problem page and the contest list each had their own
 * copy of `formatCountdown`/`formatDuration`, and they had already drifted: the
 * list printed "00d 00:00:00" for a countdown that had ended, the others printed
 * "00:00:00"; the list's duration was always "03h 00m" while the others dropped
 * the unit once it was zero. One clock means one answer to "how long is left".
 *
 * The old rendering was also a full-width saturated gradient with `text-3xl`
 * and a pulsing dot. On a working screen the clock is information, not a hero:
 * it now sits as a compact card built from the design tokens (so it is correct
 * in both themes, which the hardcoded emerald gradient never was), and it goes
 * quiet until the round is nearly over.
 *
 * The urgency states exist because the previous version had none: a contest two
 * minutes from ending looked exactly like one with three hours left, which is
 * backwards — the clock matters most when it is almost out.
 *
 * Both components render from a `ContestClock` they are handed rather than
 * deriving one: the page owns the tick (see `useContestClock`), so the digits
 * and the round's phase are always the same instant.
 */

const URGENCY_TONE: Record<ClockUrgency, { card: string; figure: string; label: string; bar: string; Icon: typeof Clock }> = {
    calm: {
        card: 'border-hairline bg-surface',
        figure: 'text-ink',
        label: 'text-muted',
        bar: 'bg-ok',
        Icon: Clock,
    },
    warn: {
        card: 'border-warn/40 bg-warn/[0.06]',
        figure: 'text-warn',
        label: 'text-warn',
        bar: 'bg-warn',
        Icon: Hourglass,
    },
    danger: {
        card: 'border-bad/50 bg-bad/[0.07]',
        figure: 'text-bad',
        label: 'text-bad',
        bar: 'bg-bad',
        Icon: Timer,
    },
}

/**
 * A round that has not started: how long until it does.
 *
 * Hidden entirely once it reaches zero, because the running clock takes over —
 * showing "00:00:00" next to a running contest says nothing useful.
 */
export function ContestCountdown({
    clock, label = 'Starts in',
}: { clock: ContestClock; label?: string }) {
    if (clock.status !== 'upcoming') return null

    // Until the *start*, not until the end: the clock's `remainingMs` counts
    // down to the round's close, which for a round that has not opened is the
    // wrong end of it.
    const untilStart = Math.max(0, clock.startMs - clock.now)
    const tone = URGENCY_TONE[clockUrgency(untilStart)]

    return (
        <div className={`flex items-center justify-between gap-3 rounded-lg border px-3 py-2 ${tone.card}`}>
            <span className={`flex items-center gap-2 text-xs font-medium ${tone.label}`}>
                <tone.Icon className="h-3.5 w-3.5 flex-shrink-0" aria-hidden="true" />
                {label}
            </span>
            <span className={`tnum font-mono text-base font-semibold tracking-tight ${tone.figure}`}>
                {formatCountdown(untilStart)}
            </span>
        </div>
    )
}

/**
 * A round in progress: how much time is left.
 *
 * Two rows and no more. The elapsed bar is kept because it is the one thing a
 * countdown number cannot tell you at a glance, but it is thin and quiet — the
 * number is the signal. The elapsed span and percentage used to take a third
 * row; they are on the bar's `title` now, because this card sits inside the
 * statement pane and every row it adds is a line of the problem pushed down.
 */
export function ContestRunningClock({ clock }: { clock: ContestClock }) {
    // A clock that has run out has nothing to count; the round has ended and the
    // status badge is the thing that should be speaking.
    if (clock.status !== 'running' && clock.status !== 'frozen') return null

    const tone = URGENCY_TONE[clock.urgency]
    const percent = Math.round(clock.progress)

    return (
        <div className={`rounded-lg border px-3 py-2 ${tone.card}`}>
            <div className="flex items-center justify-between gap-3">
                <span className={`flex items-center gap-2 text-xs font-medium ${tone.label}`}>
                    <span className="relative flex h-1.5 w-1.5 flex-shrink-0" aria-hidden="true">
                        <span className={`absolute inline-flex h-full w-full rounded-full opacity-60 ${tone.bar} ${clock.urgency === 'calm' ? 'animate-ping' : ''}`} />
                        <span className={`relative inline-flex h-1.5 w-1.5 rounded-full ${tone.bar}`} />
                    </span>
                    Running
                </span>
                <span className={`tnum font-mono text-base font-semibold tracking-tight ${tone.figure}`}>
                    {formatCountdown(clock.remainingMs)}
                    <span className="ml-1.5 text-[11px] font-normal text-muted">left</span>
                </span>
            </div>

            <div
                className="mt-1.5 h-1 w-full overflow-hidden rounded-full bg-surface-3"
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={percent}
                aria-label="Contest progress"
                title={`${percent}% elapsed`}
            >
                <div
                    className={`h-full rounded-full transition-[width] duration-1000 ease-linear ${tone.bar}`}
                    style={{ width: `${clock.progress}%` }}
                />
            </div>
        </div>
    )
}
