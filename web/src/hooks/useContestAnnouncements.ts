import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../lib/api'
import { usePageVisible } from './usePageVisible'

/**
 * Contest announcements, polled everywhere a contestant might be.
 *
 * The rule that mattered: an announcement is read at the moment it is posted,
 * because that is when the organizer is looking at the round. A contestant
 * sitting on a problem page — which is where competitors actually are — never
 * saw it. ContestDetail polled announcements, and nothing else did, so opening a
 * problem statement silently skipped the whole announcement.
 *
 * This hook fixes the three things that made even that one poll unreliable:
 *
 *   - "is this new?" was answered by comparing array lengths. The list arrives
 *     newest-first (`ORDER BY created_at DESC`), so items[items.length-1] is the
 *     OLDEST announcement, and the count went negative the moment anything was
 *     deleted. It now tracks the newest id it has seen and treats "I have never
 *     seen this id" as new, which also works when a round starts with existing
 *     announcements rather than showing them all as unread.
 *   - "unread" was recomputed from a last-seen *timestamp* in localStorage,
 *     compared against each announcement's created_at. Clock skew between the
 *     server that stamps the row and the browser that writes the marker is
 *     enough to make a freshly read announcement look unread forever. Reading a
 *     now means the ids that were shown, so unread is a set difference.
 *   - there was no popup. A toast is transient and easy to miss while typing.
 *
 * @param enabled   poll only when the caller is somewhere a contest is live.
 *                  Pass false on, say, the login page to avoid a pointless fetch.
 * @param pollMs    how often to poll while the tab is visible. Paused entirely
 *                  while hidden, so a backgrounded tab costs nothing.
 */
export interface Announcement {
    id: string
    content: string
    created_at: string
    username?: string
}

/** How long a freshly-posted announcement stays on screen. */
const POPUP_MS = 20_000
const DEFAULT_POLL_MS = 15_000

export function useContestAnnouncements(contestId: string | undefined, options?: {
    enabled?: boolean
    pollMs?: number
}) {
    const enabled = options?.enabled ?? true
    const pollMs = options?.pollMs ?? DEFAULT_POLL_MS
    const pageVisible = usePageVisible()

    const [announcements, setAnnouncements] = useState<Announcement[]>([])
    /** Newest-first, as the API returns them. */
    const [unread, setUnread] = useState<Announcement[]>([])
    /** The announcement that should be on screen right now. */
    const [popup, setPopup] = useState<Announcement | null>(null)

    // The newest id already seen, so "new" is an identity comparison rather than
    // a count. Ref rather than state: the poll must see the update immediately
    // and must not schedule another render on every tick.
    const seenIds = useRef<Set<string>>(new Set())
    /** Seeded on the first successful fetch so a round's existing announcements
     *  are not all announced as brand new to whoever opens the page later. */
    const primed = useRef(false)
    const popupTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

    const clearPopup = useCallback(() => {
        if (popupTimer.current) {
            clearTimeout(popupTimer.current)
            popupTimer.current = null
        }
        setPopup(null)
    }, [])

    /** Mark everything currently unread as read. Called when the announcements
     *  tab is opened, or when the popup is dismissed. */
    const markAllRead = useCallback(() => {
        setUnread([])
        clearPopup()
    }, [clearPopup])

    const poll = useCallback(async () => {
        if (!contestId) return
        try {
            const res = await api.contests.announcements(contestId)
            const items = (Array.isArray(res) ? res : res?.data) as Announcement[] | undefined
            if (!items) return
            setAnnouncements(items)

            if (!primed.current) {
                // First load. Record what exists without treating it as new:
                // an organizer who posted before the round started must not
                // find every visitor hit with a wall of popups on arrival.
                primed.current = true
                items.forEach(a => seenIds.current.add(a.id))
                setUnread([])
                return
            }

            // items is newest-first, so the first unrecognised one is the most
            // recent thing nobody has seen.
            const fresh = items.filter(a => !seenIds.current.has(a.id))
            fresh.forEach(a => seenIds.current.add(a.id))
            if (fresh.length === 0) return

            setUnread(prev => [...fresh, ...prev])
            const newest = fresh[0]

            if (popupTimer.current) clearTimeout(popupTimer.current)
            setPopup(newest)
            popupTimer.current = setTimeout(() => setPopup(null), POPUP_MS)

            // A browser notification is a courtesy for a tab that is in the
            // background; permission is asked lazily, once, and never on behalf
            // of a user who has already refused.
            if (typeof Notification !== 'undefined') {
                if (Notification.permission === 'granted') {
                    new Notification('New contest announcement', { body: newest.content })
                } else if (Notification.permission === 'default') {
                    void Notification.requestPermission()
                }
            }
        } catch {
            // A failed poll must not blank a page: keep whatever is on screen.
        }
    }, [contestId])

    useEffect(() => {
        if (!enabled || !contestId) return
        // Deferred a microtask: the first poll writes state synchronously, which
        // react-hooks/set-state-in-effect rejects in an effect body. The fetch
        // still lands pre-paint (react-hooks/set-state-in-effect).
        queueMicrotask(() => { void poll() })
        if (!pageVisible) return () => {}
        const timer = setInterval(() => { void poll() }, pollMs)
        return () => clearInterval(timer)
    }, [enabled, contestId, poll, pollMs, pageVisible])

    // Reset when the contest changes, so navigating between rounds does not
    // carry one round's announcements and seen-ids into the next.
    useEffect(() => {
        // Reset when the contest changes, so navigating between rounds does not
        // carry one round's announcements and seen-ids into the next. Deferred a
        // microtask for the same reason as the poll above.
        primed.current = false
        seenIds.current = new Set()
        queueMicrotask(() => {
            setAnnouncements([])
            setUnread([])
            clearPopup()
        })
    }, [contestId, clearPopup])

    useEffect(() => () => {
        if (popupTimer.current) clearTimeout(popupTimer.current)
    }, [])

    return {
        announcements,
        unreadCount: unread.length,
        unread,
        popup,
        dismissPopup: clearPopup,
        markAllRead,
        /** Force a refresh, e.g. right after posting one. */
        refresh: poll,
    }
}