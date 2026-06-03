import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { api, setTokens } from '../lib/api'

export default function Login() {
    const [tab, setTab] = useState<'regular' | 'contestant'>('regular')
    const [form, setForm] = useState({ username: '', password: '', contestId: '' })
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const nav = useNavigate()

    const handleRegular = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!form.username.trim() || !form.password.trim()) return
        setErr('')
        setLoading(true)
        try {
            const d = await api.auth.login({ username: form.username, password: form.password })
            setTokens(d.access_token, d.refresh_token)
            nav('/')
        } catch (e: any) {
            setErr(e.message || 'Login failed')
        } finally {
            setLoading(false)
        }
    }

    const handleContestant = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!form.username.trim() || !form.password.trim() || !form.contestId.trim()) return
        setErr('')
        setLoading(true)
        try {
            const d = await api.onsite.loginAsTeam(form.contestId, { username: form.username, password: form.password })
            setTokens(d.access_token, d.refresh_token)
            nav(`/contests/${form.contestId}`)
        } catch (e: any) {
            setErr(e.message || 'Login failed')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="max-w-sm mx-auto mt-20">
            <h1 className="text-2xl font-bold mb-6">Login</h1>

            <div className="flex bg-gray-100 dark:bg-gray-800 rounded-lg p-1 mb-6">
                <button
                    onClick={() => { setTab('regular'); setErr('') }}
                    className={`flex-1 py-2 text-sm font-medium rounded-md transition-colors ${tab === 'regular' ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                >
                    Regular
                </button>
                <button
                    onClick={() => { setTab('contestant'); setErr('') }}
                    className={`flex-1 py-2 text-sm font-medium rounded-md transition-colors ${tab === 'contestant' ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                >
                    Contestant
                </button>
            </div>

            {err && <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-2 rounded mb-4 text-sm">{err}</div>}

            {tab === 'regular' ? (
                <form onSubmit={handleRegular} className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Username</label>
                        <input
                            value={form.username}
                            onChange={e => setForm(p => ({ ...p, username: e.target.value }))}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                            required
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
                        <input
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
            ) : (
                <form onSubmit={handleContestant} className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Contest ID or Slug</label>
                        <input
                            value={form.contestId}
                            onChange={e => setForm(p => ({ ...p, contestId: e.target.value }))}
                            placeholder="e.g. 3 or my-contest"
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                            required
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Username</label>
                        <input
                            value={form.username}
                            onChange={e => setForm(p => ({ ...p, username: e.target.value }))}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                            required
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
                        <input
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
                        className="w-full bg-green-600 text-white py-2 rounded-md text-sm font-medium hover:bg-green-700 disabled:opacity-50 transition-colors"
                    >
                        {loading ? 'Logging in...' : 'Login as Contestant'}
                    </button>
                </form>
            )}

            <div className="text-center text-sm text-gray-500 dark:text-gray-400 mt-4 space-y-1">
                <p>Don't have an account? <Link to="/register" className="text-blue-600 dark:text-blue-400 hover:underline">Register</Link></p>
                <p><Link to="/forgot-password" className="text-blue-600 dark:text-blue-400 hover:underline">Forgot Password?</Link></p>
            </div>
        </div>
    )
}
