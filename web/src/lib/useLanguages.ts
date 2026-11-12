import { useCallback, useEffect, useState } from 'react'
import { api } from './api'
import { FALLBACK_LANGUAGES, type Language, toLanguages } from './languages'

/**
 * The judge's language list, fetched once per page load and shared.
 *
 * Every page that lets the user pick a language needs this: the IDE, the
 * problem page, the contest problem page, and the admin submission filters.
 * Each of them used to import a hardcoded array, which is why an admin edit in
 * admin#languages changed nothing.
 *
 * The cache is module-level rather than per-component so opening the IDE after
 * a problem page does not refetch. `inflight` deduplicates the simultaneous
 * mount that happens when two of these render together.
 *
 * Failure is not fatal: the fallback list renders a working dropdown, because
 * a judge that is briefly unreachable should not leave the IDE with no
 * languages at all. The submission is the thing that has to be right, and the
 * server rejects an unknown key regardless of what the dropdown showed.
 */
let cache: Language[] | null = null
let inflight: Promise<Language[]> | null = null

/** Drop the cache so the next mount refetches. Used after an admin edit. */
export function invalidateLanguages(): void {
    cache = null
    inflight = null
}

async function loadLanguages(): Promise<Language[]> {
    if (cache) return cache
    if (!inflight) {
        inflight = (async () => {
            try {
                // Optional-call syntax on purpose: a synchronous throw here
                // would escape the promise chain and surface as an unhandled
                // rejection instead of the fallback. Tests that mock `api`
                // without a `languages` member are the common case that
                // proved the point.
                const res = await api.languages?.list?.()
                return toLanguages(res?.data)
            } catch {
                // Fall back rather than leaving the dropdown empty. Cached on
                // purpose: a broken endpoint should not retry on every
                // navigation. invalidateLanguages() forces a fresh attempt.
                return FALLBACK_LANGUAGES
            }
        })()
        // Clear the slot only after the value is cached, so a caller arriving
        // during resolution joins this request instead of starting another.
        inflight.then(next => {
            cache = next
            inflight = null
        }, () => { inflight = null })
    }
    return inflight
}

export interface UseLanguagesResult {
    languages: Language[]
    /** True until the first response lands (or the fallback is used). */
    loading: boolean
    /** Names only, for a `<select>`. */
    options: { value: string; label: string }[]
    refresh: () => void
}

export function useLanguages(): UseLanguagesResult {
    const [languages, setLanguages] = useState<Language[]>(cache ?? FALLBACK_LANGUAGES)
    const [loading, setLoading] = useState(cache === null)

    const run = useCallback((cancelled: () => boolean) => {
        loadLanguages().then(next => {
            if (cancelled()) return
            setLanguages(next)
            setLoading(false)
        })
    }, [])

    useEffect(() => {
        // Already cached: adopt it without a request. Reading the cache during
        // render would be a hydration hazard, so this stays in the effect.
        if (cache) {
            queueMicrotask(() => {
                setLanguages(cache as Language[])
                setLoading(false)
            })
            return
        }
        let dead = false
        run(() => dead)
        return () => { dead = true }
    }, [run])

    const refresh = useCallback(() => {
        invalidateLanguages()
        setLoading(true)
        run(() => false)
    }, [run])

    return {
        languages,
        loading,
        options: languages.map(l => ({ value: l.key, label: l.name })),
        refresh,
    }
}
