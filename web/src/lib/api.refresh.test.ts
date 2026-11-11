import { beforeEach, afterEach, describe, expect, test, vi } from 'vitest'
import { api, setTokens, clearTokens, getAccessToken } from './api'

/**
 * Regression coverage for the concurrent-401 refresh race.
 *
 * The notification bell polls every 30s and page loads fire several reads at
 * once, so multiple requests could 401 simultaneously. Each used to POST
 * /auth/refresh on its own; the server rotates refresh tokens, so the second
 * call presented an already-consumed token, the client called clearTokens(),
 * and the user was silently logged out. All 401s must now share one refresh.
 */

const ORIGINAL_TOKEN = 'original-access'
const ORIGINAL_REFRESH = 'original-refresh'

let fetchMock: ReturnType<typeof vi.fn>

function jsonResponse(body: unknown, status = 200): Response {
    return {
        ok: status >= 200 && status < 300,
        status,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => body,
        text: async () => JSON.stringify(body),
    } as unknown as Response
}

beforeEach(() => {
    localStorage.clear()
    document.cookie = 'csrf=test-csrf-token'
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    setTokens(ORIGINAL_TOKEN, ORIGINAL_REFRESH)
})

afterEach(() => {
    vi.unstubAllGlobals()
    clearTokens()
})

describe('api token refresh', () => {
    test('concurrent 401s trigger exactly one /auth/refresh call', async () => {
        // Every protected GET 401s until the token is rotated.
        fetchMock.mockImplementation(async (url: string) => {
            if (url.includes('/auth/refresh')) {
                return jsonResponse({ access_token: 'fresh-access', refresh_token: 'fresh-refresh' })
            }
            const auth = (fetchMock.mock.calls.find(c => String(c[0]).includes('/contests'))?.[1] as { headers?: Record<string, string> })?.headers?.Authorization
            if (!auth || auth === `Bearer ${ORIGINAL_TOKEN}`) {
                return jsonResponse({ error: 'expired' }, 401)
            }
            return jsonResponse({ data: [] })
        })

        const results = await Promise.all([
            api.contests.list(0, 20, 0),
            api.notifications.unreadCount(),
            api.ratings.getByContest('12'),
        ])

        const refreshCalls = fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))
        expect(refreshCalls).toHaveLength(1)
        // The refresh body must carry the token the client actually held.
        expect(JSON.parse(refreshCalls[0][1].body)).toEqual({ refresh_token: ORIGINAL_REFRESH })
        // Every caller gets through, and the rotated token is persisted.
        expect(results).toBeDefined()
        expect(getAccessToken()).toBe('fresh-access')
        expect(localStorage.getItem('refresh_token')).toBe('fresh-refresh')
    })

    test('failed refresh clears tokens and reports session expiry', async () => {
        fetchMock.mockImplementation(async (url: string) => {
            if (url.includes('/auth/refresh')) return jsonResponse({ error: 'invalid' }, 401)
            return jsonResponse({ error: 'expired' }, 401)
        })

        await expect(api.notifications.unreadCount()).rejects.toThrow('session expired')
        expect(getAccessToken()).toBeNull()
        expect(localStorage.getItem('access_token')).toBeNull()
        expect(localStorage.getItem('refresh_token')).toBeNull()
    })

    test('a later request can refresh again after a previous one settled', async () => {
        fetchMock.mockImplementation(async (url: string) => {
            if (url.includes('/auth/refresh')) {
                return jsonResponse({ access_token: 'rotated', refresh_token: 'rotated-refresh' })
            }
            return jsonResponse({ data: [] })
        })

        await api.contests.list(0, 20, 0)
        await api.contests.list(0, 20, 0)

        // First call is a plain 200, so no refresh is needed at all.
        expect(fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))).toHaveLength(0)
    })
})

/**
 * Rotation is one-time-use, and localStorage is shared by every tab. A tab
 * that refreshes replaces the stored token; any other tab is now holding a
 * spent one. Presenting it must not end the session — the client has to adopt
 * whatever the winning tab stored and carry on.
 */
