import { beforeEach, afterEach, describe, expect, test, vi } from 'vitest'
import { clearTokens, getAccessToken, setTokens, signOut } from './api'

/**
 * Sign-out used to only clear localStorage, leaving the refresh token valid on
 * the server for its full lifetime. signOut() must revoke it and must never
 * leave the user stuck in a half-signed-out state.
 */

const ACCESS = 'access-token'
const REFRESH = 'refresh-token'

let fetchMock: ReturnType<typeof vi.fn>

beforeEach(() => {
    localStorage.clear()
    document.cookie = 'csrf=test-csrf-token'
    fetchMock = vi.fn(async () => ({ ok: true, status: 204, headers: new Headers(), text: async () => '' } as unknown as Response))
    vi.stubGlobal('fetch', fetchMock)
    setTokens(ACCESS, REFRESH)
})

afterEach(() => {
    vi.unstubAllGlobals()
    clearTokens()
})

describe('signOut', () => {
    test('revokes the refresh token and clears local state', async () => {
        await signOut()

        expect(fetchMock).toHaveBeenCalledTimes(1)
        const [url, init] = fetchMock.mock.calls[0]
        expect(String(url)).toContain('/auth/logout')
        expect(init.method).toBe('POST')
        expect(JSON.parse(init.body)).toEqual({ refresh_token: REFRESH })
        expect(init.headers['X-CSRF-Token']).toBe('test-csrf-token')

        expect(getAccessToken()).toBeNull()
        expect(localStorage.getItem('refresh_token')).toBeNull()
    })

    test('signs out locally even when the request fails', async () => {
        fetchMock.mockImplementation(async () => {
            throw new Error('network down')
        })

        await expect(signOut()).resolves.toBeUndefined()
        expect(getAccessToken()).toBeNull()
        expect(localStorage.getItem('refresh_token')).toBeNull()
    })

    test('is a no-op without a session', async () => {
        clearTokens()
        await signOut()
        expect(fetchMock).not.toHaveBeenCalled()
    })
})
