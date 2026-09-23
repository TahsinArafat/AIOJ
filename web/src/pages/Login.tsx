import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { api, setTokens } from '../lib/api'

export default function Login() {
    const [form, setForm] = useState({ username: '', password: '' })
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const [challenge, setChallenge] = useState('')
    const [code, setCode] = useState('')
    const [verifying, setVerifying] = useState(false)
    const nav = useNavigate()

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        setLoading(true)
        try {
            const d = await api.auth.login(form)
            if (d.requires_2fa && d.challenge_id) {
                setChallenge(d.challenge_id)
                return
            }
            if (!d.access_token || !d.refresh_token) {
                setErr('Login failed')
                return
            }
            setTokens(d.access_token, d.refresh_token)
            nav('/')
        } catch (e: unknown) {
            setErr(e instanceof Error ? e.message : 'Login failed')
        } finally {
            setLoading(false)
        }
    }

    const verify2fa = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        setVerifying(true)
        try {
            const d = await api.auth.verify2FA(challenge, code)
            setTokens(d.access_token, d.refresh_token)
            nav('/')
        } catch (e: unknown) {
            setErr(e instanceof Error ? e.message : 'Invalid code')
        } finally {
            setVerifying(false)
        }
    }

    if (challenge) {
        return (
            <div className="max-w-sm mx-auto mt-20">
                <h1 className="text-2xl font-bold mb-6">Two-Factor Authentication</h1>
                {err && <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-2 rounded mb-4 text-sm">{err}</div>}
                <form onSubmit={verify2fa} className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Authentication code</label>
                        <input
                            value={code}
                            onChange={e => setCode(e.target.value)}
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                            required
                        />
                    </div>
                    <button
                        type="submit"
                        disabled={verifying}
                        className="w-full bg-blue-600 text-white py-2 rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
                    >
                        {verifying ? 'Verifying...' : 'Verify'}
                    </button>
                </form>
            </div>
        )
    }

    return (
        <div className="max-w-sm mx-auto mt-20">
            <h1 className="text-2xl font-bold mb-6">Login</h1>
            {err && <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-2 rounded mb-4 text-sm">{err}</div>}
            <form onSubmit={handle} className="space-y-4">
                <div>
                    <label htmlFor="username" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Username</label>
                    <input
                        id="username"
                        name="username"
                        value={form.username}
                        onChange={e => setForm(p => ({ ...p, username: e.target.value }))}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                        required
                    />
                </div>
                <div>
                    <label htmlFor="password" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
                    <input
                        id="password"
                        name="password"
                        type="password"
                        value={form.password}
                        onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                        className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                        required
                    />
                </div>
                <button
                    type="submit"
                    disabled={loading}
                    className="w-full bg-blue-600 text-white py-2 rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
                >
                    {loading ? 'Logging in...' : 'Login'}
                </button>
            </form>
            <div className="mt-6 space-y-2">
                <a href="/api/auth/oauth/github/start" className="block w-full border border-gray-300 dark:border-gray-600 py-2 rounded-md text-sm font-medium text-center hover:bg-gray-50 dark:hover:bg-gray-800">Continue with GitHub</a>
                <a href="/api/auth/oauth/google/start" className="block w-full border border-gray-300 dark:border-gray-600 py-2 rounded-md text-sm font-medium text-center hover:bg-gray-50 dark:hover:bg-gray-800">Continue with Google</a>
            </div>
            <div className="text-center text-sm text-gray-500 dark:text-gray-400 mt-4 space-y-1">
                <p>Don't have an account? <Link to="/register" className="text-blue-600 dark:text-blue-400 hover:underline">Register</Link></p>
                <p><Link to="/forgot-password" className="text-blue-600 dark:text-blue-400 hover:underline">Forgot Password?</Link></p>
            </div>
        </div>
    )
}
