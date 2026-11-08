import { useEffect, useState } from 'react'
import { api } from './api'
import type { AuthMethods } from '../types/api'

/**
 * Which sign-in methods this instance offers.
 *
 * The server resolves this because half of the answer is not knowable in the
 * browser: an OAuth provider is only usable when an operator configured
 * credentials AND an admin left it switched on. Letting the client guess would
 * mean rendering a "Continue with Google" button on instances that never set a
 * client id.
 *
 * `methods` stays null until the request settles, and on failure. Callers treat
 * null as "assume available": the start endpoint rejects a genuinely
 * unavailable provider with a readable error, whereas hiding the buttons on a
 * slow response could remove the only way in.
 */
export function useAuthMethods(): { methods: AuthMethods | null; loading: boolean } {
    const [methods, setMethods] = useState<AuthMethods | null>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        let cancelled = false
        api.auth
            .methods()
            .then(res => {
                if (!cancelled) setMethods(res?.data ?? null)
            })
            .catch(() => {
                // Fail open, see the doc comment above.
                if (!cancelled) setMethods(null)
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })
        return () => {
            cancelled = true
        }
    }, [])

    return { methods, loading }
}

/** True when at least one OAuth provider is available, or not yet known. */
export function hasOAuthProvider(methods: AuthMethods | null): boolean {
    if (!methods) return true
    return methods.google || methods.github
}
