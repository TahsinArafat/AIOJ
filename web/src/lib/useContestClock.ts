import { useEffect, useState } from 'react'
import { contestClockAt, type ContestClock } from './contestTime'

/**
 * The contest clock, ticking, for whoever needs it.
 *
 * This is the single source of truth for "what time is it in this round". The
 * pages read `status` from here rather than calling `Date.now()` at render, so
 * a round that starts, freezes or ends while the page is open changes phase on
 * its own instead of waiting for a reload — and the digits and the phase can
 * never disagree, because both come from the same instant.
 *
 * One timer per page, not one per clock widget.
 *
 * Seeded with the current time, not the target: seeding a countdown with the
 * contest's start made the elapsed bar render at zero for one frame and then
 * jump on the first tick, which was visible on every load.
 *
 * The interval is cleared while the tab is hidden and resynced the moment it is
 * shown, so a backgrounded tab does not run a timer nobody is reading and the
 * clock is never stale on return. It also stops for good once the round is over:
 * there is nothing left to count.
 */
export function useContestClock(
    start: string | null | undefined,
    end: string | null | undefined,
    freeze?: string | null,
): ContestClock {
    const [now, setNow] = useState(() => Date.now())
    const clock = contestClockAt(start, end, freeze, now)
    const live = clock.remainingMs > 0

    useEffect(() => {
        if (!live) return
        let iv: ReturnType<typeof setInterval> | undefined
        const stop = () => {
            if (iv) { clearInterval(iv); iv = undefined }
        }
        // Deferred a microtask: the first tick writes state synchronously, which
        // react-hooks/set-state-in-effect rejects in an effect body. The clock
        // still lands pre-paint.
        const startTicking = () => {
            stop()
            queueMicrotask(() => setNow(Date.now()))
            iv = setInterval(() => setNow(Date.now()), 1000)
        }
        const onVisible = () => {
            if (document.visibilityState === 'visible') startTicking()
            else stop()
        }
        startTicking()
        document.addEventListener('visibilitychange', onVisible)
        return () => {
            stop()
            document.removeEventListener('visibilitychange', onVisible)
        }
    }, [live])

    return clock
}
