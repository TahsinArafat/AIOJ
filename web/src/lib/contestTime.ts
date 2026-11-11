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