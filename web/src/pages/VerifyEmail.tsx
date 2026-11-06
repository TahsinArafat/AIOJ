import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'

/**
 * One in-flight verify request per token, shared across mounts.
 *
 * Cleared once it settles so a genuine retry (user re-opens the link after a
 * failure) still hits the server rather than replaying a cached rejection.
 */
const inFlight = new Map<string, Promise<unknown>>()

function verifyOnce(token: string): Promise<unknown> {
    const existing = inFlight.get(token)
    if (existing) return existing
    const p = api.auth.verifyEmail(token).finally(() => {
        inFlight.delete(token)
    })
    inFlight.set(token, p)
    return p
}

export default function VerifyEmail() {
    const [params] = useSearchParams()
    const token = params.get('token') || ''
    const [state, setState] = useState<'loading' | 'ok' | 'err'>('loading')
    const [msg, setMsg] = useState('')

    useEffect(() => {
        if (!token) {
            // deferred a microtask: error lands pre-paint (react-hooks/set-state-in-effect)
            queueMicrotask(() => {
                setState('err')
                setMsg('Missing verification token.')
            })
            return
        }
        // The token is single-use server-side, so the request must fire exactly
        // once per token. React StrictMode deliberately mounts, unmounts and
        // re-mounts every component in development, which ran this effect
        // twice: the first call verified and the second came back "token
        // already used", painting a failure over a success. Sharing one
        // in-flight promise makes the effect idempotent — and also covers a
        // genuine remount (back/forward) landing on the same link.
        verifyOnce(token)
            .then(() => setState('ok'))
            .catch((e) => {
                setState('err')
                setMsg((e instanceof Error ? e.message : '') || 'Verification failed')
            })
    }, [token])

    if (state === 'loading') {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center text-muted">
                Verifying your email…
            </div>
        )
    }
    if (state === 'ok') {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center">
                <h1 className="text-2xl font-bold mb-4">Email Verified</h1>
                <p className="text-muted mb-6">
                    Your email is verified. You can now submit solutions.
                </p>
                <Link to="/login" className="inline-block bg-brand text-on-brand px-5 py-2 rounded-md text-sm font-medium hover:bg-brand-hover">
                    Continue to Login
                </Link>
            </div>
        )
    }
    return (
        <div className="max-w-sm mx-auto mt-20 text-center">
            <h1 className="text-2xl font-bold mb-4">Verification Failed</h1>
            <p className="text-muted mb-6">{msg}</p>
            <Link to="/login" className="text-brand hover:underline">Back to Login</Link>
        </div>
    )
}
