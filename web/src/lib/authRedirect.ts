import type { Location } from 'react-router-dom'

export const LOGIN_PATH = '/login'

/**
 * Destinations that must never be a post-login redirect target. Sending the
 * user back to an auth page would loop, or drop them mid-flow.
 */
const NO_REDIRECT = new Set([
    LOGIN_PATH,
    '/register',
    '/forgot-password',
    '/reset-password',
    '/verify-email',
    '/oauth/complete',
])

/**
 * The login URL for somebody who was on `from`.
 *
 * The destination rides in the query string so it survives a reload, the back
 * button and the OAuth round-trip, and so the server can sign it. Only
 * same-origin paths are ever produced; `nextParam` validates on the way out.
 */
export function loginHref(from: Pick<Location, 'pathname' | 'search'>): string {
    if (NO_REDIRECT.has(from.pathname)) return LOGIN_PATH
    const next = `${from.pathname}${from.search}`
    if (next === '/' || next === '') return LOGIN_PATH
    return `${LOGIN_PATH}?${new URLSearchParams({ next }).toString()}`
}

/**
 * The validated `next` param, or null when it is absent or not a plain
 * same-origin path.
 *
 * `//host`, `https://host` and `/\host` are all refused so a crafted login
 * link cannot turn the sign-in form into an open redirect.
 */
export function nextParam(params: URLSearchParams): string | null {
    const raw = params.get('next')
    if (!raw || !raw.startsWith('/') || raw.startsWith('//') || raw.includes('\\')) return null
    // A destination that is itself an auth page would land the person back
    // where they started after signing in.
    if (NO_REDIRECT.has(raw.split(/[?#]/, 1)[0])) return null
    return raw
}

/**
 * The destination to navigate to after a successful sign-in: the validated
 * `next`, or "/" when there is none.
 */
export function nextPath(params: URLSearchParams): string {
    return nextParam(params) ?? '/'
}

/**
 * The destination after signing in, preferring a server-supplied one.
 *
 * `redirect_to` comes from the login response and names where the account
 * belongs — for a generated onsite team account that is its own contest. It
 * outranks `?next=` because that account has exactly one reasonable
 * destination and never chose one, whereas a person who followed a login link
 * mid-navigation did.
 *
 * The server vouches for the value, but it still goes through the same
 * same-origin validation as `?next=`: a redirect target is the one thing that
 * must never be trusted to be a URL rather than a path, whoever sent it. An
 * unusable value falls back to `next`, then to "/".
 */
export function postLoginPath(params: URLSearchParams, redirectTo?: string | null): string {
    if (redirectTo) {
        const safe = nextParam(new URLSearchParams({ next: redirectTo }))
        if (safe) return safe
    }
    return nextPath(params)
}
