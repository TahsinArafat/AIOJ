import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../lib/api'
import type { Clarification } from '../types/api'
import { usePageVisible } from './usePageVisible'

/**
 * A round's clarification queue, with live updates.
 *
 * Clarifications are polled rather than pushed for the same reason announcements
 * are: a competitor watching the list needs a jury reply without reloading, and
 * a jury watching it needs new questions without reloading. Polling every few
 * seconds is cheap — the payload is one round's questions, and the interval is
 * paused while the tab is hidden.
 *
 * "New" is tracked by id, not by count: the list is ordered newest-first, so a
 * length comparison makes the oldest question look like the new one, and the
 * number goes wrong as soon as anything is deleted or filtered away.
 *
 * The filter state is owned by the caller (each surface wants different
 * defaults) and passed back in, so changing a filter is a refetch rather than a
 * client-side slice of a possibly stale page.
 */
export interface ClarificationsState {
    items: Clarification[]
    loading: boolean
    /** Ids of questions that arrived since this surface last fetched. */
    newIds: Set<string>
    /** True when the caller is staff and may answer. */
    isJudge: boolean
    error: string | null
    refresh: () => Promise<void>
    /** Apply an answer already returned by the API, without a refetch. */
    applyOne: (c: Clarification) => void
    /** Drop many ids, after a bulk answer. */
    applyMany: (ids: string[]) => void
    clearNew: () => void
}

export function useClarifications(
    contestId: string | undefined,
    filters: { problemId?: string; answered?: boolean },
    options?: { enabled?: boolean; pollMs?: number },
): ClarificationsState {
    const enabled = options?.enabled ?? true
    const pollMs = options?.pollMs ?? 10_000
    const pageVisible = usePageVisible()

    const [items, setItems] = useState<Clarification[]>([])
    const [loading, setLoading] = useState(true)
    const [isJudge, setIsJudge] = useState(false)
    // The ids already on screen, used to decide what is genuinely new.
    const known = useRef<Set<string>>(new Set())
    const [newIds, setNewIds] = useState<Set<string>>(new Set())

    const { problemId, answered } = filters

    const fetchAll = useCallback(async () => {
        if (!contestId) return
        try {
            const res = await api.clarifications.list(contestId, { problem_id: problemId, answered })
            const rows = res.data || []
            setItems(rows)
            setIsJudge(Boolean(res.is_judge))
            setLoading(false)
            // A question we have not seen before is new. On the first load
            // nothing is: a round's existing questions are not news to whoever
            // opens the tab after they were posted.
            setNewIds(prev => {
                const fresh = new Set<string>()
                rows.forEach(c => { if (known.current.has(c.id)) fresh.add(c.id) })
                if (fresh.size > 0) {
                    fresh.forEach(id => prev.add(id))
                }
                return prev
            })
            rows.forEach(c => known.current.add(c.id))
        } catch {
            // Keep whatever is on screen rather than blanking a queue the jury
            // is working through.
            setLoading(false)
        }
    }, [contestId, problemId, answered])

    useEffect(() => {
        // A filter change is a new question set: start from empty rather than
        // carrying ids across, or a question matching the new filter would look
        // "new" simply because it was filtered out before. Deferred a microtask:
        // react-hooks/set-state-in-effect rejects sync setState in an effect
        // body, and the spinner still lands pre-paint.
        known.current = new Set()
        queueMicrotask(() => {
            setNewIds(new Set())
            setLoading(true)
            if (!enabled || !contestId) setLoading(false)
        })
        if (!enabled || !contestId) return
        // Also deferred, for the same reason: the fetch writes state.
        queueMicrotask(() => { void fetchAll() })
        if (!pageVisible) return () => {}
        const timer = setInterval(() => { void fetchAll() }, pollMs)
        return () => clearInterval(timer)
    }, [enabled, contestId, fetchAll, pollMs, pageVisible])

    const applyOne = useCallback((c: Clarification) => {
        known.current.add(c.id)
        setItems(prev => prev.map(x => (x.id === c.id ? { ...x, ...c } : x)))
    }, [])

    const applyMany = useCallback((ids: string[]) => {
        setItems(prev => prev.filter(x => !ids.includes(x.id)))
    }, [])

    const clearNew = useCallback(() => setNewIds(new Set()), [])

    return {
        items, loading, newIds, isJudge,
        error: null,
        refresh: fetchAll,
        applyOne,
        applyMany,
        clearNew,
    }
}