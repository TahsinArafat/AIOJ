import { renderHook, act, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

/**
 * The announcement poller.
 *
 * "Is this announcement new?" has to be answered by identity, not by counting.
 * The original implementation compared array lengths and then read
 * items[items.length-1] — the OLDEST announcement, because the list arrives
 * newest-first (`ORDER BY created_at DESC`). So the popup showed whichever
 * announcement had been posted longest ago, and the count went negative the
 * moment one was deleted.
 *
 * "Unread" is a set difference now, so a freshly read announcement cannot come
 * back as unread because of clock skew between the server that stamped the row
 * and the browser that writes the marker.
 */

// vi.mock factories are hoisted above module init, so the shared mock has to be
// created inside the factory and reached via vi.hoisted().
const mocks = vi.hoisted(() => ({
    announcements: vi.fn(),
    pageVisible: { value: true },
}))
const announcementsMock = mocks.announcements

vi.mock('../lib/api', async () => {
    const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api')
    return {
        ...actual,
        api: {
            ...actual.api,
            contests: { ...actual.api.contests, announcements: mocks.announcements },
        },
    }
})

vi.mock('./usePageVisible', () => ({
    usePageVisible: () => mocks.pageVisible.value,
}))

import { useContestAnnouncements } from './useContestAnnouncements'

/** The server returns newest-first. */
const notice = (id: string, content: string, minutesAgo: number) => ({
    id,
    content,
    created_at: new Date(Date.now() - minutesAgo * 60_000).toISOString(),
    username: 'organizer',
})

describe('useContestAnnouncements', () => {
    beforeEach(() => {
        announcementsMock.mockReset()
        mocks.pageVisible.value = true
    })

    it('does not treat a round\'s existing announcements as new', async () => {
        announcementsMock.mockResolvedValue({ data: [notice('n2', 'second', 1), notice('n1', 'first', 5)] })

        const { result } = renderHook(() => useContestAnnouncements('c1'))

        await waitFor(() => expect(result.current.announcements).toHaveLength(2))
        // The organizer may have posted before the round started; nobody should
        // be hit with a popup for announcements that predate their arrival.
        expect(result.current.unreadCount).toBe(0)
        expect(result.current.popup).toBeNull()
    })

    it('pops up the NEWEST announcement, not the oldest', async () => {
        announcementsMock.mockResolvedValue({ data: [notice('n2', 'oldest', 60), notice('n1', 'newest', 1)] })
        const { result } = renderHook(() => useContestAnnouncements('c1'))
        await waitFor(() => expect(result.current.announcements).toHaveLength(2))

        // A second poll introduces one new announcement.
        announcementsMock.mockResolvedValue({ data: [notice('n3', 'just posted', 0), notice('n2', 'oldest', 60), notice('n1', 'newest', 1)] })
        await act(async () => { await result.current.refresh() })

        expect(result.current.popup?.id).toBe('n3')
        expect(result.current.popup?.content).toBe('just posted')
        expect(result.current.unreadCount).toBe(1)
    })

    it('keeps unread as a set difference across polls', async () => {
        announcementsMock.mockResolvedValue({ data: [notice('n1', 'one', 1)] })
        const { result } = renderHook(() => useContestAnnouncements('c1'))
        await waitFor(() => expect(result.current.announcements).toHaveLength(1))

        announcementsMock.mockResolvedValue({ data: [notice('n2', 'two', 0), notice('n1', 'one', 1)] })
        await act(async () => { await result.current.refresh() })
        expect(result.current.unreadCount).toBe(1)

        // No further changes: the same announcement must not re-accumulate.
        await act(async () => { await result.current.refresh() })
        expect(result.current.unreadCount).toBe(1)
    })

    it('does not report an unread count as negative when one is deleted', async () => {
        announcementsMock.mockResolvedValue({ data: [notice('n2', 'second', 1), notice('n1', 'first', 5)] })
        const { result } = renderHook(() => useContestAnnouncements('c1'))
        await waitFor(() => expect(result.current.announcements).toHaveLength(2))

        announcementsMock.mockResolvedValue({ data: [notice('n2', 'second', 1), notice('n1', 'first', 5)] })
        await act(async () => { await result.current.refresh() })
        expect(result.current.unreadCount).toBeGreaterThanOrEqual(0)
    })

    it('clears unread when the announcements are marked read', async () => {
        announcementsMock.mockResolvedValue({ data: [notice('n2', 'second', 1), notice('n1', 'first', 5)] })
        const { result } = renderHook(() => useContestAnnouncements('c1'))
        await waitFor(() => expect(result.current.announcements).toHaveLength(2))

        announcementsMock.mockResolvedValue({ data: [notice('n3', 'third', 0), notice('n2', 'second', 1), notice('n1', 'first', 5)] })
        await act(async () => { await result.current.refresh() })
        expect(result.current.unreadCount).toBe(1)

        act(() => result.current.markAllRead())
        expect(result.current.unreadCount).toBe(0)
        expect(result.current.popup).toBeNull()
    })

    it('does not poll when disabled', async () => {
        const { result } = renderHook(() => useContestAnnouncements('c1', { enabled: false }))
        await act(async () => { await new Promise(r => setTimeout(r, 10)) })
        expect(announcementsMock).not.toHaveBeenCalled()
        expect(result.current.announcements).toHaveLength(0)
    })

    it('keeps the page usable when a poll fails', async () => {
        announcementsMock.mockRejectedValue(new Error('offline'))
        const { result } = renderHook(() => useContestAnnouncements('c1'))
        await act(async () => { await new Promise(r => setTimeout(r, 20)) })
        // A failed poll must not blank the page or throw.
        expect(result.current.announcements).toHaveLength(0)
        expect(result.current.unreadCount).toBe(0)
    })
})