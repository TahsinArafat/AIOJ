import { useState } from 'react'
import { useSearchParams, Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function ResetPassword() {
    const [searchParams] = useSearchParams()
    const token = searchParams.get('token') || ''

    const [password, setPassword] = useState('')
    const [confirm, setConfirm] = useState('')
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [done, setDone] = useState(false)

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        if (password !== confirm) {
            setErr('Passwords do not match')
            return
        }
        if (password.length < 6) {
            setErr('Password too short (min 6)')
            return
        }
        setLoading(true)
        try {
            await api.auth.resetPassword({ token, new_password: password })
            setDone(true)
        } catch (e: any) {
            setErr(e.message || 'Reset failed')
        } finally {
            setLoading(false)
        }
    }

    if (done) {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center">
                <h1 className="text-2xl font-bold mb-4">Password Reset</h1>
                <p className="text-gray-600 mb-6">Your password has been reset successfully.</p>
                <Link to="/login" className="inline-block bg-blue-600 text-white px-5 py-2 rounded-md text-sm font-medium hover:bg-blue-700 transition-colors">
                    Go to Login
                </Link>
            </div>
        )
    }

    if (!token) {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center">
                <h1 className="text-2xl font-bold mb-4">Invalid Link</h1>
                <p className="text-gray-600 mb-6">This reset link is invalid or missing a token.</p>
                <Link to="/forgot-password" className="text-blue-600 hover:underline">Request a new reset link</Link>
            </div>
        )
    }

    return (
        <div className="max-w-sm mx-auto mt-20">
            <h1 className="text-2xl font-bold mb-6">Reset Password</h1>
            {err && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{err}</div>}
            <form onSubmit={handle} className="space-y-4">
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">New Password</label>
                    <input
                        type="password"
                        value={password}
                        onChange={e => setPassword(e.target.value)}
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                        required
                        minLength={6}
                    />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Confirm Password</label>
                    <input
                        type="password"
                        value={confirm}
                        onChange={e => setConfirm(e.target.value)}
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                        required
                        minLength={6}
                    />
                </div>
                <button
                    type="submit"
                    disabled={loading}
                    className="w-full bg-blue-600 text-white py-2 rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
                >
                    {loading ? 'Resetting...' : 'Reset Password'}
                </button>
            </form>
        </div>
    )
}
