/**
 * Contest time formatting.
 *
 * Split from the clock components so both can be imported without dragging the
 * other along, and so the components module only exports components (fast
 * refresh needs that).
 *
 * These lived in three places and had drifted: the contest list printed
 * "00d 00:00:00" for a countdown that had ended where the cards printed
 * "00:00:00", and its duration always said "03h 00m" while the others dropped
 * the unit once it reached zero. One answer to "how long is left".
 */

function pad2(n: number): string {
    return n < 10 ? `0${n}` : `${n}`
}

/**
 * A duration as a clock reads it: `1d 04:05:06` past a day, `04:05:06` within
 * one. Always HH:MM:SS so the width does not change as the value ticks — a
 * jumping clock is a clock you have to look at twice to read.
 *
 * Clamped at zero: a contest that has ended has no time left, and a negative
 * clock is worse than a zero one.
 */
export function formatCountdown(ms: number): string {
    const total = Math.max(0, Math.floor(ms / 1000))
    const days = Math.floor(total / 86_400)
    const hours = Math.floor((total % 86_400) / 3600)
    const minutes = Math.floor((total % 3600) / 60)
    const seconds = total % 60
    if (days > 0) return `${days}d ${pad2(hours)}:${pad2(minutes)}:${pad2(seconds)}`
    return `${pad2(hours)}:${pad2(minutes)}:${pad2(seconds)}`
}

/** A span of time, phrased for prose: `2h 15m`, `45m`, `30s`. */
export function formatDuration(ms: number): string {
    const totalSeconds = Math.max(0, Math.floor(ms / 1000))
    if (totalSeconds < 60) return `${totalSeconds}s`
    const minutes = Math.floor(totalSeconds / 60)
    if (minutes < 60) return `${minutes}m`
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}h ${minutes % 60}m`
    const days = Math.floor(hours / 24)
    return `${days}d ${hours % 24}h`
}

/**
 * An ACM penalty as competition software prints it: whole minutes.
 *
 * The API sends `total_penalty` and a solved cell's `time` in MINUTES — the
 * scoring formats build them with `int(d.Sub(...).Minutes())`, and the wrong-
 * attempt penalty is added in the same unit. Both scoreboards used to treat
 * that number as seconds and divide it down, so a solve three hours in (180)
 * rendered as "3:00" and a per-problem cell read "3m".
 *
 * Past an hour it reads `H:MM`, otherwise `MMm` — the trailing `m` matters,
 * because a bare `3:00` is indistinguishable from a clock time.
 */
export function formatPenaltyMinutes(minutes: number): string {
    const total = Math.max(0, Math.floor(minutes))
    if (total < 60) return `${total}m`
    const hours = Math.floor(total / 60)
    return `${hours}:${pad2(total % 60)}`
}

/** Under 15 minutes: where "how much is left" starts being asked. */
const WARN_MS = 15 * 60_000
/** Under 5 minutes: answers must be given, not considered. */
const DANGER_MS = 5 * 60_000

export type ClockUrgency = 'calm' | 'warn' | 'danger'

/**
 * How loud the clock should be.
 *
 * The previous rendering had no urgency at all: a round two minutes from ending
 * looked identical to one with three hours left, which is backwards — the clock
 * matters most when it is nearly out.
 */
export function clockUrgency(remainingMs: number): ClockUrgency {
    if (remainingMs <= DANGER_MS) return 'danger'
    if (remainingMs <= WARN_MS) return 'warn'
    return 'calm'
}

export type ContestStatus = 'upcoming' | 'running' | 'frozen' | 'ended'

/** Everything a clock on screen is made of, derived from one instant. */
export interface ContestClock {
    now: number
    status: ContestStatus
    startMs: number
    endMs: number
    totalMs: number
    elapsedMs: number
    remainingMs: number
    /** 0–100. Zero when the round has no length to divide by. */
    progress: number
    urgency: ClockUrgency
}

/**
 * Where the round stands at `now`.
 *
 * `frozen` is the scoreboard freeze, not the contest: the round is still
 * running and submittable, the standings are just held back.
 */
export function contestStatus(
    startMs: number, endMs: number, freezeMs: number | null, now: number,
): ContestStatus {
    if (now < startMs) return 'upcoming'
    if (now > endMs) return 'ended'
    if (freezeMs !== null && now >= freezeMs) return 'frozen'
    return 'running'
}

/**
 * The clock at one instant, with no state of its own.
 *
 * Both contest pages used to read `Date.now()` once at render and derive the
 * round's phase from it while a separate timer moved the digits, so the number
 * counted down inside a page that still believed the round had not started: a
 * contest that began while you were reading never switched to the running clock
 * and never re-enabled submitting. One derivation, fed the tick, keeps the
 * digits and the phase in agreement.
 *
 * An unparseable date reads as ended with nothing left, which is the same
 * answer the pages gave when the contest had not loaded yet.
 */
export function contestClockAt(
    start: string | null | undefined, end: string | null | undefined,
    freeze: string | null | undefined, now: number,
): ContestClock {
    const startMs = start ? new Date(start).getTime() : NaN
    const endMs = end ? new Date(end).getTime() : NaN
    if (!Number.isFinite(startMs) || !Number.isFinite(endMs)) {
        return {
            now, status: 'ended', startMs: 0, endMs: 0, totalMs: 0,
            elapsedMs: 0, remainingMs: 0, progress: 0, urgency: 'danger',
        }
    }

    const totalMs = Math.max(0, endMs - startMs)
    const elapsedMs = Math.min(totalMs, Math.max(0, now - startMs))
    const remainingMs = Math.max(0, endMs - now)
    const freezeMs = freeze ? new Date(freeze).getTime() : NaN

    return {
        now,
        status: contestStatus(startMs, endMs, Number.isFinite(freezeMs) ? freezeMs : null, now),
        startMs,
        endMs,
        totalMs,
        elapsedMs,
        remainingMs,
        progress: totalMs > 0 ? (elapsedMs / totalMs) * 100 : 0,
        urgency: clockUrgency(remainingMs),
    }
}