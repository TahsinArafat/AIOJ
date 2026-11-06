import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'

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
        api.auth.verifyEmail(token)
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
