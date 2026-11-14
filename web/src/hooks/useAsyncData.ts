import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * One request, with the loading/error/reload scaffold that every tab repeats.
 *
 * Eleven tab components in ContestManage open-coded the same four lines: a data
 * state, a loading state, a `load` callback, and a `useEffect` that calls it.
 * Each also swallowed its own error differently — some not at all, two with an
 * empty catch — so a failed fetch rendered as a permanent spinner on one tab and
 * an empty list on another.
 *
 * This is the shape they all wanted: data, whether it is still loading, the error
 * if it failed, and a reload for the actions that change what is on screen.
 *
 * The error is returned rather than thrown into a toast, because the tabs render
 * it inline where the content would be. A caller that wants a toast can watch
 * `error` and fire one.
 */
export interface AsyncState<T> {
    data: T | undefined
    loading: boolean
    /** The fetch error message, or null. Not cleared by a successful reload. */
    error: string | null
    /** Re-run the fetch. Safe to call after a mutation. */
    reload: () => void
}

/**
 * Run `fetcher` when `deps` change, and expose its state.
 *
 * `fetcher` is held in a ref so a caller can pass an inline arrow without
 * re-triggering the fetch on every render — the deps array is what decides when
 * to refetch, not the function's identity.
 *
 * A stale response cannot overwrite a newer one: the effect's cleanup clears a
 * flag the callbacks check, so a slow request started before a dep change is
 * discarded when it finally lands. Without it the UI shows the previous round's
 * data under the new round's heading.
 */
export function useAsyncData<T>(
    fetcher: () => Promise<T>,
    deps: ReadonlyArray<unknown>,
): AsyncState<T> {
    const [state, setState] = useState<{
        data: T | undefined
        loading: boolean
        error: string | null
    }>({ data: undefined, loading: true, error: null })
    const [nonce, setNonce] = useState(0)

    // The latest fetcher, without making it a dependency: a caller passing an
    // inline arrow should not refetch on every render, so the deps array decides
    // when to run and this only supplies the current closure.
    const latest = useRef(fetcher)

    useEffect(() => {
        latest.current = fetcher
    })

    useEffect(() => {
        // A request from a previous run is superseded: its result must not land.
        let live = true

        // Settled in the callbacks rather than synchronously in the effect body,
        // which would be a cascading render on every dep change.
        latest.current()
            .then(result => {
                if (!live) return
                setState({ data: result, loading: false, error: null })
            })
            .catch((e: unknown) => {
                if (!live) return
                setState(prev => ({
                    ...prev,
                    loading: false,
                    error: e instanceof Error ? e.message : String(e),
                }))
            })

        return () => { live = false }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [...deps, nonce])

    const reload = useCallback(() => {
        // Mark loading immediately so a reload does not leave the previous
        // content on screen under no indication that it is being replaced.
        setState(prev => ({ ...prev, loading: true }))
        setNonce(n => n + 1)
    }, [])

    return { ...state, reload }
}
