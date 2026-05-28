import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { api, setTokens } from '../lib/api'

export default function Login() {
    const [form, setForm] = useState({ username: '', password: '' })
    const [err, setErr] = useState('')
    const [loading, setLoading] = useState(false)
    const nav = useNavigate()

    const handle = async (e: React.FormEvent) => {
        e.preventDefault()
        setErr('')
        setLoading(true)
        try {
            const d = await api.auth.login(form)
            setTokens(d.access_token, d.refresh_token)
            nav('/')
        } catch (e: any) {
            setErr(e.message || 'Login failed')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="max-w-sm mx-auto mt-20">
            <h1 className="text-2xl font-bold mb-6">Login</h1>
            {err && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{err}</div>}
            <form onSubmit={handle} className="space-y-4">
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Username</label>
                    <input
                        value={form.username}
                        onChange={e => setForm(p => ({ ...p, username: e.target.value }))}
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                        required
                    />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
                    <input
                        type="password"
                        value={form.password}
                        onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
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
            <div className="text-center text-sm text-gray-500 mt-4 space-y-1">
                <p>Don't have an account? <Link to="/register" className="text-blue-600 hover:underline">Register</Link></p>
                <p><Link to="/forgot-password" className="text-blue-600 hover:underline">Forgot Password?</Link></p>
            </div>
        </div>
    )
}