describe('api token refresh across tabs', () => {
    test('prefers the token stored by another tab over the stale memory copy', async () => {
        // Another tab rotated before this one got to it.
        localStorage.setItem('refresh_token', 'rotated-by-other-tab')
        fetchMock.mockImplementation(async (url: string, init?: RequestInit) => {
            if (url.includes('/auth/refresh')) {
                return jsonResponse({ access_token: 'fresh-access', refresh_token: 'fresh-refresh' })
            }
            const auth = (init?.headers as Record<string, string> | undefined)?.Authorization
            return auth === 'Bearer fresh-access' ? jsonResponse({ count: 0 }) : jsonResponse({ error: 'expired' }, 401)
        })

        await api.notifications.unreadCount()

        const refreshCalls = fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))
        expect(JSON.parse(refreshCalls[0][1].body)).toEqual({ refresh_token: 'rotated-by-other-tab' })
        expect(localStorage.getItem('refresh_token')).toBe('fresh-refresh')
    })

    test('a token spent by a concurrent refresh falls back to the newer stored one', async () => {
        let refreshAttempts = 0
        fetchMock.mockImplementation(async (url: string, init?: RequestInit) => {
            if (url.includes('/auth/refresh')) {
                refreshAttempts++
                if (refreshAttempts === 1) {
                    // The other tab won the race and rotated while we were asking.
                    localStorage.setItem('refresh_token', 'winner-token')
                    return jsonResponse({ error: 'invalid refresh token' }, 401)
                }
                return jsonResponse({ access_token: 'fresh-access', refresh_token: 'fresh-refresh' })
            }
            const auth = (init?.headers as Record<string, string> | undefined)?.Authorization
            return auth === 'Bearer fresh-access' ? jsonResponse({ count: 0 }) : jsonResponse({ error: 'expired' }, 401)
        })

        await api.notifications.unreadCount()

        expect(refreshAttempts).toBe(2)
        expect(getAccessToken()).toBe('fresh-access')
        expect(localStorage.getItem('refresh_token')).toBe('fresh-refresh')
    })

    test('a signed-out browser stays signed out', async () => {
        // Another tab signed out: nothing is stored, and the memory copy is gone.
        clearTokens()
        fetchMock.mockImplementation(async () => jsonResponse({ error: 'expired' }, 401))

        await expect(api.notifications.unreadCount()).rejects.toThrow()
        expect(fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))).toHaveLength(0)
    })
})

/**
 * A refresh that could not be answered is not a verdict on the session.
 *
 * Refresh tokens rotate, so the client used to treat every failure — a 5xx, a
 * dropped connection, a restart — as "the token is dead", clear localStorage and
 * sign the user out. A deploy or a database blip was therefore enough to end a
 * healthy session, and one tab's failure signed out every other tab too.
 */
describe('api token refresh under failure', () => {
    test('a server failure during refresh keeps the session', async () => {
        fetchMock.mockImplementation(async (url: string) => {
            if (url.includes('/auth/refresh')) {
                return jsonResponse({ error: 'session store unavailable' }, 503)
            }
            return jsonResponse({ error: 'expired' }, 401)
        })

        await expect(api.notifications.unreadCount()).rejects.toThrow('connection lost')
        // The token is still valid, so the next request must be able to retry it.
        expect(getAccessToken()).toBe(ORIGINAL_TOKEN)
        expect(localStorage.getItem('refresh_token')).toBe(ORIGINAL_REFRESH)
    })

    test('a dropped connection during refresh keeps the session', async () => {
        fetchMock.mockImplementation(async (url: string) => {
            if (url.includes('/auth/refresh')) throw new TypeError('Failed to fetch')
            return jsonResponse({ error: 'expired' }, 401)
        })

        await expect(api.notifications.unreadCount()).rejects.toThrow('connection lost')
        expect(getAccessToken()).toBe(ORIGINAL_TOKEN)
        expect(localStorage.getItem('refresh_token')).toBe(ORIGINAL_REFRESH)
    })

    test('a rejected account still ends the session', async () => {
        // 403 is a policy answer (a suspended account), not a transport fault.
        fetchMock.mockImplementation(async (url: string) => {
            if (url.includes('/auth/refresh')) return jsonResponse({ error: 'suspended' }, 403)
            return jsonResponse({ error: 'expired' }, 401)
        })

        await expect(api.notifications.unreadCount()).rejects.toThrow('session expired')
        expect(localStorage.getItem('refresh_token')).toBeNull()
    })
})

/**
 * The rotation race across tabs.
 *
 * Single-flight only covers one tab. When two tabs hold the same token, one
 * spends it and the other's refresh comes back 401 — and if the winner has not
 * stored the replacement by the time the loser re-reads storage, the loser used
 * to conclude the session was over and sign out. The re-read backoff separates
 * "spent a moment ago by another tab" from "dead".
 */
describe('api token refresh race across tabs', () => {
    test('adopts the replacement another tab stores mid-refresh', async () => {
        let attempts = 0
        fetchMock.mockImplementation(async (url: string) => {
            if (url.includes('/auth/refresh')) {
                attempts++
                if (attempts === 1) {
                    // The other tab spent this token and is about to store its
                    // replacement; it simply has not got there yet.
                    setTimeout(() => localStorage.setItem('refresh_token', 'winner-token'), 60)
                    return jsonResponse({ error: 'invalid refresh token' }, 401)
                }
                return jsonResponse({ access_token: 'fresh-access', refresh_token: 'fresh-refresh' })
            }
            const auth = (fetchMock.mock.calls.at(-1)?.[1] as { headers?: Record<string, string> } | undefined)?.headers?.Authorization
            return auth === 'Bearer fresh-access' ? jsonResponse({ count: 0 }) : jsonResponse({ error: 'expired' }, 401)
        })

        await api.notifications.unreadCount()

        expect(attempts).toBeGreaterThanOrEqual(2)
        expect(getAccessToken()).toBe('fresh-access')
        expect(localStorage.getItem('refresh_token')).toBe('fresh-refresh')
    })
})
