import { describe, expect, it } from 'vitest'
import { clockUrgency, contestClockAt, contestStatus, formatCountdown, formatDuration, formatPenaltyMinutes } from './contestTime'

/**
 * The contest clock's strings.
 *
 * These were three implementations that had drifted: the contest list printed
 * "00d 00:00:00" for a countdown that had ended while the cards printed
 * "00:00:00", and its duration always carried the hour unit. The width of a
 * ticking clock has to be stable too — a clock that reflows as it counts is a
 * clock you read twice.
 */

describe('formatCountdown', () => {
    it('is HH:MM:SS within a day, so the width never changes', () => {
        expect(formatCountdown(0)).toBe('00:00:00')
        expect(formatCountdown(9 * 1000)).toBe('00:00:09')
        expect(formatCountdown(59 * 60_000 + 59_000)).toBe('00:59:59')
        expect(formatCountdown(3_600_000)).toBe('01:00:00')
        expect(formatCountdown(23 * 3_600_000 + 59 * 60_000)).toBe('23:59:00')
    })

    it('gains a day prefix only past 24 hours', () => {
        expect(formatCountdown(24 * 3_600_000)).toBe('1d 00:00:00')
        expect(formatCountdown(24 * 3_600_000 + 3_600_000)).toBe('1d 01:00:00')
        expect(formatCountdown(2 * 86_400_000)).toBe('2d 00:00:00')
    })

    it('never renders a negative clock', () => {
        // A round that has ended has no time left; a minus sign would read as a
        // bug rather than as "finished".
        expect(formatCountdown(-1)).toBe('00:00:00')
        expect(formatCountdown(-86_400_000)).toBe('00:00:00')
    })

    it('floors rather than rounding, so a clock never reads early', () => {
        expect(formatCountdown(1_999)).toBe('00:00:01')
        expect(formatCountdown(999)).toBe('00:00:00')
    })
})

describe('formatDuration', () => {
    it('scales its unit to the size of the span', () => {
        expect(formatDuration(0)).toBe('0s')
        expect(formatDuration(45_000)).toBe('45s')
        expect(formatDuration(90_000)).toBe('1m')
        expect(formatDuration(2 * 3_600_000 + 15 * 60_000)).toBe('2h 15m')
        expect(formatDuration(3 * 3_600_000)).toBe('3h 0m')
    })

    it('falls back to days past 24 hours', () => {
        expect(formatDuration(24 * 3_600_000)).toBe('1d 0h')
        expect(formatDuration(26 * 3_600_000)).toBe('1d 2h')
    })

    it('never renders a negative span', () => {
        expect(formatDuration(-5_000)).toBe('0s')
    })
})

describe('formatPenaltyMinutes', () => {
    it('reads the API value as minutes, never seconds', () => {
        // The scoring formats build the penalty with `.Minutes()`, so 180 is a
        // solve three hours into the round. Dividing it down rendered "3:00".
        expect(formatPenaltyMinutes(180)).toBe('3:00')
        expect(formatPenaltyMinutes(30)).toBe('30m')
        expect(formatPenaltyMinutes(1)).toBe('1m')
    })

    it('adds the wrong-attempt penalty in the same unit', () => {
        // 75 minutes plus three wrong attempts at 20 each, as ACM scores it.
        expect(formatPenaltyMinutes(135)).toBe('2:15')
    })

    it('keeps an hour-readable form past sixty minutes', () => {
        expect(formatPenaltyMinutes(59)).toBe('59m')
        expect(formatPenaltyMinutes(60)).toBe('1:00')
        expect(formatPenaltyMinutes(61)).toBe('1:01')
        expect(formatPenaltyMinutes(600)).toBe('10:00')
    })

    it('never renders a negative penalty', () => {
        expect(formatPenaltyMinutes(0)).toBe('0m')
        expect(formatPenaltyMinutes(-10)).toBe('0m')
    })
})

describe('clockUrgency', () => {
    it('is calm while there is plenty of time', () => {
        expect(clockUrgency(3 * 3_600_000)).toBe('calm')
        expect(clockUrgency(16 * 60_000)).toBe('calm')
    })

    it('warns inside the last 15 minutes', () => {
        // The previous clock had no urgency state at all, so a round two
        // minutes from ending looked exactly like one with three hours left.
        expect(clockUrgency(15 * 60_000)).toBe('warn')
        expect(clockUrgency(6 * 60_000)).toBe('warn')
    })

    it('is urgent inside the last 5 minutes', () => {
        expect(clockUrgency(5 * 60_000)).toBe('danger')
        expect(clockUrgency(60_000)).toBe('danger')
    })

    it('treats an expired clock as urgent rather than calm', () => {
        expect(clockUrgency(0)).toBe('danger')
        expect(clockUrgency(-1)).toBe('danger')
    })
})

describe('contestStatus', () => {
    const HOUR = 3_600_000
    const NOW = new Date('2026-06-01T12:00:00Z').getTime()

    it('names the phase from the instant it is given', () => {
        expect(contestStatus(NOW + HOUR, NOW + 2 * HOUR, null, NOW)).toBe('upcoming')
        expect(contestStatus(NOW - HOUR, NOW + HOUR, null, NOW)).toBe('running')
        expect(contestStatus(NOW - 2 * HOUR, NOW - HOUR, null, NOW)).toBe('ended')
    })

    it('is frozen once the scoreboard freeze passes, not before', () => {
        // The freeze holds the standings back; the round itself is still on.
        expect(contestStatus(NOW - HOUR, NOW + HOUR, NOW + 60_000, NOW)).toBe('running')
        expect(contestStatus(NOW - HOUR, NOW + HOUR, NOW - 60_000, NOW)).toBe('frozen')
    })
})

describe('contestClockAt', () => {
    const HOUR = 3_600_000
    const NOW = new Date('2026-06-01T12:00:00Z').getTime()
    const at = (offsetMs: number) => new Date(NOW + offsetMs).toISOString()

    it('derives remaining, elapsed and progress from one instant', () => {
        const clock = contestClockAt(at(-HOUR), at(HOUR), null, NOW)

        expect(clock.status).toBe('running')
        expect(clock.totalMs).toBe(2 * HOUR)
        expect(clock.elapsedMs).toBe(HOUR)
        expect(clock.remainingMs).toBe(HOUR)
        expect(clock.progress).toBe(50)
        expect(clock.urgency).toBe('calm')
    })

    it('clamps a round that has run past its end', () => {
        const clock = contestClockAt(at(-2 * HOUR), at(-HOUR), null, NOW)

        // The round was an hour long, so an hour is all the elapsed time it has.
        expect(clock.elapsedMs).toBe(HOUR)
        expect(clock.remainingMs).toBe(0)
        expect(clock.progress).toBe(100)
    })

    it('reads an unloaded or unparseable contest as ended with nothing left', () => {
        // The pages call this before the fetch resolves, and a clock that has
        // nothing to say must not claim the round is live.
        for (const clock of [
            contestClockAt(undefined, undefined, null, NOW),
            contestClockAt('not-a-date', 'not-a-date', null, NOW),
        ]) {
            expect(clock.status).toBe('ended')
            expect(clock.totalMs).toBe(0)
            expect(clock.progress).toBe(0)
            expect(clock.remainingMs).toBe(0)
        }
    })
})