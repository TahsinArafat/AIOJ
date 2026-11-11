import { describe, expect, it } from 'vitest'
import { isoToLocalInput, localInputToIso } from './datetime'

/**
 * The contest settings form round-trips a time through
 * `<input type="datetime-local">`, and the bug was that both halves used
 * toISOString(): the field showed a UTC clock face and saving shifted the value
 * again, so every save moved the contest start by the browser's UTC offset.
 *
 * These tests pin the invariant that actually matters: what comes back out is
 * the same instant that went in, whatever the machine's timezone is.
 */

describe('datetime-local round trip', () => {
    it('returns the same instant it was given', () => {
        const iso = '2026-03-14T09:30:00.000Z'
        const field = isoToLocalInput(iso)
        expect(localInputToIso(field)).toBe(new Date(iso).toISOString())
    })

    it('round-trips a contest start across a DST boundary', () => {
        // Both sides of the US spring-forward, so an offset-only conversion
        // would drift by an hour on one of them.
        for (const iso of ['2026-03-08T06:59:00.000Z', '2026-03-08T10:01:00.000Z']) {
            const field = isoToLocalInput(iso)
            expect(localInputToIso(field)).toBe(new Date(iso).toISOString())
        }
    })

    it('renders the field in local wall-clock time, not UTC', () => {
        const iso = '2026-07-01T12:00:00.000Z'
        const field = isoToLocalInput(iso)
        const d = new Date(iso)
        // The literal in the field must match the browser's own clock reading.
        expect(field).toBe(
            `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}` +
            `T${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`,
        )
    })

    it('treats empty and unparseable values as absent', () => {
        expect(isoToLocalInput('')).toBe('')
        expect(isoToLocalInput(null)).toBe('')
        expect(isoToLocalInput(undefined)).toBe('')
        expect(isoToLocalInput('not a date')).toBe('')
        expect(localInputToIso('')).toBeUndefined()
        expect(localInputToIso(null)).toBeUndefined()
        expect(localInputToIso('not a date')).toBeUndefined()
    })
})
