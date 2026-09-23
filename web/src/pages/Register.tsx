import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, setTokens } from '../lib/api'

export default function Register() {
    const [form, setForm] = useState({ username: '', email: '', password: '' })
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [sent, setSent] = useState(false)
    const [resent, setResent] = useState(false)

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        if (form.password.length < 12) { setErr('Password must be at least 12 characters'); return }
        setLoading(true)
        try {
            const d = await api.auth.register(form)
            setTokens(d.access_token, d.refresh_token)
            setSent(true)
        } catch (e: unknown) {
            setErr(e instanceof Error ? e.message : 'Registration failed')
        } finally {
            setLoading(false)
        }
    }

    const resend = async () => {
        setErr('')
        try {
            await api.auth.resendVerification()
            setResent(true)
        } catch {
            /* enumeration-safe: ignore errors quietly */
        }
    }

    if (sent) {
        return (
            <div className="max-w-sm mx-auto mt-20 text-center">
                <h1 className="text-2xl font-bold mb-4">Check Your Email</h1>
                <p className="text-gray-600 dark:text-gray-400 mb-4">
                    We sent a verification link to <strong>{form.email}</strong>.
                    Verify your email to submit solutions.
                </p>
                {resent && (
                    <p className="text-sm text-green-600 dark:text-green-400 mb-2">Verification email resent.</p>
                )}
                <button
                    type="button"
                    onClick={resend}
                    className="text-blue-600 dark:text-blue-400 hover:underline text-sm"
                >
                    Resend verification email
                </button>
                <p className="mt-4">
                    <Link to="/login" className="text-blue-600 dark:text-blue-400 hover:underline">Continue to Login</Link>
                </p>
            </div>
        )
    }

    return (
        <div className="max-w-sm mx-auto mt-20">
            <h1 className="text-2xl font-bold mb-6">Register</h1>
            {err && <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-2 rounded mb-4 text-sm">{err}</div>}
            <form onSubmit={handle} className="space-y-4">
                <div>
                    <label htmlFor="username" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Username</label>
                    <input id="username" name="username" value={form.username} onChange={e => setForm(p => ({ ...p, username: e.target.value }))}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400" required />
                </div>
                <div>
                    <label htmlFor="email" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Email</label>
                    <input id="email" name="email" type="email" value={form.email} onChange={e => setForm(p => ({ ...p, email: e.target.value }))}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400" required />
                </div>
                <div>
                    <label htmlFor="password" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
                    <input id="password" name="password" type="password" value={form.password} onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400" required />
                </div>
                <button type="submit" disabled={loading}
                    className="w-full bg-blue-600 text-white py-2 rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors">
                    {loading ? 'Creating account...' : 'Register'}
                </button>
            </form>
            <p className="text-center text-sm text-gray-500 dark:text-gray-400 mt-4">
                Already have an account? <Link to="/login" className="text-blue-600 dark:text-blue-400 hover:underline">Login</Link>
            </p>
        </div>
    )
}
