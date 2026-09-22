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
            setState('err')
            setMsg('Missing verification token.')
            return
        }
        api.auth.verifyEmail(token)
            .then(() => setState('ok'))
            .catch((e: any) => {
                setState('err')
                setMsg(e.message || 'Verification failed')
            })
    }, [token])

    if (state === 'loading') {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center text-gray-600 dark:text-gray-300">
                Verifying your email…
            </div>
        )
    }
    if (state === 'ok') {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center">
                <h1 className="text-2xl font-bold mb-4">Email Verified</h1>
                <p className="text-gray-600 dark:text-gray-400 mb-6">
                    Your email is verified. You can now submit solutions.
                </p>
                <Link to="/login" className="inline-block bg-blue-600 text-white px-5 py-2 rounded-md text-sm font-medium hover:bg-blue-700">
                    Continue to Login
                </Link>
            </div>
        )
    }
    return (
        <div className="max-w-sm mx-auto mt-20 text-center">
            <h1 className="text-2xl font-bold mb-4">Verification Failed</h1>
            <p className="text-gray-600 dark:text-gray-400 mb-6">{msg}</p>
            <Link to="/login" className="text-blue-600 dark:text-blue-400 hover:underline">Back to Login</Link>
        </div>
    )
}
