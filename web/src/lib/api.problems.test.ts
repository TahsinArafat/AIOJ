import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { api, clearTokens } from './api'

/**
 * The problem-list UI has offered a Rating filter and a Sort-by control all
 * along, but `api.problems.list` only forwarded difficulty/tags/search/source.
 * Selecting a rating or a sort silently did nothing. This pins the query string
 * the handler actually reads.
 */

let fetchMock: ReturnType<typeof vi.fn>

const jsonResponse = (body: unknown): Response =>
    ({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => body,
        text: async () => JSON.stringify(body),
    }) as unknown as Response

const requestedUrl = () => {
    const calls = fetchMock.mock.calls as unknown as [string][]
    return calls.length === 0 ? '' : String(calls[calls.length - 1][0])
}

beforeEach(() => {
    localStorage.clear()
    fetchMock = vi.fn().mockResolvedValue(jsonResponse({ data: [], total: 0 }))
    vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
    vi.unstubAllGlobals()
    clearTokens()
})

describe('api.problems.list', () => {
    test('forwards the rating bucket and the sort option', async () => {
        await api.problems.list(0, 20, { rating: '1200-1599', sort: 'most_solved' })
        const url = requestedUrl()

        expect(url).toContain('rating=1200-1599')
        expect(url).toContain('sort=most_solved')
    })

    test('leaves unset filters out of the query', async () => {
        await api.problems.list(0, 20, { rating: '', sort: '' })
        const url = requestedUrl()

        expect(url).not.toContain('rating=')
        expect(url).not.toContain('sort=')
    })

    test('encodes a rating bucket with a trailing plus', async () => {
        await api.problems.list(0, 20, { rating: '2400+' })
        expect(requestedUrl()).toContain('rating=2400%2B')
    })
})
