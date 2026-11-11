import { useEffect, useState } from 'react'
import { Clock, Hourglass, Timer } from 'lucide-react'
import { clockUrgency, formatCountdown, formatDuration, type ClockUrgency } from '../lib/contestTime'

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
 */

/**
 * Ticks once a second and hands back the current instant.
 *
 * Seeded with the current time, not with the target: seeding a countdown with
 * the contest's start made the elapsed bar render at zero for one frame and then
 * jump on the first tick, which was visible on every load.
 *
 * The interval is cleared while the tab is hidden and resynced the moment it is
 * shown, so a backgrounded tab does not run a timer nobody is reading and the
 * clock is never stale on return.
 */
function useNow(active = true): number {
    const [now, setNow] = useState(() => Date.now())

    useEffect(() => {
        if (!active) return
        let iv: ReturnType<typeof setInterval> | undefined
        const stop = () => {
            if (iv) { clearInterval(iv); iv = undefined }
        }
        // Deferred a microtask: the first tick writes state synchronously, which
        // react-hooks/set-state-in-effect rejects in an effect body. The clock
        // still lands pre-paint.
        const start = () => {
            stop()
            queueMicrotask(() => setNow(Date.now()))
            iv = setInterval(() => setNow(Date.now()), 1000)
        }
        const onVisible = () => {
            if (document.visibilityState === 'visible') start()
            else stop()
        }
        start()
        document.addEventListener('visibilitychange', onVisible)
        return () => {
            stop()
            document.removeEventListener('visibilitychange', onVisible)
        }
    }, [active])

    return now
}

const URGENCY_TONE: Record<ClockUrgency, { card: string; figure: string; label: string; Icon: typeof Clock }> = {
    calm: {
        card: 'border-hairline bg-surface',
        figure: 'text-ink',
        label: 'text-muted',
        Icon: Clock,
    },
    warn: {
        card: 'border-warn/40 bg-warn/[0.06]',
        figure: 'text-warn',
        label: 'text-warn',
        Icon: Hourglass,
    },
    danger: {
        card: 'border-bad/50 bg-bad/[0.07]',
        figure: 'text-bad',
        label: 'text-bad',
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
    target, label = 'Starts in',
}: { target: string; label?: string }) {
    const now = useNow()
    const remaining = new Date(target).getTime() - now
    if (remaining <= 0) return null

    const urgency = clockUrgency(remaining)
    const tone = URGENCY_TONE[urgency]

    return (
        <div className={`flex items-center justify-between gap-3 rounded-lg border px-3.5 py-2.5 ${tone.card}`}>
            <span className={`flex items-center gap-2 text-xs font-medium ${tone.label}`}>
                <tone.Icon className="h-3.5 w-3.5 flex-shrink-0" aria-hidden="true" />
                {label}
            </span>
            <span className={`tnum font-mono text-lg font-semibold tracking-tight ${tone.figure}`}>
                {formatCountdown(remaining)}
            </span>
        </div>
    )
}

/**
 * A round in progress: how much time is left.
 *
 * The elapsed bar is kept because it is the one thing a countdown number cannot
 * tell you at a glance, but it is thin and quiet — the number is the signal.
 */
export function ContestRunningClock({
    start, end,
}: { start: string; end: string }) {
    const now = useNow()
    const startTime = new Date(start).getTime()
    const endTime = new Date(end).getTime()
    const total = endTime - startTime
    const elapsed = Math.max(0, now - startTime)
    const remaining = endTime - now

    // A clock that has run out has nothing to count; the round has ended and the
    // status badge is the thing that should be speaking.
    if (remaining <= 0) return null

    const urgency = clockUrgency(remaining)
    const tone = URGENCY_TONE[urgency]
    const progress = total > 0 ? Math.min(100, Math.max(0, (elapsed / total) * 100)) : 0

    return (
        <div className={`rounded-lg border px-3.5 py-2.5 ${tone.card}`}>
            <div className="flex items-center justify-between gap-3">
                <span className={`flex items-center gap-2 text-xs font-medium ${tone.label}`}>
                    <span className="relative flex h-1.5 w-1.5 flex-shrink-0" aria-hidden="true">
                        <span className={`absolute inline-flex h-full w-full rounded-full opacity-60 ${urgency === 'danger' ? 'bg-bad' : urgency === 'warn' ? 'bg-warn' : 'bg-ok'} ${urgency === 'calm' ? 'animate-ping' : ''}`} />
                        <span className={`relative inline-flex h-1.5 w-1.5 rounded-full ${urgency === 'danger' ? 'bg-bad' : urgency === 'warn' ? 'bg-warn' : 'bg-ok'}`} />
                    </span>
                    Running
                </span>
                <span className={`tnum font-mono text-lg font-semibold tracking-tight ${tone.figure}`}>
                    {formatCountdown(remaining)}
                    <span className="ml-1.5 text-[11px] font-normal text-muted">left</span>
                </span>
            </div>

            <div
                className="mt-2 h-1 w-full overflow-hidden rounded-full bg-surface-3"
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={Math.round(progress)}
                aria-label="Contest progress"
            >
                <div
                    className={`h-full rounded-full transition-[width] duration-1000 ease-linear ${urgency === 'danger' ? 'bg-bad' : urgency === 'warn' ? 'bg-warn' : 'bg-ok'}`}
                    style={{ width: `${progress}%` }}
                />
            </div>

            <div className="mt-1.5 flex items-center justify-between text-[11px] text-muted">
                <span>Elapsed {formatDuration(elapsed)}</span>
                <span>{Math.round(progress)}%</span>
            </div>
        </div>
    )
}