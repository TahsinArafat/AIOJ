/**
 * Browser-timezone helpers for `<input type="datetime-local">`.
 *
 * The wire format is UTC (RFC 3339) and must stay that way: every timestamp the
 * API returns is an absolute instant, and competitions are compared across
 * timezones. What went wrong is the boundary. `new Date(iso).toISOString()`
 * re-emits UTC, so slicing it into a `datetime-local` input showed a UTC clock
 * face to someone reading a local clock, and saving it round-tripped through
 * `new Date(value).toISOString()` again — the two shifts did not cancel, so
 * every settings save silently moved the contest start by the UTC offset.
 *
 * A `datetime-local` input speaks local wall-clock time with no zone. These two
 * functions are the only place that conversion happens, and they use the local
 * getters/setters rather than toISOString so the value on screen always matches
 * what the user's own clock says.
 */

function pad2(n: number): string {
    return n < 10 ? `0${n}` : `${n}`
}

/**
 * RFC 3339 instant -> the `YYYY-MM-DDTHH:mm` a `datetime-local` input expects,
 * rendered in the browser's timezone. Returns '' for an absent or unparseable
 * value so an empty field stays empty.
 */
export function isoToLocalInput(iso: string | null | undefined): string {
    if (!iso) return ''
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return ''
    return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

/**
 * `datetime-local` value (local wall clock) -> the RFC 3339 instant to send.
 *
 * `new Date('2026-01-02T03:04')` is parsed as local time by every current
 * browser, so `.toISOString()` is the correct inverse of isoToLocalInput.
 * Returns undefined for an empty value, which callers use to mean "omit".
 */
export function localInputToIso(value: string | null | undefined): string | undefined {
    if (!value) return undefined
    const d = new Date(value)
    if (Number.isNaN(d.getTime())) return undefined
    return d.toISOString()
}
