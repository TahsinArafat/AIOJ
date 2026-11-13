import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { api, clearTokens, setTokens } from './api'

/**
 * The upload and remote-import endpoints used to open-code `fetch` with
 * `if (token) headers['Authorization'] = ...` instead of going through
 * request(). Two things broke as a result:
 *
 *  1. No 401 refresh-and-retry, so an expired access token failed the request
 *     outright.
 *  2. With no Authorization header on the wire, the CSRF middleware stopped
 *     skipping its check, and the caller got 403 "csrf cookie missing" instead
 *     of a 401 it could recover from.
 *
 * These pin the shared path: the header is always attached, and a 401 is
 * refreshed once and retried.
 */

const ACCESS = 'access-token'
const REFRESH = 'refresh-token'
const NEW_ACCESS = 'refreshed-access-token'

let fetchMock: ReturnType<typeof vi.fn>
/**
 * Authorization header per call, captured as each request is sent.
 *
 * request() reuses one headers object for the initial send and its refresh
 * retry, so reading mock.calls afterwards reports the retry's token for both.
 */
let sentAuth: string[]

const jsonResponse = (body: unknown, status = 200): Response =>
    ({
        ok: status >= 200 && status < 300,
        status,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => body,
        text: async () => JSON.stringify(body),
    }) as unknown as Response

const unauthorized = (): Response =>
    ({
        ok: false,
        status: 401,
        headers: new Headers({ 'content-type': 'text/plain' }),
        json: async () => ({}),
        text: async () => 'unauthorized',
    }) as unknown as Response

/** Install a fetch mock, recording the Authorization header of each send. */
const mockFetch = (handler: (url: string, init: RequestInit) => Promise<Response>) => {
    fetchMock = vi.fn((url: string, init: RequestInit) => {
        const header = (init?.headers as Record<string, string> | undefined)?.Authorization
        if (header) sentAuth.push(header)
        return handler(String(url), init)
    })
    vi.stubGlobal('fetch', fetchMock)
}

const urlsCalled = () => (fetchMock.mock.calls as unknown as [string][]).map(([u]) => String(u))

beforeEach(() => {
    localStorage.clear()
    document.cookie = 'csrf=test-csrf-token'
    sentAuth = []
    setTokens(ACCESS, REFRESH)
})

afterEach(() => {
    vi.unstubAllGlobals()
    clearTokens()
})

describe('upload and import endpoints share the authenticated path', () => {
    test('import sends the bearer token', async () => {
        mockFetch(async () => jsonResponse({ status: 'ok', problem_id: 'p1', slug: 's' }))

        await api.problems.importCodeforces('1000', 'A')

        expect(urlsCalled()[0]).toContain('/problems/import/codeforces')
        expect(sentAuth).toEqual([`Bearer ${ACCESS}`])
    })

    test('multipart upload sends the bearer token and no JSON content-type', async () => {
        mockFetch(async () => jsonResponse({ status: 'ok' }))

        await api.problems.uploadTestcases('two-sum', new File(['x'], 'cases.zip'))

        const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
        expect(String(url)).toContain('/problems/two-sum/testcases')
        const headers = init.headers as Record<string, string>
        expect(headers.Authorization).toBe(`Bearer ${ACCESS}`)
        // FormData must set its own boundary; overriding it would break the upload.
        expect(headers['Content-Type']).toBeUndefined()
    })

    test('an expired token is refreshed once, then the request is retried', async () => {
        let importAttempts = 0
        mockFetch(async url => {
            if (url.includes('/auth/refresh')) {
                return jsonResponse({ access_token: NEW_ACCESS, refresh_token: 'rotated' })
            }
            if (url.includes('/problems/import/cses')) {
                importAttempts++
                return importAttempts === 1
                    ? unauthorized()
                    : jsonResponse({ status: 'ok', problem_id: 'p2', slug: 's2' })
            }
            return jsonResponse({})
        })

        const result = await api.problems.importCSES('1068')

        expect(importAttempts).toBe(2)
        expect(result).toEqual({ status: 'ok', problem_id: 'p2', slug: 's2' })
        // The retry must carry the replacement token, not the expired one.
        expect(sentAuth).toEqual([`Bearer ${ACCESS}`, `Bearer ${NEW_ACCESS}`])
    })

    test('a rejected refresh surfaces the session-expired error, once', async () => {
        let importAttempts = 0
        mockFetch(async url => {
            if (url.includes('/auth/refresh')) {
                // 403 is a policy answer (a suspended account), not a retryable 401.
                return jsonResponse({ error: 'account suspended' }, 403)
            }
            importAttempts++
            return unauthorized()
        })

        await expect(api.problems.importToph('abc')).rejects.toThrow('session expired')
        expect(importAttempts).toBe(1)
    })
})
