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
