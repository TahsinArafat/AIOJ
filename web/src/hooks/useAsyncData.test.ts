import { act, renderHook, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import { useAsyncData } from './useAsyncData'

/**
 * The one-request scaffold that eleven ContestManage tabs repeated.
 *
 * The behaviours worth pinning are the two that were wrong when each tab
 * hand-rolled this: a failed fetch leaving a permanent spinner (some tabs never
 * caught), and a slow response from a superseded request landing over a newer one.
 */

test('resolves data and clears loading', async () => {
    const { result } = renderHook(() => useAsyncData(() => Promise.resolve('value'), []))

    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toBe('value')
    expect(result.current.error).toBeNull()
})

test('a rejection surfaces the message rather than hanging', async () => {
    const { result } = renderHook(() => useAsyncData(() => Promise.reject(new Error('boom')), []))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('boom')
    // Data stays undefined, so a caller can tell "failed" from "empty".
    expect(result.current.data).toBeUndefined()
})

test('a non-Error rejection is still reported', async () => {
    const { result } = renderHook(() => useAsyncData(() => Promise.reject('plain string'), []))

    await waitFor(() => expect(result.current.error).toBe('plain string'))
})

test('reload refetches and shows loading again', async () => {
    const fetcher = vi.fn()
        .mockResolvedValueOnce('first')
        .mockResolvedValueOnce('second')

    const { result } = renderHook(() => useAsyncData(fetcher, []))
    await waitFor(() => expect(result.current.data).toBe('first'))

    act(() => { result.current.reload() })

    // The reload must not leave the old content on screen with no indication.
    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.data).toBe('second'))
    expect(fetcher).toHaveBeenCalledTimes(2)
})

// The bug this guards: a request started before a dep change must not write its
// result over the newer one.
test('a superseded request cannot overwrite a newer result', async () => {
    let releaseFirst: (v: string) => void = () => {}
    const first = new Promise<string>(res => { releaseFirst = res })

    const fetcher = vi.fn()
        .mockReturnValueOnce(first)
        .mockResolvedValueOnce('second')

    const { result, rerender } = renderHook(
        ({ id }: { id: number }) => useAsyncData(fetcher, [id]),
        { initialProps: { id: 1 } },
    )

    // Change the dep while the first request is still in flight.
    rerender({ id: 2 })
    await waitFor(() => expect(result.current.data).toBe('second'))

    // Let the stale one land. It must be discarded.
    await act(async () => {
        releaseFirst('stale')
        await first
    })

    expect(result.current.data).toBe('second')
})

test('a dep change refetches', async () => {
    const fetcher = vi.fn()
        .mockResolvedValueOnce('one')
        .mockResolvedValueOnce('two')

    const { result, rerender } = renderHook(
        ({ id }: { id: number }) => useAsyncData(fetcher, [id]),
        { initialProps: { id: 1 } },
    )
    await waitFor(() => expect(result.current.data).toBe('one'))

    rerender({ id: 2 })
    await waitFor(() => expect(result.current.data).toBe('two'))
    expect(fetcher).toHaveBeenCalledTimes(2)
})