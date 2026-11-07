import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { AUTH_CHANGE_EVENT, api } from '../lib/api'
import type { ResolvedFeature } from '../types/api'

/**
 * Feature gating for the SPA.
 *
 * The verdict is computed on the server (it needs the caller's role and
 * rating), so this context is a cache of GET /api/features, not a policy
 * engine. Two rules keep it honest:
 *
 *  1. It fails open. If the request fails, `features` is empty and every gate
 *     renders its children. The backend still enforces the policy, so a user
 *     who slips past a hidden link gets a clear 403 from the API — far better
 *     than a blank site because one optional endpoint was down.
 *  2. It refetches when the session changes, because the verdict for the same
 *     feature differs between an anonymous visitor and a rated user.
 */
interface FeaturesState {
    /** Keyed by feature key. Empty means "not loaded (yet)". */
    features: Record<string, ResolvedFeature>
    loading: boolean
    error: string | null
    refresh: () => Promise<void>
    /** True unless the feature is known to be denied. */
    isAllowed: (key: string) => boolean
    /** The resolved entry, for showing the admin's message or a reason. */
    get: (key: string) => ResolvedFeature | undefined
}

const FeaturesContext = createContext<FeaturesState | null>(null)

export function FeaturesProvider({ children }: { children: React.ReactNode }) {
    const [features, setFeatures] = useState<Record<string, ResolvedFeature>>({})
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    // Guards against a slow response for a previous session overwriting the
    // verdicts of the current one (login while the anonymous fetch is in
    // flight is the common case).
    const requestSeq = useRef(0)

    const load = useCallback(async () => {
        const seq = ++requestSeq.current
        try {
            const res = await api.features.list()
            if (seq !== requestSeq.current) return
            const next: Record<string, ResolvedFeature> = {}
            for (const f of res?.data ?? []) next[f.key] = f
            setFeatures(next)
            setError(null)
        } catch (err) {
            if (seq !== requestSeq.current) return
            // Fail open: keep whatever we had and stop gating.
            setError(err instanceof Error ? err.message : 'failed to load features')
        } finally {
            if (seq === requestSeq.current) setLoading(false)
        }
    }, [])

    // Deferred one microtask: `load` flips `loading` synchronously, and
    // calling setState directly inside an effect body triggers cascading
    // renders (react-hooks/set-state-in-effect).
    useEffect(() => {
        queueMicrotask(load)
    }, [load])

    useEffect(() => {
        const onAuthChange = () => {
            setLoading(true)
            void load()
        }
        window.addEventListener(AUTH_CHANGE_EVENT, onAuthChange)
        return () => window.removeEventListener(AUTH_CHANGE_EVENT, onAuthChange)
    }, [load])

    const value = useMemo<FeaturesState>(() => {
        const get = (key: string) => features[key]
        return {
            features,
            loading,
            error,
            refresh: load,
            // Unknown feature (not loaded, or a typo): allow. The server is the
            // authority; hiding a surface because the client did not hear about
            // it would break pages whenever /api/features lags a deploy.
            isAllowed: (key: string) => {
                const f = features[key]
                return f ? f.allowed : true
            },
            get,
        }
    }, [features, loading, error, load])

    return <FeaturesContext.Provider value={value}>{children}</FeaturesContext.Provider>
}

export function useFeatures(): FeaturesState { // eslint-disable-line react-refresh/only-export-components -- the hook is the point of this module; splitting it would churn every import
    const ctx = useContext(FeaturesContext)
    if (!ctx) throw new Error('useFeatures must be used inside <FeaturesProvider>')
    return ctx
}

/** Convenience hook for a single feature. */
export function useFeature(key: string): { allowed: boolean; feature?: ResolvedFeature; loading: boolean } { // eslint-disable-line react-refresh/only-export-components -- see useFeatures
    const { isAllowed, get, loading } = useFeatures()
    return { allowed: isAllowed(key), feature: get(key), loading }
}
