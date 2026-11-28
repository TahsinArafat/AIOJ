import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useContestClock } from './useContestClock'

/**
 * The ticking half of the contest clock.
 *
 * Both contest pages used to call `Date.now()` once at render to decide whether
 * a round was upcoming, running or over, and then tick a second, private timer
 * just for the digits. The number moved; the page around it did not. A contest
 * that started while you were on the page kept its countdown leftovers, never
 * switched to the running clock, and never re-enabled submitting until reload.
 *
 * These tests pin the fix: the phase comes from the same tick as the digits, so
 * it changes on its own, and the timer stops when there is nothing left to
 * count.
 *
 * The window is built once per test, before the render. Rebuilding it inside
 * the render callback would move the round forward with the clock under test —
 * every render would see "one hour left" and nothing would look like it ticked.
 */

const HOUR = 3_600_000
const MINUTE = 60_000

function at(offsetMs: number): string {
    return new Date(Date.now() + offsetMs).toISOString()
}

beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-01T12:00:00Z'))
})

afterEach(() => {
    vi.useRealTimers()
})

describe('useContestClock', () => {
    it('ticks down once a second', async () => {
        const start = at(-HOUR)
        const end = at(HOUR)
        const { result } = renderHook(() => useContestClock(start, end))
        // The deferred resync lands before the first interval tick.
        await act(async () => { await Promise.resolve() })
        expect(result.current.remainingMs).toBe(HOUR)

        act(() => { vi.advanceTimersByTime(2000) })
        expect(result.current.remainingMs).toBe(HOUR - 2000)
    })

    it('starts the round without a reload', async () => {
        const start = at(2 * MINUTE)
        const end = at(3 * HOUR)
        const { result } = renderHook(() => useContestClock(start, end))
        await act(async () => { await Promise.resolve() })
        expect(result.current.status).toBe('upcoming')

        // Two minutes later the round is on: the page must know, because
        // submitting, the spoiler strip and the clock card all key off it.
        act(() => { vi.advanceTimersByTime(2 * MINUTE) })
        expect(result.current.status).toBe('running')
        expect(result.current.elapsedMs).toBe(0)
    })

    it('ends the round without a reload, and stops ticking', async () => {
        const start = at(-HOUR)
        const end = at(1000)
        const { result } = renderHook(() => useContestClock(start, end))
        await act(async () => { await Promise.resolve() })
        expect(result.current.status).toBe('running')

        act(() => { vi.advanceTimersByTime(2000) })
        expect(result.current.status).toBe('ended')
        expect(result.current.remainingMs).toBe(0)
        expect(vi.getTimerCount()).toBe(0)
    })

    it('reports the freeze while the round is still running', async () => {
        const start = at(-HOUR)
        const end = at(HOUR)
        const freeze = at(-10 * MINUTE)
        const { result } = renderHook(() => useContestClock(start, end, freeze))
        await act(async () => { await Promise.resolve() })

        expect(result.current.status).toBe('frozen')
        expect(result.current.remainingMs).toBe(HOUR)
    })

    it('tracks progress across the round', async () => {
        const start = at(-HOUR)
        const end = at(HOUR)
        const { result } = renderHook(() => useContestClock(start, end))
        await act(async () => { await Promise.resolve() })
        expect(Math.round(result.current.progress)).toBe(50)

        act(() => { vi.advanceTimersByTime(30 * MINUTE) })
        expect(Math.round(result.current.progress)).toBe(75)
    })

    it('reads a contest it has not loaded yet as ended, with no timer running', () => {
        // The pages call the hook before the fetch resolves, and hooks cannot be
        // conditional. This is the same answer they used to give for "no contest".
        const { result } = renderHook(() => useContestClock(undefined, undefined))

        expect(result.current.status).toBe('ended')
        expect(result.current.remainingMs).toBe(0)
        expect(result.current.progress).toBe(0)
        expect(vi.getTimerCount()).toBe(0)
    })
})
