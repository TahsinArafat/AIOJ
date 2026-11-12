import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import { FALLBACK_LANGUAGES } from './languages'
import { invalidateLanguages, useLanguages } from './useLanguages'

/**
 * The IDE and the submit forms read their language list through this hook, and
 * the whole point of the hook is that the list comes from the server — a
 * hardcoded array is exactly the bug this replaced.
 *
 * The module-level cache is the risky part: it must not leak between tests, and
 * it must not turn one failed request into a permanently empty dropdown while
 * also not hammering a broken endpoint on every navigation.
 */

const list = vi.fn()

vi.mock('./api', () => ({
    api: {
        get languages() {
            return { list }
        },
    },
}))

beforeEach(() => {
    list.mockReset()
    invalidateLanguages()
})

test('serves the languages the server reports', async () => {
    list.mockResolvedValue({
        data: [
            { key: 'cpp-gpp-64', name: 'C++ 20 (GCC 13)', extensions: ['.cpp'] },
            { key: 'brainfuck', name: 'Brainfuck', extensions: ['.bf'] },
        ],
    })

    const { result } = renderHook(() => useLanguages())
    await waitFor(() => expect(result.current.loading).toBe(false))

    // The renamed display name and the unfamiliar key both survive: this is the
    // "admin edited the panel" path the hardcoded list could never support.
    expect(result.current.options).toEqual([
        { value: 'cpp-gpp-64', label: 'C++ 20 (GCC 13)' },
        { value: 'brainfuck', label: 'Brainfuck' },
    ])
})

test('falls back to the shipped list when the request fails', async () => {
    list.mockRejectedValue(new Error('network down'))

    const { result } = renderHook(() => useLanguages())
    await waitFor(() => expect(result.current.loading).toBe(false))

    // A dropdown with no languages would make the IDE unusable; the fallback
    // keeps it working, and the server still rejects an unknown key.
    expect(result.current.options).toHaveLength(FALLBACK_LANGUAGES.length)
})

test('a missing endpoint does not throw, it falls back', async () => {
    // An older backend without GET /api/languages, or a partial test mock.
    list.mockRejectedValue(new TypeError('not a function'))

    const { result } = renderHook(() => useLanguages())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.options).toHaveLength(FALLBACK_LANGUAGES.length)
})

test('the list is fetched once and shared across callers', async () => {
    list.mockResolvedValue({ data: [{ key: 'python', name: 'Python 3', extensions: ['.py'] }] })

    const first = renderHook(() => useLanguages())
    await waitFor(() => expect(first.result.current.loading).toBe(false))

    // A second consumer — the IDE after the problem page — must reuse the cache.
    const second = renderHook(() => useLanguages())
    await waitFor(() => expect(second.result.current.options[0]?.value).toBe('python'))

    expect(list).toHaveBeenCalledTimes(1)
})

test('a failure is cached too, so a broken judge is not polled on every page', async () => {
    list.mockRejectedValue(new Error('down'))

    const first = renderHook(() => useLanguages())
    await waitFor(() => expect(first.result.current.loading).toBe(false))

    const second = renderHook(() => useLanguages())
    await waitFor(() => expect(second.result.current.loading).toBe(false))

    expect(list).toHaveBeenCalledTimes(1)
})

test('refresh re-reads after an admin edit', async () => {
    list.mockResolvedValueOnce({ data: [{ key: 'old', name: 'Old Name', extensions: [] }] })

    const { result } = renderHook(() => useLanguages())
    await waitFor(() => expect(result.current.options[0]?.value).toBe('old'))

    // The panel calls invalidateLanguages() after saving; refresh does the same
    // for anything holding this hook open.
    list.mockResolvedValueOnce({ data: [{ key: 'new', name: 'New Name', extensions: [] }] })
    act(() => { result.current.refresh() })

    await waitFor(() => expect(result.current.options[0]?.value).toBe('new'))
    expect(list).toHaveBeenCalledTimes(2)
})
