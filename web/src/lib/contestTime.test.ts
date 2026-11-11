import { describe, expect, it } from 'vitest'
import { clockUrgency, formatCountdown, formatDuration } from './contestTime'

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